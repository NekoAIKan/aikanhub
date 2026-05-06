"""Replay-mode upstream Volcano Ark mock.

Listens on :8721 (override with MOCK_PORT) and serves:

  POST /api/v3/contents/generations/tasks
  GET  /api/v3/contents/generations/tasks/{id}
  GET  /storage/{case}/video.mp4

Routes incoming requests to the right fixture in ./fixtures/{case}/ by inspecting
the request shape (first_frame + last_frame ⇒ i2v_firstlast, audio_url ⇒
multimodal, etc.). Each task progresses queued → running → succeeded across
three polls so the gateway exercises the same loop it does against real
Volcano.

Used by docker-side aikanhub: configure a Channel with
  base_url = http://host.docker.internal:8721
The api_key only needs to be non-empty (we don't validate Bearer tokens).
"""
from __future__ import annotations

import json
import os
import re
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Lock
from typing import Any

HERE = Path(__file__).resolve().parent
FIX = HERE / "fixtures"
PORT = int(os.environ.get("MOCK_PORT", "8721"))
PUBLIC_BASE = os.environ.get("MOCK_PUBLIC_BASE", f"http://host.docker.internal:{PORT}")

# How many polls before "succeeded". 0 = first poll already succeeded.
POLLS_BEFORE_DONE = int(os.environ.get("MOCK_POLLS", "1"))


# ---------------------------------------------------------------------------
# Fixture loading
# ---------------------------------------------------------------------------

def _load_fixtures() -> dict[str, dict[str, Any]]:
    out: dict[str, dict[str, Any]] = {}
    for d in sorted(FIX.iterdir()):
        if not d.is_dir() or d.name.startswith("_"):
            continue
        meta_path = d / "meta.json"
        final_path = d / "final.json"
        if not meta_path.exists():
            continue
        meta = json.loads(meta_path.read_text())
        request = json.loads((d / "request.json").read_text())
        submit = json.loads((d / "submit.json").read_text()) if (d / "submit.json").exists() else None
        final = json.loads(final_path.read_text()) if final_path.exists() else None
        out[d.name] = {
            "name": d.name,
            "request": request,
            "submit": submit,
            "final": final,
            "meta": meta,
            "video_path": d / "video.mp4",
        }
    return out


FIXTURES = _load_fixtures()
print(f"[mock] loaded {len(FIXTURES)} fixtures: {list(FIXTURES)}")


# ---------------------------------------------------------------------------
# Request → fixture matching
# ---------------------------------------------------------------------------

def _content_signals(req: dict[str, Any]) -> dict[str, bool]:
    signals = {
        "has_first_frame": False,
        "has_last_frame": False,
        "has_image": False,
        "has_video": False,
        "has_audio": False,
        "has_tools": bool(req.get("tools")),
        "is_1080p": (req.get("resolution") == "1080p"),
        "is_fast_model": "fast" in (req.get("model") or ""),
    }
    for item in req.get("content", []) or []:
        t = item.get("type")
        role = item.get("role", "")
        if t == "image_url":
            signals["has_image"] = True
            if role == "first_frame":
                signals["has_first_frame"] = True
            elif role == "last_frame":
                signals["has_last_frame"] = True
        elif t == "video_url":
            signals["has_video"] = True
        elif t == "audio_url":
            signals["has_audio"] = True
    return signals


def match_fixture(req: dict[str, Any]) -> tuple[str | None, str | None]:
    """Return (case_name, error) — error non-empty when we want to reply 400.

    Order matters: more specific signals first.
    """
    s = _content_signals(req)

    # Submit-time validation failure: fast model + 1080p.
    if s["is_fast_model"] and s["is_1080p"]:
        return "fail_fast_1080p", None

    # Multimodal: any audio_url.
    if s["has_audio"]:
        return "multimodal", None
    # Edit: image AND video reference.
    if s["has_image"] and s["has_video"]:
        return "edit", None
    # Extend: video reference, no image.
    if s["has_video"]:
        return "extend", None
    # First-tail.
    if s["has_first_frame"] and s["has_last_frame"]:
        return "i2v_firstlast", None
    # Image only.
    if s["has_image"]:
        return "i2v_first", None
    # Web search.
    if s["has_tools"]:
        return "web_search", None
    # Resolution-driven text-to-video.
    if s["is_1080p"]:
        return "text_1080p_5s", None
    return "text_720p_5s", None


