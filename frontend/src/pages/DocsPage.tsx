import { CodeBlock } from "../components/CodeBlock";

export function DocsPage() {
  return (
    <section className="page docs-page">
      <header className="page-header">
        <p className="eyebrow">文档</p>
        <h2>消息投递指南</h2>
      </header>

      <div className="card doc-content">
        <h3>整体架构</h3>
        <CodeBlock>{`上游系统 (GitHub / GitLab / 自定义)
        │
        │  POST /webhook/incoming/{token}
        ▼
┌───────────────────┐
│   HookForward     │
│   Server          │
│                   │
│  1. 接收 Webhook  │
│  2. 签名验证      │
│  3. 消息落库      │
│  4. WebSocket 推送│
└────────┬──────────┘
         │  WebSocket (wss://)
         ▼
┌───────────────────┐
│   Relay Client    │
│                   │
│  1. 接收消息      │
│  2. 本地处理      │
│  3. 返回 ACK      │
└───────────────────┘`}</CodeBlock>

        <h3>消息投递流程</h3>

        <h4>第一步：上游发送 Webhook</h4>
        <p>上游系统向 HookForward 服务器发送 HTTP 请求：</p>
        <CodeBlock>{`curl -X POST https://your-server.com/webhook/incoming/abc123token \\
  -H "Content-Type: application/json" \\
  -H "X-GitHub-Event: push" \\
  -H "X-Hub-Signature-256: sha256=xxxxxx" \\
  -d '{
    "ref": "refs/heads/main",
    "commits": [{"message": "fix: update readme"}]
  }'`}</CodeBlock>

        <h4>第二步：服务端处理</h4>
        <ol>
          <li>根据 URL 中的 <code>token</code> 找到对应的 Client</li>
          <li>如果开启了签名验证，校验 HMAC 签名（支持 <code>hmac-sha256</code>、<code>hmac-sha1</code>、<code>plain</code>）</li>
          <li>将消息写入数据库，状态标记为 <code>received</code></li>
          <li>通过 WebSocket 推送给在线的 Relay Client</li>
        </ol>

        <h4>第三步：WebSocket 推送 + ACK 确认</h4>
        <p>服务端向客户端发送消息后，等待客户端在 <strong>12 秒</strong>内返回 ACK：</p>
        <ul>
          <li>ACK <code>success: true</code> → 状态更新为 <code>delivered</code></li>
          <li>ACK <code>success: false</code> → 状态更新为 <code>delivery_failed</code>，记录错误原因</li>
          <li>超时无响应 → 状态更新为 <code>delivery_failed</code>，错误为 <code>ack timeout</code></li>
          <li>客户端离线 → 状态更新为 <code>delivery_failed</code>，错误为 <code>client offline</code></li>
        </ul>

        <h4>第四步：断线重连恢复</h4>
        <p>客户端重新连接 WebSocket 后，服务端自动查找该 Client 下所有未成功投递的消息（<code>received</code>、<code>delivering</code>、<code>delivery_failed</code>），重新推送，最多恢复 100 条。</p>

        <hr />

        <h3>WebSocket 协议说明</h3>

        <h4>连接地址</h4>
        <CodeBlock>{`ws://localhost:8080/ws/connect    # 开发环境
wss://your-server.com/ws/connect  # 生产环境`}</CodeBlock>

        <h4>认证握手</h4>
        <p>连接建立后，客户端必须在 <strong>15 秒</strong>内发送认证消息：</p>
        <CodeBlock>{`→ {"type": "auth", "client_id": "client_xxx", "client_secret": "secret_xxx"}
← {"type": "auth_ok", "client_id": "client_xxx"}`}</CodeBlock>
        <p>认证失败会返回：</p>
        <CodeBlock>{`← {"type": "auth_error", "error": "invalid credentials"}`}</CodeBlock>

        <h4>心跳保活</h4>
        <ul>
          <li>服务端每 <strong>30 秒</strong>发送一次 Ping</li>
          <li>客户端收到 Ping 后自动回复 Pong</li>
          <li>如果 <strong>90 秒</strong>内没有收到任何消息，连接判定超时断开</li>
        </ul>

        <h4>消息格式</h4>
        <p><strong>服务端 → 客户端（Webhook 消息推送）：</strong></p>
        <CodeBlock>{`{
  "type": "webhook_message",
  "message_id": "msg_a1b2c3d4",
  "event": "push",
  "method": "POST",
  "path": "/webhook/incoming/abc123token",
  "query": "",
  "source": "github",
  "headers": {
    "Content-Type": "application/json",
    "X-GitHub-Event": "push",
    "X-Hub-Signature-256": "sha256=..."
  },
  "payload": {
    "ref": "refs/heads/main",
    "commits": [{"message": "fix: update readme"}]
  },
  "received_at": "2026-04-23T10:30:00Z"
}`}</CodeBlock>

        <p><strong>客户端 → 服务端（ACK 确认）：</strong></p>
        <CodeBlock>{`{"type": "ack", "message_id": "msg_a1b2c3d4", "success": true, "error": ""}`}</CodeBlock>
        <p>处理失败时：</p>
        <CodeBlock>{`{"type": "ack", "message_id": "msg_a1b2c3d4", "success": false, "error": "forward failed: connection refused"}`}</CodeBlock>

        <hr />

        <h3>客户端接入 Demo</h3>

        <h4>Go 客户端（使用内置 SDK）</h4>
        <CodeBlock>{`package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/user/hookforward/backend/pkg/realtimeclient"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
        <-sig
        cancel()
    }()

    client := realtimeclient.New(realtimeclient.Options{
        WSEndpoint:   "wss://your-server.com/ws/connect",
        ClientID:     "client_xxx",
        ClientSecret: "secret_xxx",
        Logger:       log.Default(),
        OnMessage: func(ctx context.Context, msg realtimeclient.Message) error {
            fmt.Printf("收到 Webhook: event=%s source=%s\\n", msg.Event, msg.Source)
            fmt.Printf("  Payload: %s\\n", string(msg.Payload))
            return nil
        },
    })

    if err := client.Run(ctx); err != nil {
        log.Fatalf("client stopped: %v", err)
    }
}`}</CodeBlock>

        <h4>Python 客户端</h4>
        <CodeBlock>{`import asyncio
import json
import websockets

WS_URL = "wss://your-server.com/ws/connect"
CLIENT_ID = "client_xxx"
CLIENT_SECRET = "secret_xxx"

async def handle_message(payload: dict):
    print(f"收到 Webhook: event={payload['event']} source={payload['source']}")
    return True

async def connect():
    while True:
        try:
            async with websockets.connect(WS_URL) as ws:
                await ws.send(json.dumps({
                    "type": "auth",
                    "client_id": CLIENT_ID,
                    "client_secret": CLIENT_SECRET,
                }))
                resp = json.loads(await ws.recv())
                if resp["type"] != "auth_ok":
                    print(f"认证失败: {resp.get('error', 'unknown')}")
                    return
                print("连接成功，等待消息...")
                async for raw in ws:
                    msg = json.loads(raw)
                    if msg["type"] != "webhook_message":
                        continue
                    try:
                        success = await handle_message(msg)
                        await ws.send(json.dumps({
                            "type": "ack",
                            "message_id": msg["message_id"],
                            "success": success,
                            "error": "",
                        }))
                    except Exception as e:
                        await ws.send(json.dumps({
                            "type": "ack",
                            "message_id": msg["message_id"],
                            "success": False,
                            "error": str(e),
                        }))
        except (websockets.ConnectionClosed, OSError) as e:
            print(f"连接断开: {e}，3 秒后重连...")
            await asyncio.sleep(3)

if __name__ == "__main__":
    asyncio.run(connect())`}</CodeBlock>

        <h4>Node.js 客户端</h4>
        <CodeBlock>{`const WebSocket = require("ws");

const WS_URL = "wss://your-server.com/ws/connect";
const CLIENT_ID = "client_xxx";
const CLIENT_SECRET = "secret_xxx";

function connect() {
  const ws = new WebSocket(WS_URL);

  ws.on("open", () => {
    ws.send(JSON.stringify({
      type: "auth",
      client_id: CLIENT_ID,
      client_secret: CLIENT_SECRET,
    }));
  });

  ws.on("message", async (raw) => {
    const msg = JSON.parse(raw);
    if (msg.type === "auth_ok") {
      console.log("连接成功，等待消息...");
      return;
    }
    if (msg.type === "auth_error") {
      console.error("认证失败:", msg.error);
      ws.close();
      return;
    }
    if (msg.type === "webhook_message") {
      console.log(\`收到 Webhook: event=\${msg.event} source=\${msg.source}\`);
      try {
        await handleMessage(msg);
        ws.send(JSON.stringify({
          type: "ack",
          message_id: msg.message_id,
          success: true,
          error: "",
        }));
      } catch (err) {
        ws.send(JSON.stringify({
          type: "ack",
          message_id: msg.message_id,
          success: false,
          error: err.message,
        }));
      }
    }
  });

  ws.on("close", () => {
    console.log("连接断开，3 秒后重连...");
    setTimeout(connect, 3000);
  });

  ws.on("error", (err) => {
    console.error("WebSocket 错误:", err.message);
  });
}

async function handleMessage(msg) {
  console.log("  Payload:", JSON.stringify(msg.payload).slice(0, 200));
}

connect();`}</CodeBlock>

        <h4>cURL 测试（使用 websocat）</h4>
        <CodeBlock>{`# 安装 websocat: brew install websocat

# 连接并手动交互
websocat ws://localhost:8080/ws/connect

# 输入认证（连接后立即发送）：
{"type":"auth","client_id":"client_xxx","client_secret":"secret_xxx"}

# 收到消息后回复 ACK：
{"type":"ack","message_id":"msg_a1b2c3d4","success":true,"error":""}`}</CodeBlock>

        <hr />

        <h3>消息状态说明</h3>
        <table className="table">
          <thead>
            <tr>
              <th>状态</th>
              <th>含义</th>
            </tr>
          </thead>
          <tbody>
            <tr><td><code>received</code></td><td>服务端已收到，尚未推送</td></tr>
            <tr><td><code>delivering</code></td><td>正在推送，等待 ACK</td></tr>
            <tr><td><code>delivered</code></td><td>投递成功，客户端已确认</td></tr>
            <tr><td><code>delivery_failed</code></td><td>投递失败（超时 / 离线 / 客户端报错）</td></tr>
            <tr><td><code>validation_failed</code></td><td>签名验证不通过，不会投递</td></tr>
          </tbody>
        </table>

        <h3>手动重新投递</h3>
        <p>对于投递失败的消息，可以通过 API 手动触发重新投递：</p>
        <CodeBlock>{`curl -X POST https://your-server.com/api/v1/messages/msg_a1b2c3d4/redeliver \\
  -H "Authorization: Bearer {jwt_token}"`}</CodeBlock>
        <p>前提是对应的 Relay Client 当前在线。</p>
      </div>
    </section>
  );
}
