# Video Generation API 视频生成 API

AIKanHub 的视频生成接口采用**异步任务**模式：先提交任务拿到 `task_id`，再轮询直到状态终结。

## 鉴权

所有接口都用标准的 OpenAI 风格 Bearer Token：

```http
Authorization: Bearer sk-YOUR_TOKEN
Content-Type: application/json
```

`sk-xxx` 在控制台 **个人中心 → 令牌** 创建。

## 端点速览

| 操作 | 方法 | 路径 |
| --- | --- | --- |
| 提交视频生成任务 | `POST` | `/v1/video/generations` |
| 查询任务状态 | `GET`  | `/v1/tasks/{task_id}` |

## 提交任务

### 请求体

```json
{
  "model": "doubao-seedance-1-0-lite-t2v",
  "prompt": "A red panda surfing on a wave at sunset, cinematic",
  "resolution": "720p",
  "duration": 5,
  "ratio": "16:9",
  "image": "https://example.com/seed.png"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | ✅ | 见下文 [模型列表](#模型列表)。模型决定能用的分辨率和是否支持图生视频。 |
| `prompt` | string | ✅ | 文本描述。建议英文，可附风格关键词。 |
| `resolution` | string | 否 | `480p` / `720p` / `1080p`，或显式写 `1280x720` / `1920x1080`。默认依模型而定。 |
| `duration` | int | 否 | 视频时长（秒）。常用 5 / 10。上限因模型而异。 |
| `ratio` | string | 否 | 画幅比例，例 `16:9` / `9:16` / `1:1`。 |
| `image` | string | 仅图生视频 | 图生视频（i2v）模型必填，文生视频（t2v）模型忽略。 |

### 成功响应（HTTP 200）

```json
{ "id": "task_abc123" }
```

把 `id` 拿去查询。

### 错误响应

| HTTP | 含义 | 怎么办 |
| --- | --- | --- |
| 401 | 鉴权失败 | 检查 `Authorization` 头和 Token 是否启用 |
| 402 | 钱包余额不足 | 充值或换更便宜的模型 / 分辨率 |
| 403 | 模型不在你套餐允许列表 | 升级套餐（[Pricing](./pricing.md)），或换 Lite 系列 |
| 429 | 触发速率限制 | 等几秒重试，详见 [Rate Limits](./rate-limits.md) |
| 4xx + `no available channel` | 当前套餐没有该模型的访问权 | 升级或换模型 |
| 5xx | 平台暂时不可用 | 稍后重试，已扣费会自动退 |

## 查询任务状态

```http
GET /v1/tasks/{task_id}
Authorization: Bearer sk-YOUR_TOKEN
```

### 响应字段

```json
{
  "id": "task_abc123",
  "status": "succeeded",
  "model": "doubao-seedance-1-0-lite-t2v",
  "resolution": "720p",
  "duration": 5,
  "content": {
    "video_url": "https://..."
  },
  "usage": {
    "completion_tokens": 35200,
    "total_tokens": 35200
  },
  "created_at": "2026-05-05T08:30:00Z",
  "finished_at": "2026-05-05T08:30:42Z"
}
```

| `status` | 含义 |
| --- | --- |
| `pending` | 已提交，排队中 |
| `running` | 正在生成 |
| `succeeded` | 完成，`content.video_url` 可下载 |
| `failed` | 失败，已自动退款。`error` 字段含原因 |

> **`video_url` 时效**：下载链接通常在生成后 2 小时左右过期，请尽快下载。我们不保留长期镜像。

### 推荐轮询节奏

- 提交后等 10 秒再首次查询（多数任务 30–60 秒完成）。
- 之后每 5 秒查一次，直到 `succeeded` / `failed`。
- 不要小于 1 秒间隔轮询，会被速率限制。

## 模型列表

> 模型 ID 是调用 API 时 `model` 字段的取值。下表中"档位"对应钱包定价 —— 详见 [Billing & Credits](./billing.md)。

| 模型 ID | 类型 | 最大分辨率 | 套餐可用 | 档位 |
| --- | --- | --- | --- | --- |
| `doubao-seedance-1-0-lite-t2v` | 文生视频 | 720p | Trial / Beta / Paid | Lite |
| `doubao-seedance-1-0-lite-i2v` | 图生视频 | 720p | Trial / Beta / Paid | Lite |
| `doubao-seedance-1-0-pro-250528` | 文生视频（高质量） | 1080p | Beta / Paid | Pro |
| `doubao-seedance-1-5-pro-251215` | 文生视频（高质量 v1.5） | 1080p | Beta / Paid | Pro |
| `doubao-seedance-2-0-fast-260128` | 文生视频（快速） | 1080p | Beta / Paid | Pro |
| `doubao-seedance-2-0-260128` | 文生视频（最高质量） | 1080p | Beta / Paid | Pro |

## 端到端示例

### Python

```python
import time, requests

BASE = "https://YOUR_AIKANHUB_HOST"
KEY  = "sk-YOUR_TOKEN"
H = {"Authorization": f"Bearer {KEY}", "Content-Type": "application/json"}

# 1. 提交
r = requests.post(f"{BASE}/v1/video/generations", json={
    "model": "doubao-seedance-1-0-lite-t2v",
    "prompt": "A red panda surfing on a wave at sunset, cinematic",
    "resolution": "720p",
    "duration": 5,
    "ratio": "16:9",
}, headers=H)
task_id = r.json()["id"]

# 2. 轮询
while True:
    s = requests.get(f"{BASE}/v1/tasks/{task_id}", headers=H).json()
    if s["status"] in ("succeeded", "failed"):
        break
    time.sleep(5)

if s["status"] == "succeeded":
    print("Video URL:", s["content"]["video_url"])
else:
    print("Failed:", s.get("error"))
```

### Node.js

```javascript
const BASE = "https://YOUR_AIKANHUB_HOST";
const KEY  = "sk-YOUR_TOKEN";
const H = { "Authorization": `Bearer ${KEY}`, "Content-Type": "application/json" };

const submit = await fetch(`${BASE}/v1/video/generations`, {
  method: "POST", headers: H,
  body: JSON.stringify({
    model: "doubao-seedance-2-0-fast-260128",
    prompt: "A timelapse of a flower blooming",
    resolution: "1080p",
    duration: 5,
  })
});
const { id: taskId } = await submit.json();

while (true) {
  const r = await fetch(`${BASE}/v1/tasks/${taskId}`, { headers: H });
  const s = await r.json();
  if (s.status === "succeeded") { console.log(s.content.video_url); break; }
  if (s.status === "failed")    { console.error(s.error); break; }
  await new Promise(r => setTimeout(r, 5000));
}
```

## 退款规则

- 任务**进入排队**就会预扣预估额度。
- 任务**成功完成**时按真实分辨率和时长结算（多退少补）。
- 任务**失败**时全额退款，钱包恢复到提交前。
- 关于多退少补的精确算法见 [Billing & Credits](./billing.md)。
