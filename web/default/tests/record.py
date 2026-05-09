"""Record real Volcano Ark request/response/video triples for offline replay.

Submits all scenarios in parallel, polls in parallel, downloads each video.
Output layout:

    fixtures/<case>/request.json          — what we POSTed
    fixtures/<case>/submit.json           — Volcano's first response (task id)
    fixtures/<case>/final.json            — terminal poll response
    fixtures/<case>/poll_log.json         — list of intermediate poll snapshots
    fixtures/<case>/video.mp4             — downloaded binary (when status=succeeded)
    fixtures/<case>/meta.json             — sanitised summary (no signed URLs)

Set KITTYVIBE_WRITE_RAW_FIXTURES=1 to also write final.raw.json with signed
Volcano URLs for local debugging. Raw signed captures are gitignored.

Run:
    python record.py                      # all cases
    python record.py text i2v_first       # subset
    python record.py --skip multimodal    # all but listed

Token comes from ../../../.env.maomao (KITTYVIBE_TOKEN must be a real
ark- prefixed Volcano Ark API key, not the gateway sk- token).
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import time
import traceback
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

import httpx
from dotenv import load_dotenv

HERE = Path(__file__).resolve().parent
load_dotenv(HERE.parent.parent.parent / ".env.maomao")

def require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"set {name}")
    return value


TOKEN = require_env("KITTYVIBE_TOKEN")
FAST_MODEL = os.environ.get("KITTYVIBE_MODEL", "doubao-seedance-2-0-fast-260128")
FULL_MODEL = "doubao-seedance-2-0-260128"
ARK_BASE = os.environ.get("ARK_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3")
TIMEOUT = int(os.environ.get("KITTYVIBE_TIMEOUT", "900"))
WRITE_RAW_FIXTURES = os.environ.get("KITTYVIBE_WRITE_RAW_FIXTURES") == "1"

FIX = HERE / "fixtures"
FIX.mkdir(exist_ok=True)

IMG_FIRST = "https://picsum.photos/seed/aikanhub-first/800/600"
IMG_LAST = "https://picsum.photos/seed/aikanhub-last/800/600"
IMG_REF1 = "https://picsum.photos/seed/aikanhub-ref1/800/600"
IMG_REF2 = "https://picsum.photos/seed/aikanhub-ref2/800/600"
VIDEO_REF = (
    "https://test-videos.co.uk/vids/bigbuckbunny/mp4/h264/720/"
    "Big_Buck_Bunny_720_10s_1MB.mp4"
)
AUDIO_REF = "https://www.kozco.com/tech/piano2.wav"


def cases() -> dict[str, dict[str, Any]]:
    """Each case is a request payload, plus optional `expect_status`.

    The default is "succeeded"; set "failed" for the failure case.
    """
    return {
        "text_720p_5s": {
            "model": FAST_MODEL,
            "content": [
                {"type": "text", "text": "A ginger cat strolls down a Tokyo street at sunset, 4K, cinematic"},
            ],
            "duration": 5,
            "ratio": "16:9",
            "resolution": "720p",
        },
        "text_1080p_5s": {
            "model": FULL_MODEL,
            "content": [
                {"type": "text", "text": "Macro shot of dew on a leaf at sunrise, slow camera lift, vivid greens"},
            ],
            "duration": 5,
            "ratio": "16:9",
            "resolution": "1080p",
        },
        "i2v_first": {
            "model": FAST_MODEL,
            "content": [
                {"type": "text", "text": "Slow camera push-in, subject animates from still to motion"},
                {"type": "image_url", "image_url": {"url": IMG_FIRST}, "role": "first_frame"},
            ],
            "duration": 5,
            "ratio": "16:9",
        },
        "i2v_firstlast": {
            "model": FAST_MODEL,
            "content": [
                {"type": "text", "text": "Smooth transition from first frame to last frame"},
                {"type": "image_url", "image_url": {"url": IMG_FIRST}, "role": "first_frame"},
                {"type": "image_url", "image_url": {"url": IMG_LAST}, "role": "last_frame"},
            ],
            "duration": 5,
            "ratio": "16:9",
        },
        "multimodal": {
            "model": FULL_MODEL,
            "content": [
                {"type": "text", "text": "POV beverage commercial in the style of video1, scored with audio1"},
                {"type": "image_url", "image_url": {"url": IMG_REF1}, "role": "reference_image"},
                {"type": "image_url", "image_url": {"url": IMG_REF2}, "role": "reference_image"},
                {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
                {"type": "audio_url", "audio_url": {"url": AUDIO_REF}, "role": "reference_audio"},
            ],
            "duration": 5,
            "ratio": "16:9",
            "generate_audio": True,
        },
        "edit": {
            "model": FULL_MODEL,
            "content": [
                {"type": "text", "text": "Replace the subject in video1 with the object from image1; keep camera motion intact"},
                {"type": "image_url", "image_url": {"url": IMG_REF1}, "role": "reference_image"},
                {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
            ],
            "duration": 5,
            "ratio": "16:9",
            "generate_audio": True,
        },
        "extend": {
            "model": FULL_MODEL,
            "content": [
                {"type": "text", "text": "Continue the scene with cinematic camera motion"},
                {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
            ],
            "duration": 8,
            "ratio": "16:9",
            "generate_audio": True,
        },
        "web_search": {
            "model": FULL_MODEL,
            "content": [
                {"type": "text", "text": "Macro shot of a glass frog on an emerald leaf; focus shifts from skin to its transparent belly"},
            ],
            "duration": 5,
            "ratio": "16:9",
            "generate_audio": True,
            "tools": [{"type": "web_search"}],
        },
        # Submission-time validation failure: fast model + 1080p combination is rejected.
        "fail_fast_1080p": {
            "_expect_status": "submit_400",
            "model": FAST_MODEL,
            "content": [
                {"type": "text", "text": "Seedance billing fixture failure: unsupported fast-model 1080p validation check."}
            ],
            "duration": 5,
            "ratio": "16:9",
            "resolution": "1080p",
        },
    }


def _strip_signed_url(url: str) -> str:
    """Remove the query string from signed TOS URLs so fixtures don't leak the
    24h-valid signature when committed."""
    parts = urlsplit(url)
    return f"{parts.scheme}://{parts.netloc}{parts.path}"


def _sanitise(payload: Any) -> Any:
    """Walk the structure and drop signed-URL query strings on TOS hosts."""
    if isinstance(payload, dict):
        out = {}
        for k, v in payload.items():
            out[k] = _sanitise(v)
        return out
    if isinstance(payload, list):
        return [_sanitise(v) for v in payload]
    if isinstance(payload, str) and re.search(r"X-Tos-Signature", payload):
        return _strip_signed_url(payload)
    return payload


def submit(client: httpx.Client, name: str, payload: dict[str, Any]) -> dict[str, Any]:
    """POST /tasks. Return both the request body we sent and the response body."""
    body = {k: v for k, v in payload.items() if not k.startswith("_")}
    r = client.post("/contents/generations/tasks", json=body)
    rec = {
        "name": name,
        "request": body,
        "submit_status": r.status_code,
        "submit_response": _safe_json(r),
    }
    return rec


def poll(client: httpx.Client, task_id: str, log: list[dict[str, Any]]) -> dict[str, Any]:
    deadline = time.time() + TIMEOUT
    while time.time() < deadline:
        r = client.get(f"/contents/generations/tasks/{task_id}", timeout=30)
        body = _safe_json(r)
        log.append({"t": int(time.time()), "status": body.get("status"), "http": r.status_code})
        if body.get("status") in ("succeeded", "failed", "cancelled"):
            return body
        time.sleep(6)
    raise TimeoutError(f"task {task_id} did not finish in {TIMEOUT}s")


def _safe_json(r: httpx.Response) -> Any:
    try:
        return r.json()
    except Exception:  # noqa: BLE001
        return {"_raw": r.text}


def record_one(name: str, payload: dict[str, Any]) -> dict[str, Any]:
    out = FIX / name
    out.mkdir(parents=True, exist_ok=True)
    expect = payload.get("_expect_status", "succeeded")

    request_body = {k: v for k, v in payload.items() if not k.startswith("_")}
    (out / "request.json").write_text(json.dumps(request_body, indent=2, ensure_ascii=False))

    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    with httpx.Client(base_url=ARK_BASE, headers=headers, timeout=60) as client:
        # Submit.
        sub = client.post("/contents/generations/tasks", json=request_body)
        submit_body = _safe_json(sub)
        (out / "submit.json").write_text(
            json.dumps({"http": sub.status_code, "body": submit_body}, indent=2, ensure_ascii=False)
        )

        if expect == "submit_400":
            if sub.status_code != 400:
                raise AssertionError(
                    f"{name}: expected submit_400, got HTTP {sub.status_code}: {submit_body}"
                )
            (out / "meta.json").write_text(json.dumps({
                "case": name,
                "expected": expect,
                "submit_status": sub.status_code,
                "submit_body": submit_body,
                "video_artifact": {"kind": "none"},
            }, indent=2, ensure_ascii=False))
            return {"name": name, "ok": True, "expected": expect, "task_id": None}

        if sub.status_code != 200:
            raise RuntimeError(f"{name}: submit failed HTTP {sub.status_code}: {submit_body}")
        task_id = submit_body["id"]

        # Poll.
        poll_log: list[dict[str, Any]] = []
        final = poll(client, task_id, poll_log)
        (out / "poll_log.json").write_text(json.dumps(poll_log, indent=2, ensure_ascii=False))
        # Keep signed URLs in memory for downloading. Only write raw signed
        # captures when explicitly requested for local debugging.
        if WRITE_RAW_FIXTURES:
            (out / "final.raw.json").write_text(json.dumps(final, indent=2, ensure_ascii=False))
        (out / "final.json").write_text(json.dumps(_sanitise(final), indent=2, ensure_ascii=False))

        if final.get("status") != "succeeded":
            (out / "meta.json").write_text(json.dumps({
                "case": name,
                "expected": expect,
                "actual_status": final.get("status"),
                "task_id": task_id,
                "video_artifact": {"kind": "none"},
            }, indent=2, ensure_ascii=False))
            return {"name": name, "ok": False, "actual": final.get("status"), "task_id": task_id}

        # Download the binary.
        url = (final.get("content") or {}).get("video_url")
        if not url:
            raise RuntimeError(f"{name}: succeeded but no video_url in final response")
        with httpx.stream("GET", url, follow_redirects=True, timeout=300) as r:
            r.raise_for_status()
            video_path = out / "video.mp4"
            with video_path.open("wb") as f:
                for chunk in r.iter_bytes(64 * 1024):
                    f.write(chunk)
        size = video_path.stat().st_size

        billing = {
            "model": final.get("model"),
            "resolution": final.get("resolution"),
            "duration": final.get("duration"),
            "fps": final.get("framespersecond"),
            "completion_tokens": (final.get("usage") or {}).get("completion_tokens"),
            "total_tokens": (final.get("usage") or {}).get("total_tokens"),
        }
        (out / "meta.json").write_text(json.dumps({
            "case": name,
            "expected": expect,
            "actual_status": "succeeded",
            "task_id": task_id,
            "billing_fields": billing,
            "video_artifact": {"kind": "binary", "path": "video.mp4", "bytes": size},
            "poll_count": len(poll_log),
        }, indent=2, ensure_ascii=False))
        return {"name": name, "ok": True, "task_id": task_id, "bytes": size}


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("only", nargs="*")
    p.add_argument("--skip", nargs="*", default=[])
    p.add_argument("--workers", type=int, default=8)
    args = p.parse_args()

    all_cases = cases()
    selected = args.only if args.only else list(all_cases.keys())
    selected = [c for c in selected if c not in args.skip and c in all_cases]
    print(f"Recording {len(selected)} cases concurrently (workers={args.workers}):")
    for c in selected:
        print(f"  - {c}")
    print()

    results: list[dict[str, Any]] = []
    t0 = time.time()
    with ThreadPoolExecutor(max_workers=args.workers) as ex:
        futures = {ex.submit(record_one, name, all_cases[name]): name for name in selected}
        for fut in as_completed(futures):
            name = futures[fut]
            try:
                r = fut.result()
                results.append(r)
                print(f"  [done] {name}: {r}")
            except Exception as e:  # noqa: BLE001
                traceback.print_exc()
                results.append({"name": name, "ok": False, "error": repr(e)})
                print(f"  [FAIL] {name}: {e}")

    print()
    print("=" * 60)
    print(f"SUMMARY ({time.time()-t0:.1f}s wall)")
    print("=" * 60)
    ok = sum(1 for r in results if r.get("ok"))
    for r in sorted(results, key=lambda x: x["name"]):
        flag = "✓" if r.get("ok") else "✗"
        print(f"  {flag} {r['name']:20}  {r}")
    print(f"\n{ok}/{len(results)} ok")
    if ok != len(results):
        sys.exit(1)


if __name__ == "__main__":
    main()