# ---------------------------------------------------------------------------
# In-memory task tracking
# ---------------------------------------------------------------------------

class TaskState:
    def __init__(self, fixture_name: str):
        self.fixture_name = fixture_name
        self.created_at = int(time.time())
        self.poll_count = 0


_tasks: dict[str, TaskState] = {}
_lock = Lock()


def _video_url_for(case_name: str) -> str:
    return f"{PUBLIC_BASE}/storage/{case_name}/video.mp4"


# ---------------------------------------------------------------------------
# HTTP handler
# ---------------------------------------------------------------------------

class Handler(BaseHTTPRequestHandler):
    def _json(self, status: int, body: dict[str, Any]) -> None:
        data = json.dumps(body, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, fmt: str, *args: Any) -> None:  # noqa: A003
        ts = time.strftime("%H:%M:%S")
        print(f"[mock {ts}] {self.command} {self.path} -> {fmt % args}", flush=True)

    def do_POST(self) -> None:  # noqa: N802
        if self.path != "/api/v3/contents/generations/tasks":
            self._json(404, {"error": {"code": "NotFound", "message": self.path}})
            return
        n = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(n) or b"{}")
        case_name, _err = match_fixture(body)
        if not case_name or case_name not in FIXTURES:
            self._json(400, {"error": {"code": "FixtureMissing",
                                        "message": f"no fixture matches request: {body}"}})
            return
        fx = FIXTURES[case_name]
        # Special case: failure fixture replays the recorded 400.
        if case_name == "fail_fast_1080p":
            sub = (fx["submit"] or {}).get("body") or {}
            self._json(400, sub)
            return
        task_id = f"mock-cgt-{uuid.uuid4().hex[:14]}"
        with _lock:
            _tasks[task_id] = TaskState(case_name)
        self._json(200, {"id": task_id})

    def do_GET(self) -> None:  # noqa: N802
        m = re.match(r"^/api/v3/contents/generations/tasks/([^/?]+)/?$", self.path)
        if m:
            self._handle_poll(m.group(1))
            return
        m = re.match(r"^/storage/([^/]+)/video\.mp4$", self.path)
        if m:
            self._handle_video(m.group(1))
            return
        self._json(404, {"error": {"code": "NotFound", "message": self.path}})

    def _handle_poll(self, task_id: str) -> None:
        with _lock:
            state = _tasks.get(task_id)
        if state is None:
            self._json(404, {"error": {"code": "TaskNotFound",
                                        "message": f"task {task_id} not found"}})
            return
        with _lock:
            state.poll_count += 1
            cur = state.poll_count
        fx = FIXTURES[state.fixture_name]
        final = fx["final"]
        if cur <= POLLS_BEFORE_DONE:
            self._json(200, {
                "id": task_id,
                "model": final.get("model"),
                "status": "running" if cur > 0 else "queued",
                "created_at": state.created_at,
                "updated_at": int(time.time()),
            })
            return
        # Replay final, but rewrite the task id and the video_url to point at us.
        replay = json.loads(json.dumps(final))
        replay["id"] = task_id
        if (replay.get("content") or {}).get("video_url"):
            replay["content"]["video_url"] = _video_url_for(state.fixture_name)
        self._json(200, replay)

    def _handle_video(self, case_name: str) -> None:
        fx = FIXTURES.get(case_name)
        if fx is None or not fx["video_path"].exists():
            self._json(404, {"error": {"code": "VideoMissing", "message": case_name}})
            return
        data = fx["video_path"].read_bytes()
        self.send_response(200)
        self.send_header("Content-Type", "video/mp4")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def main() -> None:
    print(f"[mock] listening on http://0.0.0.0:{PORT}  (public base {PUBLIC_BASE})")
    print(f"[mock] polls-before-done = {POLLS_BEFORE_DONE}")
    ThreadingHTTPServer(("0.0.0.0", PORT), Handler).serve_forever()


if __name__ == "__main__":
    main()
