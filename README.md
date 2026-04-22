# HookForward

公网 Webhook 接收 + WebSocket 实时转发平台，帮助没有公网入口的用户接收和转发 Webhook 消息。

## 功能特性

- **多租户隔离** — 每个用户独立管理自己的消息通道和消息
- **通用 Webhook 接入** — 支持任意能发 HTTP Webhook 的上游系统（GitHub、GitLab、Stripe 等）
- **WebSocket 实时推送** — 消息通过 WebSocket 主动推送到用户客户端，支持 ACK 确认
- **自动重连恢复** — 服务重启或网络闪断后，客户端自动恢复连接，用户无感知
- **签名校验** — 支持 GitHub HMAC 签名校验，可扩展其他签名策略
- **消息管理** — 查看投递状态、失败原因，支持手动重发
- **邮箱注册登录** — 邮箱验证码 + 密码，预留 GitHub OAuth 扩展
- **管理员后台** — 用户管理、通道管理、全局消息查看

## 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | React 18, React Router, TypeScript, Vite |
| 后端 | Go, pgx, Redis, WebSocket (gorilla) |
| 数据库 | PostgreSQL 16 |
| 缓存/队列 | Redis 7 |
| 部署 | Docker Compose, Nginx |

## 项目结构

```
hookforward/
├── frontend/          # React Web 应用
├── backend/           # Go 服务
│   ├── cmd/server/          # 主服务入口
│   ├── cmd/relay-client/    # WebSocket 调试客户端
│   ├── internal/            # 业务逻辑
│   ├── migrations/          # 数据库迁移
│   └── pkg/                 # 公共包
├── deploy/            # 部署配置
│   ├── docker-compose.yml
│   └── nginx/
├── docs/              # 补充文档
└── SOLUTION.md        # 方案设计
```

## 快速开始

### 环境要求

- Go 1.25+
- Node.js 18+
- PostgreSQL 16+
- Redis 7+
- Docker & Docker Compose（可选）

### 本地开发

1. 复制环境变量并按需修改：

```bash
cp .env.example .env
```

2. 使用 Docker Compose 启动依赖服务：

```bash
cd deploy && docker-compose up -d postgres redis
```

3. 启动后端：

```bash
cd backend && go run ./cmd/server
```

4. 启动前端：

```bash
cd frontend && npm install && npm run dev
```

### Docker 部署

```bash
cd deploy && docker-compose up -d
```

### 生产部署

```bash
cd deploy && docker-compose -f docker-compose.production.yml up -d
```

## 配置说明

参考 [.env.example](.env.example) 了解所有可配置项，包括：

- 数据库和 Redis 连接
- JWT 密钥
- SMTP 邮件服务
- GitHub OAuth 凭据
- 管理员账号

## 核心链路

```
上游系统 —→ POST /webhook/incoming/:token —→ 消息落库
                                              ↓
                                         Redis 投递队列
                                              ↓
                                    WebSocket 推送到客户端
                                              ↓
                                         客户端 ACK
                                              ↓
                                        更新投递状态
```

## 许可证

[MIT](LICENSE)
