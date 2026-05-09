# E2E test results (Seedance routes)

Run against `kittyvibe-app` docker container on `http://localhost:3000`,
token from `aikanhub/.env.maomao`. Each row is a real generation against the
Doubao upstream — no mocks.

## Summary

11/11 ✅ — full Doubao upstream round-trip on every flow.

| Check                 | SDK         | Result | Time   | Notes |
|-----------------------|-------------|--------|--------|-------|
| `text`                | OpenAI      | ✅      | 189 s  | TOS URL returned |
| `i2v_first`           | OpenAI      | ✅      | 158 s  | `extra_body={"images":[...]}` works |
| `i2v_firstlast`       | Volcano Ark | ✅      | 170 s  | Two-image content[] |
| `multimodal`          | Volcano Ark | ✅      | 439 s  | 2 images + 1 video + 1 audio |
| `edit`                | Volcano Ark | ✅      | 345 s  | image + video reference |
| `extend`              | Volcano Ark | ✅      | 350 s  | single 10 s ref clip |
| `web_search`          | Volcano Ark | ✅      | 287 s  | `tools=[{"type":"web_search"}]` works |
| `download`            | httpx       | ✅      | 127 s  | Saved 1.8 MB mp4 to `out/` |
| `error_missing_model` | raw HTTP    | ✅      | 0 s    | 400 |
| `error_unknown_id`    | raw HTTP    | ✅      | 1.6 s  | 404 |
| `error_no_auth`       | raw HTTP    | ✅      | 0 s    | 401 |

## Issues found and fixed during testing

### 1. `NOT_START` mapped to "unknown" instead of "queued"

When the OpenAI SDK polled an immediately-after-submit task it saw
`status="unknown"`, because `model.TaskStatus.ToVideoStatus()` had no case
for the brand-new `NOT_START` status. Fixed in
[`model/task.go`](../../../model/task.go) by adding `TaskStatusNotStart` to
the `queued` arm.

### 2. Volcano Ark SDK error envelope mismatch

When the upstream Volcano API rejected a request (e.g. unfetchable image
URL), our gateway returned the internal `dto.TaskError` shape
(`{code, message, data}`) verbatim. The Volcano SDK expects
`{error: {code, message}}` — without it, `ArkBadRequestError.message` is a
huge ugly blob.

Fixed in [`middleware/volcengine_ark_adapter.go`](../../../middleware/volcengine_ark_adapter.go):
when status ≥ 400 and the body has `code`/`message` (but no `error`), the
middleware re-envelopes it before flushing.

After fix, the SDK exception cleanly surfaces:

```text
ArkBadRequestError: Error code: 400 - {'error': {'code': 'fail_to_fetch_task',
'message': '<upstream Volcano error JSON>'}}
```

### 3. Test asset bugs — not gateway bugs

The first batch picked URLs that violated Seedance's input limits:

| File                      | Property        | Limit   |
|---------------------------|-----------------|---------|
| `mov_bbb.mp4` (320×240)   | side ≥ 300 px   | failed  |
| `mov_bbb.mp4` pixel area  | ≥ 409600 px²    | failed  |
| `horse.mp3` duration      | ≥ 1.8 s         | failed  |
| 3× 10 s ref videos = 30 s | total ≤ 15.2 s  | failed  |

Replaced with `Big_Buck_Bunny_720_10s_1MB.mp4` (720p, 1 MB) and
`piano2.wav` (~7 s), and trimmed `extend` to a single 10 s clip.

## Reproducing

```bash
cd aikanhub/web/default/tests
python3 -m venv .venv && source .venv/bin/activate
pip install openai 'volcengine-python-sdk[ark]' httpx python-dotenv
bash fetch-assets.sh
python run.py
```

Token comes from `../../../.env.maomao` (`KITTYVIBE_TOKEN=sk-...`). Override
endpoint with `KITTYVIBE_BASE_URL`. Per-task timeout is 900 s — heavier flows
(multimodal, edit) routinely take 5–10 min on the Doubao side.
