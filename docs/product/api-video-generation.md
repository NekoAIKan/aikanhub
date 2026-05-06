# 视频生成 API

视频生成 API 用于创建、查询和管理异步视频任务。它适合广告素材、产品展示、创意草图和批量内容生产等场景。

## 调用流程

1. 创建任务：提交提示词、模型 ID 和生成参数。
2. 查询任务：轮询任务状态，直到成功、失败或取消。
3. 获取结果：读取返回的视频地址或文件信息。
4. 记录账单：按任务结果和站点策略扣减 credits。

## 请求示例

```bash
curl /v1/video/generations \
  -H "Authorization: Bearer $AIKANHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2-0-fast-260128",
    "prompt": "一杯咖啡放在木桌上，清晨阳光从窗边照进来",
    "size": "1280x720"
  }'
```

## 参数表

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 视频模型 ID，例如 `doubao-seedance-2-0-fast-260128` |
| `prompt` | string | 生成提示词 |
| `size` | string | 输出尺寸，按站点支持范围填写 |
| `duration` | string | 输出时长，按站点支持范围填写 |

## 状态处理

异步任务可能经历排队、运行、成功、失败或取消等状态。客户端应当用固定间隔查询任务，失败时展示错误原因，并允许用户重新提交或修改提示词。

## 相关页面

- credits 扣减：[计费说明](billing.md)
- 限流策略：[速率限制](rate-limits.md)
- 快速接入：[快速开始](getting-started.md)
