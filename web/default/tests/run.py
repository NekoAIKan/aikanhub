"""Seedance E2E test runner.

Exercises every endpoint documented under web/default/src/features/docs/
against a running kittyvibe instance. Submits each generation, polls until
success/failure (5 min cap), and writes outputs to ./out/.

Usage:
    python run.py                        # run every check
    python run.py text i2v_first         # only the listed checks
    python run.py --skip i2v_firstlast   # all except listed
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import time
import traceback
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Callable

import httpx
from dotenv import load_dotenv
from openai import OpenAI
from volcenginesdkarkruntime import Ark

HERE = Path(__file__).resolve().parent
load_dotenv(HERE.parent.parent.parent / ".env.maomao")

def require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"set {name}")
    return value


BASE = os.environ.get("KITTYVIBE_BASE_URL", "http://localhost:3000")
TOKEN = require_env("KITTYVIBE_TOKEN")
MODEL = os.environ.get("KITTYVIBE_MODEL", "doubao-seedance-2-0-fast-260128")
TIMEOUT = int(os.environ.get("KITTYVIBE_TIMEOUT", "900"))  # seconds per task

OUT = HERE / "out"
OUT.mkdir(exist_ok=True)

openai_client = OpenAI(api_key=TOKEN, base_url=f"{BASE}/v1")
ark_client = Ark(api_key=TOKEN, base_url=f"{BASE}/api/v3")

# Public reference assets (Volcano upstream fetches these).
IMG_FIRST = "https://picsum.photos/seed/aikanhub-first/800/600"
IMG_LAST = "https://picsum.photos/seed/aikanhub-last/800/600"
IMG_REF1 = "https://picsum.photos/seed/aikanhub-ref1/800/600"
IMG_REF2 = "https://picsum.photos/seed/aikanhub-ref2/800/600"
VIDEO_REF = (
    "https://test-videos.co.uk/vids/bigbuckbunny/mp4/h264/720/"
    "Big_Buck_Bunny_720_10s_1MB.mp4"
)
AUDIO_REF = "https://www.kozco.com/tech/piano2.wav"


# ---------------------------------------------------------------------------
# Result tracking
# ---------------------------------------------------------------------------

@dataclass
class CheckResult:
    name: str
    ok: bool
    duration: float
    note: str = ""
    issues: list[str] = field(default_factory=list)


RESULTS: list[CheckResult] = []


def _log(prefix: str, msg: str) -> None:
    print(f"  {prefix:>8}  {msg}", flush=True)


def run_check(name: str, fn: Callable[[], str]) -> None:
    print(f"\n=== {name} ===", flush=True)
    t0 = time.time()
    try:
        note = fn() or ""
        RESULTS.append(CheckResult(name, True, time.time() - t0, note))
        _log("PASS", f"{name} ({time.time() - t0:.1f}s) {note}")
    except Exception as e:  # noqa: BLE001
        traceback.print_exc()
        RESULTS.append(
            CheckResult(name, False, time.time() - t0, repr(e), issues=[repr(e)])
        )
        _log("FAIL", f"{name}: {e}")


# ---------------------------------------------------------------------------
# Polling helpers — both SDKs converge on a "succeeded" / "failed" string.
# ---------------------------------------------------------------------------

def poll_openai(task_id: str) -> dict[str, Any]:
    deadline = time.time() + TIMEOUT
    while time.time() < deadline:
        v = openai_client.videos.retrieve(task_id)
        # OpenAI Video status values: queued / in_progress / completed / failed
        s = v.status
        _log("openai", f"{task_id[:16]} status={s}")
        if s in ("completed", "succeeded"):
            return v.model_dump()
        if s == "failed":
            raise RuntimeError(f"OpenAI poll: task failed: {getattr(v, 'error', None)}")
        time.sleep(5)
    raise TimeoutError(f"task {task_id} did not finish in {TIMEOUT}s (openai)")


def poll_volc(task_id: str) -> dict[str, Any]:
    deadline = time.time() + TIMEOUT
    while time.time() < deadline:
        t = ark_client.content_generation.tasks.get(task_id=task_id)
        s = t.status
        _log("volcano", f"{task_id[:16]} status={s}")
        if s == "succeeded":
            return t.model_dump()
        if s in ("failed", "cancelled"):
            raise RuntimeError(f"Volcano poll: task {s}: {getattr(t, 'error', None)}")
        time.sleep(5)
    raise TimeoutError(f"task {task_id} did not finish in {TIMEOUT}s (volcano)")


# ---------------------------------------------------------------------------
# Checks — each returns a short note (mostly the produced URL) on success.
# ---------------------------------------------------------------------------

def check_text() -> str:
    """OpenAI SDK + plain text-to-video."""
    v = openai_client.videos.create(
        model=MODEL,
        prompt="A ginger cat strolls down a Tokyo street at sunset, 4K, cinematic",
        seconds="5",
        size="720p",
        extra_body={"metadata": {"ratio": "16:9"}},
    )
    final = poll_openai(v.id)
    url = (final.get("metadata") or {}).get("url", "")
    return f"id={v.id} url={url[:80]}"


def check_i2v_first() -> str:
    """OpenAI SDK + image-to-video first frame."""
    v = openai_client.videos.create(
        model=MODEL,
        prompt="Slow camera push-in, subject animates from still to motion",
        seconds="5",
        size="720p",
        extra_body={"images": [IMG_FIRST], "metadata": {"ratio": "16:9"}},
    )
    final = poll_openai(v.id)
    url = (final.get("metadata") or {}).get("url", "")
    return f"id={v.id} url={url[:80]}"


def check_i2v_firstlast() -> str:
    """Volcano SDK + first/last frame."""
    t = ark_client.content_generation.tasks.create(
        model=MODEL,
        content=[
            {"type": "text", "text": "Smooth transition from first frame to last frame"},
            {"type": "image_url", "image_url": {"url": IMG_FIRST}, "role": "first_frame"},
            {"type": "image_url", "image_url": {"url": IMG_LAST}, "role": "last_frame"},
        ],
        duration=5,
        ratio="16:9",
    )
    final = poll_volc(t.id)
    url = (final.get("content") or {}).get("video_url", "")
    return f"id={t.id} url={url[:80]}"


def check_multimodal() -> str:
    """Volcano SDK + 2 images + 1 video + 1 audio."""
    t = ark_client.content_generation.tasks.create(
        model="doubao-seedance-2-0-260128",  # multimodal needs the full model
        content=[
            {"type": "text", "text": "POV beverage commercial in the style of video1, scored with audio1"},
            {"type": "image_url", "image_url": {"url": IMG_REF1}, "role": "reference_image"},
            {"type": "image_url", "image_url": {"url": IMG_REF2}, "role": "reference_image"},
            {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
            {"type": "audio_url", "audio_url": {"url": AUDIO_REF}, "role": "reference_audio"},
        ],
        duration=5,
        ratio="16:9",
        generate_audio=True,
    )
    final = poll_volc(t.id)
    url = (final.get("content") or {}).get("video_url", "")
    return f"id={t.id} url={url[:80]}"


def check_edit() -> str:
    """Volcano SDK + edit-video (image + video)."""
    t = ark_client.content_generation.tasks.create(
        model="doubao-seedance-2-0-260128",
        content=[
            {"type": "text", "text": "Replace the subject in video1 with the object from image1; keep camera motion intact"},
            {"type": "image_url", "image_url": {"url": IMG_REF1}, "role": "reference_image"},
            {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
        ],
        duration=5,
        ratio="16:9",
        generate_audio=True,
    )
    final = poll_volc(t.id)
    url = (final.get("content") or {}).get("video_url", "")
    return f"id={t.id} url={url[:80]}"


def check_extend() -> str:
    """Volcano SDK + extend-video.

    Seedance caps the SUM of reference video durations at 15.2 s, so we send
    one ~10 s clip rather than three. The flow itself supports up to three
    clips totalling ≤ 15 s — see media-limits in the API docs.
    """
    t = ark_client.content_generation.tasks.create(
        model="doubao-seedance-2-0-260128",
        content=[
            {"type": "text", "text": "Continue the scene with cinematic camera motion"},
            {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
        ],
        duration=8,
        ratio="16:9",
        generate_audio=True,
    )
    final = poll_volc(t.id)
    url = (final.get("content") or {}).get("video_url", "")
    return f"id={t.id} url={url[:80]}"


def check_web_search() -> str:
    """Volcano SDK + web_search tool."""
    t = ark_client.content_generation.tasks.create(
        model="doubao-seedance-2-0-260128",
        content=[
            {"type": "text", "text": "Macro shot of a glass frog on an emerald leaf; focus shifts from skin to its transparent belly"},
        ],
        duration=5,
        ratio="16:9",
        generate_audio=True,
        tools=[{"type": "web_search"}],
    )
    final = poll_volc(t.id)
    url = (final.get("content") or {}).get("video_url", "")
    return f"id={t.id} url={url[:80]}"


def check_download(task_id: str | None = None) -> str:
    """GET /v1/videos/:id/content — proxies the upstream signed URL."""
    if not task_id:
        # fall back to a fresh quick task
        v = openai_client.videos.create(
            model=MODEL, prompt="cat walking", seconds="5", size="720p",
            extra_body={"metadata": {"ratio": "16:9"}},
        )
        poll_openai(v.id)
        task_id = v.id
    r = httpx.get(
        f"{BASE}/v1/videos/{task_id}/content",
        headers={"Authorization": f"Bearer {TOKEN}"},
        follow_redirects=True,
        timeout=120,
    )
    r.raise_for_status()
    out = OUT / f"{task_id}.mp4"
    out.write_bytes(r.content)
    return f"saved {len(r.content)} bytes -> {out.relative_to(HERE)}"


# ---------------------------------------------------------------------------
# Negative paths — make sure the new routes report sensible errors.
# ---------------------------------------------------------------------------

def check_error_missing_model() -> str:
    r = httpx.post(
        f"{BASE}/api/v3/contents/generations/tasks",
        headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"},
        json={"content": [{"type": "text", "text": "x"}]},
    )
    if r.status_code != 400:
        raise AssertionError(f"expected 400, got {r.status_code}: {r.text}")
    return f"HTTP 400 ({r.json()})"


def check_error_unknown_id() -> str:
    r = httpx.get(
        f"{BASE}/api/v3/contents/generations/tasks/task_does_not_exist_xxx",
        headers={"Authorization": f"Bearer {TOKEN}"},
    )
    if r.status_code != 404:
        raise AssertionError(f"expected 404, got {r.status_code}: {r.text}")
    return f"HTTP 404 ({r.json()})"


def check_error_no_auth() -> str:
    r = httpx.get(f"{BASE}/api/v3/contents/generations/tasks/anything")
    if r.status_code != 401:
        raise AssertionError(f"expected 401, got {r.status_code}: {r.text}")
    return f"HTTP 401 ({r.json()})"


# ---------------------------------------------------------------------------
# Action labels — submission-only smoke test for the dashboard "task action"
# column. Each entry submits a request shaped like the corresponding flow
# and asserts the gateway picked the right `action` label. We use the
# internal /v1/video/generations/:id endpoint to read the stored row.
# ---------------------------------------------------------------------------

def _submit_volc(content: list[dict[str, Any]], **kwargs: Any) -> str:
    t = ark_client.content_generation.tasks.create(
        model=kwargs.pop("model", "doubao-seedance-2-0-260128"),
        content=content,
        duration=kwargs.pop("duration", 5),
        ratio=kwargs.pop("ratio", "16:9"),
        **kwargs,
    )
    return t.id


def _read_action(task_id: str) -> str:
    r = httpx.get(
        f"{BASE}/v1/video/generations/{task_id}",
        headers={"Authorization": f"Bearer {TOKEN}"},
    )
    r.raise_for_status()
    return r.json()["data"]["action"]


def check_action_labels() -> str:
    """Submit one of each flow and assert the stored action is correct."""
    cases = [
        ("textGenerate", [{"type": "text", "text": "a cat walks down the street"}]),
        ("generate", [
            {"type": "text", "text": "x"},
            {"type": "image_url", "image_url": {"url": IMG_FIRST}, "role": "first_frame"},
        ]),
        ("firstTailGenerate", [
            {"type": "text", "text": "x"},
            {"type": "image_url", "image_url": {"url": IMG_FIRST}, "role": "first_frame"},
            {"type": "image_url", "image_url": {"url": IMG_LAST}, "role": "last_frame"},
        ]),
        ("referenceGenerate", [
            {"type": "text", "text": "x"},
            {"type": "video_url", "video_url": {"url": VIDEO_REF}, "role": "reference_video"},
        ]),
    ]
    notes = []
    for expected, content in cases:
        tid = _submit_volc(content)
        time.sleep(1)  # let the row land
        actual = _read_action(tid)
        if actual != expected:
            raise AssertionError(f"expected action={expected!r}, got {actual!r} for {tid}")
        notes.append(f"{expected}={tid[:14]}")
    return "; ".join(notes)


# ---------------------------------------------------------------------------
# Registry + main
# ---------------------------------------------------------------------------

CHECKS: dict[str, Callable[[], str]] = {
    "text": check_text,
    "i2v_first": check_i2v_first,
    "i2v_firstlast": check_i2v_firstlast,
    "multimodal": check_multimodal,
    "edit": check_edit,
    "extend": check_extend,
    "web_search": check_web_search,
    "download": check_download,
    "error_missing_model": check_error_missing_model,
    "error_unknown_id": check_error_unknown_id,
    "error_no_auth": check_error_no_auth,
    "action_labels": check_action_labels,
}


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("only", nargs="*", help="run only these checks")
    p.add_argument("--skip", nargs="*", default=[], help="skip these checks")
    args = p.parse_args()

    selected = list(args.only) if args.only else list(CHECKS.keys())
    selected = [c for c in selected if c not in args.skip]

    print(f"BASE = {BASE}")
    print(f"MODEL = {MODEL}")
    print(f"checks = {selected}")

    for name in selected:
        fn = CHECKS.get(name)
        if not fn:
            print(f"  unknown check: {name}", file=sys.stderr)
            continue
        run_check(name, fn)

    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)
    for r in RESULTS:
        flag = "✓" if r.ok else "✗"
        print(f"  {flag} {r.name:25}  {r.duration:6.1f}s  {r.note[:90]}")
    fails = [r for r in RESULTS if not r.ok]
    if fails:
        print(f"\n{len(fails)} failure(s)")
        sys.exit(1)


if __name__ == "__main__":
    main()
