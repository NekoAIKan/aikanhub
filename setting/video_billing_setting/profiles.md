# Video Billing Profiles

Operator handover for `video_billing_setting.profiles` — the per-model JSON
config that drives video task billing. The repository ships **no** vendor
prices, fallbacks, or markup defaults; everything below must be set at
runtime by the operator (or never bill).

## Where it lives

- **DB:** `Option` table, key `video_billing_setting.profiles` (string-encoded JSON).
- **Admin UI:** `系统设置 → 常规 → 积分与利润 → 视频计费配置` (the textarea).
- **Live preview:** `POST /api/option/video_billing/preview` (RootAuth) — see
  the "视频计费预览（实时）" panel on the same page.
- **Runtime read:** `setting/video_billing_setting.GetProfile(model)` — returns
  `(profile, false)` when the model has no entry. The `relay/relay_task.go`
  dispatcher routes any model with a profile + `mode == "formula"` to the
  per-token formula path; everything else falls through to `ModelPrice`.

## JSON shape

Each map key is a model id; the value is the billing recipe.

```json
{
  "doubao-seedance-2-0-260128": {
    "mode": "formula",
    "unit_price": 7.89,
    "unit_price_with_video": 4.80,
    "unit_price_by_resolution": {"480p": 7.89, "720p": 7.89, "1080p": 8.74},
    "unit_price_with_video_by_resolution": {"480p": 4.80, "720p": 4.80, "1080p": 5.31},
    "min_tokens_with_video": 108000,
    "fallback_fps": 24,
    "fallback_width": 1280,
    "fallback_height": 720,
    "fallback_duration_seconds": 5,
    "use_upstream_usage": true,
    "conservative_multiplier": 1.25,
    "reference_conservative_multiplier": 2.0,
    "draft_multiplier": 0.5,
    "resolution_aliases": {
      "480p":  {"width": 832,  "height": 480},
      "720p":  {"width": 1280, "height": 720},
      "1080p": {"width": 1920, "height": 1080}
    }
  }
}
```

### Field reference

| Field | Type | Required | Meaning |
|---|---|---|---|
| `mode` | string | yes | `"formula"` routes through `service.CalculateVideoBilling`. Anything else (or empty) → falls back to the legacy `ModelPrice` path on the model_ratio admin page. |
| `unit_price` | float | one of the four\* | Default rate, USD per 1M tokens. Used when no resolution-specific or with-video override matches. |
| `unit_price_with_video` | float | optional | Default rate when the request carries reference video. Lower than `unit_price` for vendors that discount video-input. |
| `unit_price_by_resolution` | object | optional | Map alias → $/1M. Keys must match the `resolution` alias surfaced by the request (e.g. `"720p"`, `"1080p"`). |
| `unit_price_with_video_by_resolution` | object | optional | With-video variant of `unit_price_by_resolution`. Highest precedence. |
| `min_tokens_with_video` | int | optional | Token floor applied **only** when the request has reference media. Mirrors vendor minimums on video-input requests. |
| `fallback_fps` / `fallback_width` / `fallback_height` / `fallback_duration_seconds` | int | yes | Used when the request omits a parameter. Set to your vendor's most common defaults. |
| `use_upstream_usage` | bool | recommended `true` | When the upstream response reports `usage.completion_tokens`, prefer it over the formula. |
| `conservative_multiplier` | float | recommended `≥ 1` | Pre-charge inflation (e.g. 1.25 = pre-hold 125%). Returned to the user on settle. |
| `reference_conservative_multiplier` | float | recommended `≥ conservative_multiplier` | Same, but used when the request has reference video (because the actual upstream cost variance is wider). |
| `draft_multiplier` | float | optional | Multiplier applied when the request includes `metadata.draft = true`. Typical: 0.5. |
| `resolution_aliases` | object | yes if you use `resolution` strings | Maps human aliases (`"720p"`) to width/height. The runtime resolves the request alias here to compute tokens. |

