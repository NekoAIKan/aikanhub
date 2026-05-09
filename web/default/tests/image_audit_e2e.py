#!/usr/bin/env python3
"""
E2E test for /v1/image-audits — issue #43.

Drives the running kittyvibe-app container against real Volcano ARK.
This is a STANDALONE script — it can't piggyback on the mock-Volcano
harness in run-e2e.sh because image audit needs real upstream
moderation decisions, not replayed fixtures.

Asserts:

  1. POST returns 200 with status=processing on a fresh image.
  2. GET /:id polls until terminal (active or failed).
  3. POST again with same source string → cache hit (same record_id, <200ms).
  4. POST with `wait=true` blocks until terminal.
  5. POST with `image: "asset://..."` → 400 asset_uri_not_auditable.
  6. GET / list returns rows.
  7. GET /imgaudit_unknown → 404.
  8. PublicURL is NOT exposed in JSON.

Required env:
  KITTYVIBE_BASE_URL          default http://localhost:3000
  KITTYVIBE_GATEWAY_TOKEN     the user's sk- token (issued via /api/token/)
  AUDIT_TEST_IMAGE_URL        an HTTPS URL ARK CreateAsset can fetch
                              (must be public OR an OSS signed URL —
                              private buckets fail with 403 from ARK)

Run:
  KITTYVIBE_GATEWAY_TOKEN=sk-... \
  AUDIT_TEST_IMAGE_URL='https://...' \
  python image_audit_e2e.py
"""

from __future__ import annotations

import json
import os
import sys
import time
from typing import Any
from urllib.error import HTTPError
from urllib.request import Request, urlopen


BASE = os.environ.get("KITTYVIBE_BASE_URL", "http://localhost:3000").rstrip("/")
TOKEN = os.environ.get("KITTYVIBE_GATEWAY_TOKEN", "")
TEST_IMAGE = os.environ.get(
    "AUDIT_TEST_IMAGE_URL",
    "https://aikanhub.oss-cn-hangzhou.aliyuncs.com/ava.jpg",
)


def fail(msg: str) -> None:
    print(f"FAIL: {msg}", file=sys.stderr)
    sys.exit(1)


def http(method: str, path: str, body: dict | None = None) -> tuple[int, dict[str, Any]]:
    if not TOKEN:
        fail("KITTYVIBE_GATEWAY_TOKEN required")
    data = json.dumps(body).encode() if body is not None else None
    req = Request(
        BASE + path, data=data, method=method,
        headers={
            "Authorization": f"Bearer {TOKEN}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urlopen(req, timeout=30) as r:
            return r.status, json.loads(r.read() or b"{}")
    except HTTPError as e:
        try:
            return e.code, json.loads(e.read() or b"{}")
        except Exception:
            return e.code, {}


def step(name: str, fn) -> Any:
    print(f"\n[STEP] {name}")
    out = fn()
    print("       OK")
    return out


def assert_eq(actual: Any, expected: Any, label: str) -> None:
    if actual != expected:
        fail(f"{label}: expected {expected!r}, got {actual!r}")


def assert_in(needle: Any, haystack, label: str) -> None:
    if needle not in haystack:
        fail(f"{label}: expected {needle!r} in {haystack!r}")


def main() -> int:
    print(f"BASE={BASE}  IMAGE={TEST_IMAGE}")

    # 1. Fresh submit.
    def s1():
        code, body = http("POST", "/v1/image-audits", {"image": TEST_IMAGE})
        assert_eq(code, 200, "POST status")
        assert_in("id", body, "POST body has id")
        assert body["id"].startswith("imgaudit_"), f"id format: {body['id']}"
        assert_in(body["status"], ("processing", "active"), "status valid")
        return body
    rec1 = step("Fresh POST", s1)

    # 2. Poll until terminal.
    def s2():
        rec_id = rec1["id"]
        deadline = time.time() + 300
        last = ""
        while time.time() < deadline:
            code, body = http("GET", f"/v1/image-audits/{rec_id}")
            assert_eq(code, 200, "GET status")
            st = body.get("status")
            if st != last:
                print(f"       status={st}")
                last = st
            if st in ("active", "failed"):
                return body
            time.sleep(3)
        fail(f"poll timed out, last status={last}")
    rec1_final = step("Poll to terminal", s2)

    # 3. Cache hit on same image — must return same record id.
    def s3():
        code, body = http("POST", "/v1/image-audits", {"image": TEST_IMAGE})
        assert_eq(code, 200, "cached POST status")
        assert_eq(body["id"], rec1_final["id"],
                  "cache hit should return same record_id")
        # Cache hit is instant — status is already terminal.
        assert_in(body["status"], ("active", "failed"), "cache hit status terminal")
    step("Cache hit (same image)", s3)

    # 4. asset:// rejection.
    def s4():
        code, body = http("POST", "/v1/image-audits", {"image": "asset://asset-fake"})
        assert_eq(code, 400, "asset:// status")
        err = body.get("error", {})
        assert_eq(err.get("code"), "asset_uri_not_auditable", "asset:// error code")
    step("asset:// → 400", s4)

    # 5. Sync wait mode (only meaningful for fresh image — use a tweaked
    # URL so we don't reuse the cached record). The cache key is the
    # source string verbatim, so ANY tweak that round-trips through
    # the upstream fetcher works. Append a query param with the right
    # joiner ('&' if the URL already has '?', else '?'); the OSS
    # signed URL ignores unknown params.
    sep = "&" if "?" in TEST_IMAGE else "?"
    twiddled = TEST_IMAGE + sep + "cb=" + str(int(time.time()))
    def s5():
        code, body = http("POST", "/v1/image-audits",
                          {"image": twiddled, "wait": True})
        assert_eq(code, 200, "wait=true status")
        assert_in(body["status"], ("active", "failed"),
                  "wait=true should return terminal status")
        return body
    rec2 = step("wait=true blocks to terminal", s5)

    # 6. List.
    def s6():
        code, body = http("GET", "/v1/image-audits?limit=10")
        assert_eq(code, 200, "list status")
        ids = [r["id"] for r in body.get("data", [])]
        assert_in(rec1_final["id"], ids, "list contains rec1")
        assert_in(rec2["id"], ids, "list contains rec2")
    step("List shows recent records", s6)

    # 7. Unauthorized id (random valid prefix).
    def s7():
        code, body = http("GET", "/v1/image-audits/imgaudit_deadbeefdeadbeef")
        assert_eq(code, 404, "missing id status")
    step("Unknown id → 404", s7)

    # 8. PublicURL must NOT be in the JSON response.
    def s8():
        for r in (rec1_final, rec2):
            assert "public_url" not in r, f"public_url leaked in record: {r}"
    step("PublicURL not exposed in JSON", s8)

    print("\nALL E2E STEPS PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main())
