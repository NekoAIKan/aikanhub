# 快速开始

本页帮助新用户完成从注册到第一次 API 调用的流程。更多产品背景见 [产品文档首页](README.md)，费用规则见 [价格与套餐](pricing.md) 与 [计费说明](billing.md)。

## 创建账号

打开站点后，选择注册入口并填写账号信息。若站点开启活动邀请码门槛，注册时需要输入有效的活动邀请码；个人邀请关系不会替代活动邀请码。

注册完成后，系统会根据站点策略分配默认分组、初始 credits 和可选的 starter token。starter token 可能带有到期时间或模型范围限制，具体取决于管理员配置。

## 创建或查看令牌

进入控制台的令牌页面。你可以使用系统生成的 starter token，也可以手动创建新令牌。建议为不同应用分别创建令牌，并在名称中标记用途，方便后续停用或排查。

令牌只会完整展示一次。请把它存放在服务端环境变量或安全配置系统中，不要放进前端代码、公开仓库或客户端安装包。

## 发送第一次请求

把令牌作为 Bearer Token 放入请求头。示例仅展示通用结构，实际模型 ID 请以控制台可用模型列表为准。

```bash
curl /v1/chat/completions \
  -H "Authorization: Bearer $KITTYVIBE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "你好，请用一句话介绍这个平台。"}
    ]
  }'
```

## 下一步

- 了解 credits 如何扣减：[计费说明](billing.md)
- 规划业务预算：[价格与套餐](pricing.md)
- 处理限流与重试：[速率限制](rate-limits.md)
- 调用视频任务接口：[视频生成 API](api-video-generation.md)
