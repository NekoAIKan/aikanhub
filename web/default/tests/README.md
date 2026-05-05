# Seedance E2E test harness

Drives every documented Seedance endpoint via both the OpenAI SDK and the
Volcano Ark SDK, against a running aikanhub instance.

## Prerequisites

```bash
# 1. aikanhub running on $AIKANHUB_BASE_URL (default http://localhost:3000)
#    with a Doubao channel configured.
# 2. Token in .env.maomao (one level up):
#       AIKANHUB_TOKEN=sk-...

cd aikanhub/web/default/tests
python3 -m venv .venv && source .venv/bin/activate
pip install openai 'volcengine-python-sdk[ark]' httpx python-dotenv

# Pull a small set of reusable assets (images/video/audio) into ./assets/
bash fetch-assets.sh

python run.py                        # all checks
python run.py text                   # a single check by name
python run.py --skip i2v_firstlast   # skip one
```

## What it covers

| Check                  | SDK          | Endpoint                                                |
|------------------------|--------------|---------------------------------------------------------|
| `text`                 | OpenAI       | `POST /v1/videos`                                       |
| `i2v_first`            | OpenAI       | `POST /v1/videos` with `images[]`                       |
| `i2v_firstlast`        | Volcano Ark  | `POST /api/v3/contents/generations/tasks`               |
| `multimodal`           | Volcano Ark  | …                                                       |
| `edit`                 | Volcano Ark  | …                                                       |
| `extend`               | Volcano Ark  | …                                                       |
| `web_search`           | Volcano Ark  | …                                                       |
| `poll_openai`          | OpenAI       | `GET /v1/videos/{id}`                                   |
| `poll_volc`            | Volcano Ark  | `GET /api/v3/contents/generations/tasks/{id}`           |
| `download`             | httpx        | `GET /v1/videos/{id}/content`                           |
| `error_missing_model`  | raw HTTP     | 400 path                                                |
| `error_unknown_id`     | raw HTTP     | 404 path                                                |
| `error_no_auth`        | raw HTTP     | 401 path                                                |

The runner submits each generation, polls until success/failure (or 5 min),
and writes any downloaded videos under `./out/`.

## Asset URLs

Volcano upstream fetches reference URLs on its own side, so we need
**publicly reachable** URLs. `fetch-assets.sh` mirrors a small set into
`./assets/` for local inspection, but the test harness uses the upstream URLs
directly (so a flaky local copy doesn't break the test).
