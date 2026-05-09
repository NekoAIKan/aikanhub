"""Replay-mode end-to-end test.

Assumes:
  - kittyvibe running on $KITTYVIBE_BASE_URL (default http://localhost:3000)
  - mock_volcano.py running on host port 8721
  - Channel #1 (or any Doubao type=54 channel) configured with
      base_url=http://host.docker.internal:8721
      models=doubao-seedance-2-0-fast-260128,doubao-seedance-2-0-260128
  - $KITTYVIBE_GATEWAY_TOKEN set to a sk- token

Submits one of each PR #27 task shape against the mocked upstream and
asserts:
  - submit returns 200 (or 400 for the failure case)
  - poll converges to "succeeded" (or stays as "FAILURE" for the failure case)
  - the gateway-side row carries the right action label (text/generate/
    firstTailGenerate/referenceGenerate)
  - quota was charged
"""
from __future__ import annotations

import os
import json
import sqlite3
import sys
import time
from pathlib import Path

import httpx
from dotenv import load_dotenv

HERE = Path(__file__).resolve().parent
load_dotenv(HERE.parent.parent.parent / ".env.maomao")

def env(name: str, legacy_name: str, default: str | None = None) -> str | None:
    return os.environ.get(name) or os.environ.get(legacy_name) or default


def require_env(name: str, legacy_name: str) -> str:
    value = env(name, legacy_name)
    if not value:
        raise RuntimeError(f"set {name} or {legacy_name}")
    return value


BASE = env("KITTYVIBE_BASE_URL", "AIKANHUB_BASE_URL", "http://localhost:3000")
TOKEN = require_env("KITTYVIBE_GATEWAY_TOKEN", "AIKANHUB_GATEWAY_TOKEN")
DB = os.environ.get("AIKANHUB_DB", "/Users/randomradio/src/aikanhub/data/one-api.db")
TIMEOUT = int(os.environ.get("REPLAY_TIMEOUT", "120"))
MOCK_BASE = env("KITTYVIBE_E2E_MOCK_BASE_URL", "AIKANHUB_E2E_MOCK_BASE_URL", "http://host.docker.internal:8721")

FAST = "doubao-seedance-2-0-fast-260128"
FULL = "doubao-seedance-2-0-260128"

IMG_FIRST = "https://picsum.photos/seed/aikanhub-first/800/600"
IMG_LAST = "https://picsum.photos/seed/aikanhub-last/800/600"
IMG_REF1 = "https://picsum.photos/seed/aikanhub-ref1/800/600"
IMG_REF2 = "https://picsum.photos/seed/aikanhub-ref2/800/600"
VIDEO_REF = "https://test-videos.co.uk/x.mp4"
AUDIO_REF = "https://www.kozco.com/tech/piano2.wav"


