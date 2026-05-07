# Platform Money Pricing Migration Progress

This document tracks the implementation status for PR #33 on branch `codex/platform-money-pricing`.

## Goal

Migrate live billing and user-facing balances away from quota toward money-denominated accounting while keeping historical quota data readable.

## Progress Log

### 2026-05-07: Foundation

- Added money micros and FX conversion helpers.
- Added FX snapshot model.
- Added money wallet ledger with top-up, preauth, settle, release, refund, and adjustment transactions.
- Added non-video money usage pricing profile validation and quote calculation.
- Added channel cost and retail pricing policy persistence.
- Added pure legacy pricing migration helpers for `ModelPrice`, `ModelRatio`, `CompletionRatio`, and `GroupRatio`.
- Added `billing_setting.money_billing_mode` and `billing_setting.settlement_currency`.
- Top-up completion paths credit money wallets. In `money` mode they no longer mirror funds into `User.Quota`.

Verification:

- `rtk go test ./model ./service ./service/money_pricing ./setting/billing_setting -count=1 -timeout=180s`
- `rtk go test ./...`

### Current Focus

- Add money-denominated API token budget fields.
- Migrate grants and admin balance entry points to money wallet transactions.

### 2026-05-07: Quota Backfill And Guard Rails

- Added quota-to-money conversion helper using the historical `QuotaPerUnit` conversion.
- Added backfill preview across user quota, token remaining quota, and subscription remaining quota.
- Added idempotent user-wallet backfill apply with `source=quota_backfill` transaction metadata.
- Added `money` mode guard for `IncreaseUserQuota` and `DecreaseUserQuota`.

Verification:

- `rtk go test ./service ./model ./setting/billing_setting -run 'QuotaBackfill|MoneyBillingMode|QuotaGuard|QuotaWrite|MoneyWallet|RechargeMoneyMode' -count=1 -timeout=180s`

### Current Focus

- Add money-denominated API token budget fields.
- Migrate grants and admin balance entry points to money wallet transactions.

## Remaining Acceptance Items

- Chat, responses, image, audio, tool/search, violation-fee, and video billing paths freeze and settle money wallet balances.
- API token and subscription budgets are money-denominated.
- Redemption, invite, affiliate, check-in, onboarding, subscription, admin adjustment, and transfer flows write money wallet transactions.
- Pricing pages show money units and examples.
- Historical quota reports remain readable and clearly labeled as legacy data.
