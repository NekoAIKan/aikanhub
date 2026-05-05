# 快速上手

从注册到第一次成功生成视频，约 5 分钟。

## 1. 注册账户

访问 AIKanHub 官网，使用邮箱或第三方账号注册。

- **若你有邀请码**：在注册表单的"邀请码 (Invitation / Aff Code)"字段填入。
  - 用 Beta 邀请码注册可直接进入 Beta 测试组（钱包额度与权限按 [Beta Program](./beta-program.md) 自动配置）。
  - 不填邀请码也能注册，进入默认的 Trial 体验组。
- **邮箱验证**：若管理员开启了邮箱验证，请到注册邮箱完成确认。

## 2. 拿到你的 API Key

登录后进入 **个人中心 → 令牌 (Tokens)**。

1. 点击"添加令牌"。
2. 设定一个名字（例如 `cli-test`）和额度上限（可设为 `无限制 / Unlimited`，则等同于钱包余额）。
3. 保存后，从列表里复制以 `sk-` 开头的 Key。

> 一个账户可以创建多个 Key，每个 Key 都共享你账户的 `User.Group` 权限和钱包余额。Key 的额度只是软限制，不能突破账户钱包。

## 3. 第一次调用 —— 生成一段 720p 视频

视频生成是异步的：先提交任务，再轮询状态。

### 提交任务

```bash
curl -X POST https://YOUR_AIKANHUB_HOST/v1/video/generations \
  -H "Authorization: Bearer sk-YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-1-0-lite-t2v",
    "prompt": "A red panda surfing on a wave at sunset, cinematic",
    "resolution": "720p",
    "duration": 5,
    "ratio": "16:9"
  }'
```

返回：

```json
{ "id": "task_abc123" }
```

### 轮询状态

```bash
curl https://YOUR_AIKANHUB_HOST/v1/tasks/task_abc123 \
  -H "Authorization: Bearer sk-YOUR_TOKEN"
```

返回（成功时）：

```json
{
  "status": "succeeded",
  "content": { "video_url": "https://..." },
  "usage": { "completion_tokens": 35200 }
}
```

每 3–5 秒轮询一次，直到 `status` 变为 `succeeded` 或 `failed`。完整字段说明见 [Video Generation API](./api-video-generation.md)。

## 4. 查看余额

控制台首页显示当前钱包积分。每次生成扣费在任务完成后结算（不是提交时），失败的任务自动退款，不影响余额。

## 5. 下一步

- 了解每个分辨率/时长的具体单价：[Billing & Credits](./billing.md)
- 想跑批量？先看：[Rate Limits](./rate-limits.md)
- 完整 API 参考：[Video Generation API](./api-video-generation.md)
- 申请 Beta，解锁 1080p 和全量模型：[Beta Program](./beta-program.md)

## 常见问题

**Q：能用 OpenAI SDK 直接调用吗？**

A：可以。AIKanHub 兼容 OpenAI 风格的鉴权头，只需把 `base_url` 指向你的 AIKanHub 地址，把 `api_key` 换成 `sk-xxx`。视频生成的端点路径与 OpenAI 不完全一致 —— 见 [API 参考](./api-video-generation.md)。

**Q：第一次提交就报 "no available channel"。**

A：你请求的模型不在当前套餐允许列表里。Trial 只支持 Lite 系列；想用 Pro 或 1080p 请升级到 Beta 或 Paid。

**Q：返回 429 怎么办？**

A：触发了套餐速率上限，等 30 秒重试即可。详见 [Rate Limits](./rate-limits.md)。
