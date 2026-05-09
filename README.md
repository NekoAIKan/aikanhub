<div align="center">

# KittyVibe

**给开发者的视频生成 API 平台，一个 key 调用所有主流模型。**

[English](./README.en.md) · [License](./LICENSE) · [Acknowledgments](./NOTICE.md)

</div>

---

## 这是什么

KittyVibe 是一个聚合视频生成模型 API 的网关。开发者用统一的 key 和接口，就能调用 Seedance、Pixverse 等主流视频生成模型，不用一家家上游对接、管 key、对账单。

## 当前支持

| 模型 | 状态 |
|---|---|
| 字节跳动 Seedance 2.0 / 2.0 fast | ✅ 已支持（文生视频、图生视频、首尾帧、多模态参考、有声视频） |
| Pixverse v5.5 | 🚧 占位中，规划中 |
| 更多视频模型 | 按需求加 |

## 快速开始（本地）

需要 Docker。`docker-compose.local.yml` 会启动应用和 Redis；数据库使用 `.env.local` 里的 Neon 本地开发分支。

```bash
# 1. 克隆
git clone git@github.com:NekoAIKan/aikanhub.git
cd aikanhub

# 2. 准备环境变量
cp .env.local.example .env.local
# 把 SQL_DSN 填成 Neon 本地开发分支连接串；不要填生产 Neon/NDB 连接串

# 3. 启动（首次会 build 镜像，约 5-10 分钟）
docker compose -f docker-compose.local.yml --env-file .env.local up -d

# 4. 访问
open http://localhost:3000
```

## 调用 API

部署完成后，登录后台 → **令牌** 创建一个 `sk-xxx`，然后：

```bash
export KITTYVIBE_TOKEN=sk-你的token
export KITTYVIBE_HOST=http://localhost:3000   # 部署地址

# 1) 提交视频生成任务（异步）
curl -X POST $KITTYVIBE_HOST/v1/video/generations \
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2-0-fast-260128",
    "prompt": "一只橘猫慢慢走过夕阳下的东京街道，4K，电影感",
    "size": "720p",
    "duration": 5,
    "metadata": { "ratio": "16:9" }
  }'
# → {"task_id": "task_xxx", "status": "queued"}

# 2) 轮询状态（5s 一次，通常 90-120s 完成）
curl $KITTYVIBE_HOST/v1/video/generations/task_xxx \
  -H "Authorization: Bearer $KITTYVIBE_TOKEN"
# → {"data": {"status": "SUCCESS", "data": {"content": {"video_url": "https://..."}}}}
```

完整 Python / Node.js 示例：见 [tools/test-seedance.sh](./tools/test-seedance.sh)。

**支持的模型**：

| Model ID | 价格（720p / 5s） | 状态 |
|---|---|---|
| `doubao-seedance-2-0-260128` | $0.885 / video | ✅ |
| `doubao-seedance-2-0-fast-260128` | $0.712 / video | ✅ |
| `pixverse-v5.5` | — | 🚧 规划中 |
| `happyhorse` | — | 🚧 规划中 |

## 部署

完整本地启动步骤（含首次启动、定价配置、常见问题、生产化清单）见 [DEPLOY.md](./DEPLOY.md)。

## 协议

[AGPL-3.0](./LICENSE)。如果你用 KittyVibe 对外提供网络服务，必须向用户开放完整源码（包括你的修改）。如需闭源商用，请联系上游 [QuantumNous](mailto:support@quantumnous.com)。

## 致谢

KittyVibe 基于 [Calcium-Ion/new-api](https://github.com/Calcium-Ion/new-api) fork 而来，详见 [NOTICE.md](./NOTICE.md)。