CASES = [
    {
        "name": "text_720p_5s",
        "expected_status": "succeeded",
        "expected_action": "textGenerate",
        "expected_quota": 54450,
        "model": FAST,
        "content": [{"type": "text", "text": "a tiny ginger cat walks along a sunny windowsill"}],
        "duration": 5,
        "ratio": "16:9",
        "resolution": "720p",
    },
    {
        "name": "text_1080p_5s",
        "expected_status": "succeeded",
        "expected_action": "textGenerate",
        "expected_quota": 122512,
        "model": FULL,
        "content": [{"type": "text", "text": "macro dew on leaf at sunrise"}],
        "duration": 5,
        "ratio": "16:9",
        "resolution": "1080p",
    },
    {
        "name": "i2v_first",
        "expected_status": "succeeded",
        "expected_action": "generate",
        "expected_quota": 54450,
        "model": FAST,
        "content": [
            {"type": "text", "text": "subject animates from still"},
            {"type": "image_url", "image_url": {"url": IMG_FIRST}, "role": "first_frame"},
        ],
        "duration": 5,
        "ratio": "16:9",
    },
    {
        "name": "i2v_firstlast",
        "expected_status": "succeeded",
        "expected_action": "firstTailGenerate",
        "expected_quota": 54450,
        "model": FAST,
        "content": [
            {"type": "text", "text": "smooth transition"},
            {"type": "image_url", "image_url": {"url": IMG_FIRST}, "role": "first_frame"},
            {"type": "image_url", "image_url": {"url": IMG_LAST}, "role": "last_frame"},
        ],
        "duration": 5,
        "ratio": "16:9",
    },
    {
        "name": "edit",
        "expected_status": "succeeded",
        "expected_action": "referenceGenerate",
        "expected_quota": 162450,
        "model": FULL,
        "content": [
            {"type": "text", "text": "edit"},
            {"type": "image_url", "image_url": {"url": IMG_REF1}, "role": "reference_image"},
            {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
        ],
        "duration": 5,
        "ratio": "16:9",
    },
    {
        "name": "extend",
        "expected_status": "succeeded",
        "expected_action": "referenceGenerate",
        "expected_quota": 194850,
        "model": FULL,
        "content": [
            {"type": "text", "text": "continue scene"},
            {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
        ],
        "duration": 8,
        "ratio": "16:9",
    },
    {
        "name": "multimodal",
        "expected_status": "succeeded",
        "expected_action": "referenceGenerate",
        "expected_quota": 162450,
        "model": FULL,
        "content": [
            {"type": "text", "text": "POV"},
            {"type": "image_url", "image_url": {"url": IMG_REF1}, "role": "reference_image"},
            {"type": "image_url", "image_url": {"url": IMG_REF2}, "role": "reference_image"},
            {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
            {"type": "audio_url", "audio_url": {"url": AUDIO_REF}, "role": "reference_audio"},
        ],
        "duration": 5,
        "ratio": "16:9",
        "generate_audio": True,
    },
    {
        "name": "web_search",
        "expected_status": "succeeded",
        "expected_action": "textGenerate",
        "expected_quota": 54450,
        "model": FULL,
        "content": [{"type": "text", "text": "glass frog"}],
        "duration": 5,
        "ratio": "16:9",
        "tools": [{"type": "web_search"}],
    },
    {
        "name": "fail_fast_1080p",
        "expected_status": "FAILURE",
        "expected_action": "textGenerate",
        "expected_submit_status": 400,
        "model": FAST,
        "content": [{"type": "text", "text": "fast model 1080p should fail"}],
        "duration": 5,
        "ratio": "16:9",
        "resolution": "1080p",
    },
]


def _db_lookup(task_id: str) -> dict | None:
    con = sqlite3.connect(DB)
    con.row_factory = sqlite3.Row
    row = con.execute(
        "select id, task_id, status, action, quota, fail_reason, private_data from tasks where task_id=?",
        (task_id,),
    ).fetchone()
    con.close()
    return dict(row) if row else None


def _json_obj(value) -> dict:
    if value is None:
        return {}
    if isinstance(value, bytes):
        value = value.decode("utf-8", errors="replace")
    if not isinstance(value, str) or not value.strip():
        return {}
    try:
        parsed = json.loads(value)
    except json.JSONDecodeError:
        return {}
    return parsed if isinstance(parsed, dict) else {}


def _log_others(task_id: str) -> list[dict]:
    con = sqlite3.connect(DB)
    con.row_factory = sqlite3.Row
    rows = con.execute(
        "select id, type, quota, other from logs where other like ? order by id desc limit 20",
        (f'%"{task_id}"%',),
    ).fetchall()
    con.close()
    out = []
    for row in rows:
        other = _json_obj(row["other"])
        if other:
            out.append({"id": row["id"], "type": row["type"], "quota": row["quota"], "other": other})
    return out


def _billing_evidence(task_id: str, row: dict) -> dict:
    private = _json_obj(row.get("private_data"))
    billing_context = private.get("billing_context") if isinstance(private, dict) else None
    if isinstance(billing_context, dict):
        estimated = int(billing_context.get("estimated_quota") or 0)
        if estimated > 0:
            return {
                "source": "task.private_data.billing_context",
                "precharged_quota": estimated,
                "billing_mode": billing_context.get("billing_mode"),
                "pricing_version": billing_context.get("pricing_version"),
            }

    for entry in _log_others(task_id):
        other = entry["other"]
        precharged = int(other.get("pre_consumed_quota") or other.get("estimated_quota") or 0)
        if precharged > 0:
            return {
                "source": f"log:{entry['id']}",
                "precharged_quota": precharged,
                "billing_mode": other.get("billing_mode"),
                "pricing_version": other.get("pricing_version"),
            }
    return {}


def _assert_mock_channel_configured() -> None:
    if not os.path.exists(DB):
        raise RuntimeError(f"AIKANHUB_DB does not exist: {DB}")
    con = sqlite3.connect(DB)
    con.row_factory = sqlite3.Row
    try:
        rows = con.execute(
            "select id, base_url, models, status from channels where type = 54 and status = 1"
        ).fetchall()
    finally:
        con.close()
    for row in rows:
        models = {m.strip() for m in (row["models"] or "").split(",")}
        if row["base_url"] == MOCK_BASE and {FAST, FULL}.issubset(models):
            return
    raise RuntimeError(
        f"refusing to replay: no enabled Doubao type=54 channel in {DB} points at {MOCK_BASE}"
    )


def run_case(client: httpx.Client, case: dict) -> dict:
    name = case["name"]
    body = {k: v for k, v in case.items() if not k.startswith("expected") and k != "name"}
    expected_status = case.get("expected_status", "succeeded")
    expected_submit = case.get("expected_submit_status", 200)
    expected_action = case["expected_action"]

    t0 = time.time()
    sub = client.post("/api/v3/contents/generations/tasks", json=body)
    if sub.status_code != expected_submit:
        return {
            "name": name,
            "ok": False,
            "stage": "submit",
            "got": sub.status_code,
            "want": expected_submit,
            "body": sub.text[:200],
        }
    if sub.status_code != 200:
        return {"name": name, "ok": True, "stage": "submit_terminal_400"}

    task_id = sub.json()["id"]
    deadline = time.time() + TIMEOUT
    final = None
    while time.time() < deadline:
        r = client.get(f"/api/v3/contents/generations/tasks/{task_id}")
        body = r.json()
        st = body.get("status")
        if st in ("succeeded", "failed", "cancelled"):
            final = body
            break
        time.sleep(2)
    if final is None:
        return {"name": name, "ok": False, "stage": "poll_timeout", "task_id": task_id}

    if final["status"] != expected_status:
        return {
            "name": name,
            "ok": False,
            "stage": "terminal_status",
            "got": final["status"],
            "want": expected_status,
            "task_id": task_id,
        }

    # Look the task up in the DB to verify action label + quota.
    row = _db_lookup(task_id)
    if row is None:
        return {"name": name, "ok": False, "stage": "db_missing", "task_id": task_id}
    if row["action"] != expected_action:
        return {
            "name": name,
            "ok": False,
            "stage": "action_label",
            "got": row["action"],
            "want": expected_action,
            "task_id": task_id,
        }
    if expected_status == "succeeded" and (row["quota"] or 0) <= 0:
        return {
            "name": name,
            "ok": False,
            "stage": "quota_zero",
            "got": row["quota"],
            "task_id": task_id,
        }
    expected_quota = case.get("expected_quota")
    if expected_status == "succeeded" and expected_quota is not None and row["quota"] != expected_quota:
        return {
            "name": name,
            "ok": False,
            "stage": "quota_exact",
            "got": row["quota"],
            "want": expected_quota,
            "task_id": task_id,
        }

    if expected_status == "succeeded":
        evidence = _billing_evidence(task_id, row)
        precharged = int(evidence.get("precharged_quota") or 0)
        if precharged <= 0:
            return {
                "name": name,
                "ok": False,
                "stage": "precharge_missing",
                "task_id": task_id,
            }
        if precharged < row["quota"]:
            return {
                "name": name,
                "ok": False,
                "stage": "precharge_less_than_final",
                "got": precharged,
                "want_at_least": row["quota"],
                "source": evidence.get("source"),
                "task_id": task_id,
            }

    return {
        "name": name,
        "ok": True,
        "task_id": task_id,
        "status": row["status"],
        "action": row["action"],
        "quota": row["quota"],
        "precharged_quota": precharged if expected_status == "succeeded" else None,
        "duration": round(time.time() - t0, 2),
    }


def main() -> None:
    _assert_mock_channel_configured()
    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    results = []
    with httpx.Client(base_url=BASE, headers=headers, timeout=60, trust_env=False) as client:
        for case in CASES:
            r = run_case(client, case)
            results.append(r)
            flag = "✓" if r["ok"] else "✗"
            print(f"  {flag} {r['name']:20} {r}")

    print("\n" + "=" * 60)
    ok = sum(1 for r in results if r["ok"])
    print(f"{ok}/{len(results)} passed")
    if ok != len(results):
        sys.exit(1)


if __name__ == "__main__":
    main()
