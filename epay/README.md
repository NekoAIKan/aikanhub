# EPay-Alipay Gateway

This folder owns the minimal EPay-compatible proxy for kittyvibe.ai.

The gateway lets the main New API application keep using its existing EPay integration while the actual settlement goes through an approved Alipay web application.

## Runtime Flow

```text
kittyvibe.ai New API
  -> pay.kittyvibe.ai /submit.php
  -> Alipay alipay.trade.page.pay
  -> pay.kittyvibe.ai /alipay/notify
  -> kittyvibe.ai /api/user/epay/notify
```

## Boundaries

- New API stores only `EpayId` and `EpayKey`.
- This gateway stores Alipay AppID, app private key, Alipay public key, and seller ID.
- Alipay never calls New API directly.
- New API never stores Alipay private keys.

## New API Configuration

```text
ServerAddress = https://kittyvibe.ai
PayAddress = https://pay.kittyvibe.ai
EpayId = kittyvibe
EpayKey = <same value as gateway EPAY_KEY>
PayMethods = [{"name":"支付宝","type":"alipay","color":"#1677FF"}]
CustomCallbackAddress = empty
```

## Gateway Environment

```text
EPAY_PID=kittyvibe
EPAY_KEY=<openssl rand -hex 32>
PUBLIC_BASE_URL=https://pay.kittyvibe.ai
ALLOWED_NOTIFY_HOSTS=kittyvibe.ai
ALIPAY_APP_ID=202100...
ALIPAY_APP_PRIVATE_KEY=<PEM or base64 DER>
ALIPAY_PUBLIC_KEY=<PEM or base64 DER>
ALIPAY_SELLER_ID=<optional seller id>
ALIPAY_GATEWAY=https://openapi.alipay.com/gateway.do
ORDER_STORE_PATH=/var/lib/epay-gateway/orders.json
LISTEN_ADDR=:3001
```

Run locally:

```bash
go test ./epay
go run ./epay
```
