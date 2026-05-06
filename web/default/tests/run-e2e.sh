#!/usr/bin/env bash
# End-to-end test orchestrator.
#
# Steps:
#   1. ensure aikanhub-app container is running with up-to-date code
#   2. start mock Volcano upstream (./mock_volcano.py) on :8721
#   3. configure Channel #1 to point at the mock + correct seedance models
#   4. run the python replay (backend round-trip)
#   5. run the Playwright admin-UI suite
#
# Requirements:
#   - .env.maomao with AIKANHUB_TOKEN (gateway sk- token, NOT the upstream key)
#   - admin user (default: admin/admin123456) for the UI suite
set -euo pipefail
cd "$(dirname "$0")"

GATEWAY_TOKEN="${AIKANHUB_GATEWAY_TOKEN:-}"
if [ -z "$GATEWAY_TOKEN" ]; then
  echo "Set AIKANHUB_GATEWAY_TOKEN to a gateway sk- token (issued via /api/token/)." >&2
  exit 1
fi
export AIKANHUB_DB="${AIKANHUB_DB:-/Users/randomradio/src/aikanhub/data/one-api.db}"
export AIKANHUB_BASE_URL="${AIKANHUB_BASE_URL:-http://localhost:3000}"
export AIKANHUB_ADMIN_USER="${AIKANHUB_ADMIN_USER:-admin}"
export AIKANHUB_ADMIN_PASS="${AIKANHUB_ADMIN_PASS:-admin123456}"
MOCK_BASE_URL="${AIKANHUB_E2E_MOCK_BASE_URL:-http://host.docker.internal:8721}"
SEEDANCE_MODELS="doubao-seedance-2-0-fast-260128,doubao-seedance-2-0-260128"

# Activate or create the python venv.
if [ ! -d .venv ]; then python3 -m venv .venv; fi
# shellcheck source=/dev/null
source .venv/bin/activate
pip install -q --disable-pip-version-check httpx python-dotenv >/dev/null

# 1. Mock upstream.
if ! curl -sS -m 2 -o /dev/null http://localhost:8721/api/v3/contents/generations/tasks/ping; then
  echo "Starting mock_volcano on :8721..."
  nohup python mock_volcano.py > mock_volcano.log 2>&1 &
  sleep 1
fi
curl -sS -m 3 -o /dev/null -w "[mock] HTTP %{http_code}\n" \
  http://localhost:8721/api/v3/contents/generations/tasks/ping

# 2. Force local DB routing to the mock before any replay can submit work.
AIKANHUB_E2E_MOCK_BASE_URL="$MOCK_BASE_URL" \
AIKANHUB_E2E_SEEDANCE_MODELS="$SEEDANCE_MODELS" \
python - <<'PY'
import os
import sqlite3
import sys
import time

db = os.environ["AIKANHUB_DB"]
mock_base_url = os.environ["AIKANHUB_E2E_MOCK_BASE_URL"]
models = os.environ["AIKANHUB_E2E_SEEDANCE_MODELS"]
model_names = [m.strip() for m in models.split(",") if m.strip()]
groups = ["default", "trial", "beta", "invited", "b2b", "enterprise"]

if not os.path.exists(db):
    print(f"AIKANHUB_DB does not exist: {db}", file=sys.stderr)
    sys.exit(2)

