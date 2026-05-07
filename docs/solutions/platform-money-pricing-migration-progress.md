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

### 2026-05-07: Runtime Money Wallet Settlement Bridge

- Wallet billing sessions now freeze money wallet balances in money mode, settle the frozen amount on success, release on refund, and adjust for actual usage differences.
- Token pre-consume and post-consume paths now use token money budgets in money mode.
- Legacy `PreConsumeQuota` / `PostConsumeQuota` fallbacks avoid `User.Quota` writes in money mode by adjusting money wallet balances.
- Trust-bypass preconsume is disabled in money mode so paid requests freeze funds before upstream submission.
- Added service coverage for exact settlement, over-estimate refund, and under-estimate supplement charge.

Verification:

- `rtk go test ./service -run 'MoneyBillingSession|QuotaBackfill|MoneyWallet|Token' -count=1 -timeout=240s`
- `rtk go test ./service ./model ./controller ./setting/billing_setting -run 'Money|QuotaBackfill|Token|Billing|PostConsume|PreConsume|Redeem|Checkin|TransferAff|Recharge' -count=1 -timeout=360s`
- `rtk go test ./service -count=1 -timeout=360s`

### 2026-05-07: Grant Flow Money Mode

- Added shared grant helpers that convert legacy quota grants into settlement-currency money micros when money billing mode is enabled.
- Money mode now routes redemption, check-in, onboarding, invitee reward, affiliate transfer, and admin add/subtract/override balance changes to the money wallet instead of `User.Quota`.
- Legacy mode keeps the existing quota write behavior for compatibility.
- Added coverage for money-mode grant helper, redemption, check-in, affiliate transfer, onboarding, and admin wallet adjustment flows.

Verification:

- `rtk go test ./model -run 'GrantUserQuotaOrMoney|RedeemCreditsMoney|CheckinCreditsMoney|TransferAffQuotaCreditsMoney|UserInsertOnboardingCreditsMoney|AdminLegacyQuotaWallet' -count=1 -timeout=180s`
- `rtk go test ./model ./service ./controller ./setting/billing_setting ./setting/onboarding_setting -run 'Money|QuotaBackfill|Redeem|Checkin|TransferAff|Register|Token|Topup|Recharge|Onboarding' -count=1 -timeout=300s`
- `rtk go test ./model -count=1 -timeout=300s`

### 2026-05-07: API Token Money Budgets

- Added money-denominated API token budget fields: remaining amount, used amount, currency, and unlimited amount.
- Token create/update/status/usage endpoints now preserve and expose money budget fields while keeping legacy quota response fields for compatibility.
- `ValidateUserToken` now checks token money budget exhaustion when `billing_setting.money_billing_mode=money`.
- Token quota mutation helpers now reject writes in money mode.
- Added token money budget debit/refund helpers with insufficient-budget protection.

Verification:

- `rtk go test ./model -run 'Token.*Money|TokenQuotaWrites|ValidateUserTokenUsesMoneyBudget' -count=1 -timeout=180s`
- `rtk go test ./controller -run 'Token.*Money|GetTokenUsageIncludesMoney|GetTokenStatusIncludesMoney|AddTokenStoresMoney|UpdateTokenStoresMoney' -count=1 -timeout=180s`
- `rtk go test ./model ./controller ./service ./setting/billing_setting -run 'Token|MoneyBillingMode|QuotaBackfill|MoneyWallet|RechargeMoneyMode' -count=1 -timeout=240s`

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