\* If none of `unit_price` / `unit_price_with_video` / `unit_price_by_resolution` /
`unit_price_with_video_by_resolution` is set and `profit_setting.upstream_cost_per_million_tokens`
is also zero, the resolved unit price will be **0** — the request bills as free.
This is intentional: an OSS binary should not silently bill at a magic-number
fallback. Configure at least one rate, or accept free billing.

## Token formula

```
tokens = (input_seconds + output_seconds) × width × height × fps / 1024
```

If `use_upstream_usage` is `true` AND the upstream response includes
`usage.completion_tokens > 0`, that value replaces the formula result.

The min-token floor is applied **after** both branches:

```
if has_reference_media and tokens < min_tokens_with_video:
    tokens = min_tokens_with_video
```

## Unit price precedence

When resolving `RetailUnitPrice` ($/1M tokens):

1. `has_reference_media` AND `unit_price_with_video_by_resolution[resolution]` exists → use it
2. `has_reference_media` AND `unit_price_with_video > 0` → use it
3. `unit_price_by_resolution[resolution]` exists → use it
4. `unit_price > 0` → use it
5. `profit_setting.upstream_cost_per_million_tokens > 0` AND
   `profit_setting.apply_to_default_video_profiles == true` →
   `upstream_cost × (1 + markup_percent/100)` (the deployment-wide derived rate)
6. Otherwise → **0** (request bills free)

## Quota conversion

After the unit price resolves:

```
raw_cost  = tokens × unit_price                  (USD)
quota     = raw_cost / 1_000_000 × QuotaPerUnit × group_ratio
            (QuotaPerUnit = 500_000 by default, i.e. 1 USD = 500_000 quota)
```

Pre-charge applies `conservative_multiplier` (or
`reference_conservative_multiplier` when reference media is present) on top.
Settle re-runs without the multiplier, refunding the difference.

## Verifying a profile

After saving, dispatch a real request and:

1. Open the admin live preview at the bottom of the credits settings page,
   pick the model, confirm the matrix matches what you expect.
2. Hit the API with the same shape and check the dashboard usage log's
   billing breakdown — `unit_price_per_million`, `tokens`, and
   `actual_quota` should all match what the live preview showed.

The test suite asserts the same numbers in two layers:

- `service.TestSeedanceFixturePricingMatrix` — the billing engine itself.
- `controller.TestPreviewVideoBilling_SeedanceMatrix` — the wire format
  the admin UI reads.

If you change the formula, `QuotaPerUnit`, or the resolver precedence,
expect both tests to fail at the same line.

## Migration notes (from the pre-extension schema)

If you were running an earlier build that shipped a `defaultSeedanceProfile()`
fallback (i.e. seedance models worked without a configured profile), this is
a **breaking change** in the sense that:

- After the upgrade, `GetProfile(model)` returns `false` for any model with no
  explicit entry in `video_billing_setting.profiles`. The dispatcher then falls
  through to the `ModelPrice` ratio path. If `ModelPrice` is also unset, the
  request errors at billing rather than running free.
- Prior `defaultSeedanceProfile()` parameters (24 fps, 1280×720, 1.25/2.0/0.5)
  are no longer applied implicitly. If you relied on them, reproduce them in
  the JSON for each affected model.

The recommended deploy sequence:

1. Pre-deploy: PUT the `video_billing_setting.profiles` value via
   `/api/option/` with profiles for every model that was relying on the old
   defaults. This takes effect immediately — no restart needed.
2. Deploy the new binary.
3. Verify with a test request and the live preview panel.

The hardcoded `videoInputRatioMap` (`28/46`, `22/37`) in
`relay/channel/task/doubao/constants.go` is removed in this revision; replace
it by setting `unit_price_with_video` and/or
`unit_price_with_video_by_resolution` per profile.

## Pricing in this repo

This file documents the schema only. Specific prices are operator data and
should never be committed to the repository. Inject them via the admin UI,
`/api/option/` PUT, or a deployment-time secret-injection step — whichever
your ops setup uses. A fork that publishes prices in source code becomes a
maintenance trap when vendor rates move.