con = sqlite3.connect(db)
try:
    channel_columns = {
        row[1] for row in con.execute("pragma table_info(channels)").fetchall()
    }
    ability_columns = {
        row[1] for row in con.execute("pragma table_info(abilities)").fetchall()
    }
    required_channel = {"type", "key", "status", "name", "base_url", "models", "group"}
    required_ability = {"group", "model", "channel_id", "enabled", "priority", "weight"}
    if not required_channel.issubset(channel_columns):
        missing = sorted(required_channel - channel_columns)
        raise RuntimeError(f"channels table missing columns: {missing}")
    if not required_ability.issubset(ability_columns):
        missing = sorted(required_ability - ability_columns)
        raise RuntimeError(f"abilities table missing columns: {missing}")

    now = int(time.time())
    key = "sk-local-seedance-fixture"
    channel = con.execute(
        "select id from channels where type = ? order by id limit 1",
        (54,),
    ).fetchone()
    if channel:
        channel_id = int(channel[0])
        con.execute(
            """
            update channels
               set key = ?,
                   status = 1,
                   name = ?,
                   base_url = ?,
                   models = ?,
                   "group" = ?
             where id = ?
            """,
            (key, "E2E Seedance Mock", mock_base_url, models, ",".join(groups), channel_id),
        )
    else:
        con.execute(
            """
            insert into channels
                (type, key, status, name, base_url, models, "group", created_time)
            values
                (?, ?, 1, ?, ?, ?, ?, ?)
            """,
            (54, key, "E2E Seedance Mock", mock_base_url, models, ",".join(groups), now),
        )
        channel_id = int(con.execute("select last_insert_rowid()").fetchone()[0])

    con.execute(
        "update channels set status = 2 where type = ? and id <> ?",
        (54, channel_id),
    )
    placeholders = ",".join("?" for _ in model_names)
    con.execute(
        f"update abilities set enabled = 0 where model in ({placeholders}) and channel_id <> ?",
        (*model_names, channel_id),
    )
    for group in groups:
        for model in model_names:
            con.execute(
                """
                insert into abilities ("group", model, channel_id, enabled, priority, weight)
                values (?, ?, ?, 1, 100000, 100000)
                on conflict("group", model, channel_id) do update set
                    enabled = excluded.enabled,
                    priority = excluded.priority,
                    weight = excluded.weight
                """,
                (group, model, channel_id),
            )

    con.commit()
    row = con.execute(
        "select id, type, status, base_url, models from channels where id = ?",
        (channel_id,),
    ).fetchone()
    if not row or row[1] != 54 or row[2] != 1 or row[3] != mock_base_url or row[4] != models:
        raise RuntimeError(f"failed to verify mock channel update: {row!r}")
    competitors = con.execute(
        "select id, base_url, models from channels where type = ? and status = 1 and id <> ?",
        (54, channel_id),
    ).fetchall()
    if competitors:
        raise RuntimeError(f"enabled competing Seedance channels remain: {competitors!r}")
    print(f"[db] channel #{channel_id} type=54 forced to {mock_base_url}")
finally:
    con.close()
PY

# 2b. Refresh the in-process channel cache; direct SQLite edits are not visible
# until the backend rebuilds its cache.
python - <<'PY'
import os
import sys

import httpx

base_url = os.environ["AIKANHUB_BASE_URL"].rstrip("/")
username = os.environ["AIKANHUB_ADMIN_USER"]
password = os.environ["AIKANHUB_ADMIN_PASS"]

with httpx.Client(base_url=base_url, timeout=20, trust_env=False) as client:
    login = client.post(
        "/api/user/login",
        json={"username": username, "password": password},
    )
    try:
        login_body = login.json()
    except Exception:
        login_body = {"raw": login.text}
    if login.status_code != 200 or not login_body.get("success"):
        print(f"admin login failed while refreshing channel cache: {login_body}", file=sys.stderr)
        sys.exit(3)
    user_id = str((login_body.get("data") or {}).get("id") or "")
    if not user_id:
        print(f"admin login did not return a user id: {login_body}", file=sys.stderr)
        sys.exit(3)

    fix = client.post("/api/channel/fix", headers={"New-Api-User": user_id})
    try:
        fix_body = fix.json()
    except Exception:
        fix_body = {"raw": fix.text}
    if fix.status_code != 200 or not fix_body.get("success"):
        print(f"channel cache refresh failed: {fix_body}", file=sys.stderr)
        sys.exit(4)

print("[api] channel cache refreshed via /api/channel/fix")
PY

# 3. Backend round-trip.
echo
echo "=== backend replay ==="
AIKANHUB_GATEWAY_TOKEN="$GATEWAY_TOKEN" python replay.py

# 4. Playwright suite.
echo
echo "=== Playwright admin UI ==="
cd e2e
if [ ! -d node_modules ]; then bun install; fi
if [ ! -d ~/Library/Caches/ms-playwright/chromium-1217 ]; then
  bunx playwright install chromium
fi
bunx playwright test --reporter=line
