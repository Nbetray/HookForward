import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";

import { useAuth } from "../app/auth";
import {
  formatTime,
  getClient,
  getClientMessages,
  updateClientSecurity,
  updateClientHeaders,
  type Client,
  type Message,
} from "../app/api";

export function ClientDetailPage() {
  const { clientId } = useParams();
  const { token, user, loading } = useAuth();
  const [client, setClient] = useState<Client | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [updatingSecurity, setUpdatingSecurity] = useState(false);

  const [sigHeader, setSigHeader] = useState("");
  const [sigAlgo, setSigAlgo] = useState("");
  const [evtHeader, setEvtHeader] = useState("");
  const [savingHeaders, setSavingHeaders] = useState(false);
  const [headersSaved, setHeadersSaved] = useState(false);

  useEffect(() => {
    if (!token || !clientId) return;

    let cancelled = false;
    Promise.all([getClient(token, clientId), getClientMessages(token, clientId)])
      .then(([clientResult, messagesResult]) => {
        if (cancelled) return;
        setClient(clientResult);
        setSigHeader(clientResult.signatureHeader || "");
        setSigAlgo(clientResult.signatureAlgorithm || "");
        setEvtHeader(clientResult.eventTypeHeader || "");
        setMessages(messagesResult.items || []);
        setError(null);
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      });

    return () => { cancelled = true; };
  }, [token, clientId]);

  if (loading) {
    return (
      <section className="page">
        <p className="muted">正在加载会话...</p>
      </section>
    );
  }

  async function handleToggleSignature() {
    if (!token || !client) return;
    setUpdatingSecurity(true);
    setError(null);
    try {
      const updated = await updateClientSecurity(token, client.id, !client.verifySignature);
      setClient(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新安全设置失败");
    } finally {
      setUpdatingSecurity(false);
    }
  }

  async function handleSaveHeaders() {
    if (!token || !client) return;
    setSavingHeaders(true);
    setError(null);
    setHeadersSaved(false);
    try {
      const updated = await updateClientHeaders(token, client.id, sigHeader.trim(), sigAlgo, evtHeader.trim());
      setClient(updated);
      setHeadersSaved(true);
      setTimeout(() => setHeadersSaved(false), 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新自定义 Header 失败");
    } finally {
      setSavingHeaders(false);
    }
  }

  if (!user || !token) {
    return (
      <section className="page">
        <div className="card">
          <p className="muted">请先登录。</p>
        </div>
      </section>
    );
  }

  return (
    <section className="page">
      <header className="page-header">
        <p className="eyebrow">消息通道详情</p>
        <h2>{client?.name ?? clientId}</h2>
      </header>

      {error ? <p className="error-text">{error}</p> : null}

      <div className="grid two">
        <article className="card">
          <h3>连接信息</h3>
          <h4 style={{ margin: "1rem 0 0.5rem", fontSize: "0.9rem", color: "#486581" }}>Webhook 配置</h4>
          <dl className="details-list">
            <div>
              <dt>Webhook URL</dt>
              <dd className="mono">{client?.webhookUrl ?? "-"}</dd>
            </div>
            <div>
              <dt>Webhook Secret</dt>
              <dd className="mono">{client?.webhookSecret ?? "默认不返回，请在创建后妥善保存"}</dd>
            </div>
          </dl>
          <h4 style={{ margin: "1.25rem 0 0.5rem", fontSize: "0.9rem", color: "#486581" }}>WebSocket 配置</h4>
          <dl className="details-list">
            <div>
              <dt>WebSocket URL</dt>
              <dd className="mono">{client?.wsEndpoint ?? "-"}</dd>
            </div>
            <div>
              <dt>Client ID</dt>
              <dd className="mono">{client?.clientId ?? "-"}</dd>
            </div>
            <div>
              <dt>Client Secret</dt>
              <dd className="mono">{client?.clientSecret ?? "默认不返回，请在创建后妥善保存"}</dd>
            </div>
          </dl>
        </article>

        <article className="card">
          <h3>签名与事件配置</h3>
          <dl className="details-list">
            <div>
              <dt>签名校验</dt>
              <dd>
                <span style={{ marginRight: "0.75rem" }}>{client?.verifySignature ? "已开启" : "未开启"}</span>
                <button type="button" className="secondary" onClick={handleToggleSignature} disabled={!client || updatingSecurity} style={{ fontSize: "0.85rem", padding: "0.4rem 0.85rem" }}>
                  {updatingSecurity ? "更新中..." : client?.verifySignature ? "关闭" : "开启"}
                </button>
              </dd>
            </div>
            <div>
              <dt>签名 Header</dt>
              <dd>
                <input
                  value={sigHeader}
                  onChange={(e) => setSigHeader(e.target.value)}
                  placeholder="留空则按默认顺序检测"
                  style={{ width: "100%", padding: "0.5rem 0.75rem", borderRadius: "10px", border: "1px solid rgba(16,42,67,0.12)", background: "rgba(255,255,255,0.92)", font: "inherit", fontSize: "0.9rem" }}
                />
                <p className="muted" style={{ margin: "0.25rem 0 0", fontSize: "0.75rem" }}>
                  默认依次检测 X-Hub-Signature-256、X-Webhook-Signature-256
                </p>
              </dd>
            </div>
            <div>
              <dt>签名算法</dt>
              <dd>
                <select
                  value={sigAlgo}
                  onChange={(e) => setSigAlgo(e.target.value)}
                  style={{ width: "100%", padding: "0.5rem 0.75rem", borderRadius: "10px", border: "1px solid rgba(16,42,67,0.12)", background: "rgba(255,255,255,0.92)", font: "inherit", fontSize: "0.9rem" }}
                >
                  <option value="">HMAC-SHA256（默认）</option>
                  <option value="hmac-sha256">HMAC-SHA256</option>
                  <option value="hmac-sha1">HMAC-SHA1</option>
                  <option value="plain">Plain（直接对比）</option>
                </select>
                <p className="muted" style={{ margin: "0.25rem 0 0", fontSize: "0.75rem" }}>
                  plain 模式直接对比 Header 值和 Webhook Secret
                </p>
              </dd>
            </div>
            <div>
              <dt>事件类型 Header</dt>
              <dd>
                <input
                  value={evtHeader}
                  onChange={(e) => setEvtHeader(e.target.value)}
                  placeholder="留空则按默认顺序检测"
                  style={{ width: "100%", padding: "0.5rem 0.75rem", borderRadius: "10px", border: "1px solid rgba(16,42,67,0.12)", background: "rgba(255,255,255,0.92)", font: "inherit", fontSize: "0.9rem" }}
                />
                <p className="muted" style={{ margin: "0.25rem 0 0", fontSize: "0.75rem" }}>
                  默认依次检测 X-GitHub-Event、X-Gitlab-Event、X-Event-Type
                </p>
              </dd>
            </div>
          </dl>
          <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
            <button type="button" onClick={handleSaveHeaders} disabled={!client || savingHeaders}>
              {savingHeaders ? "保存中..." : "保存 Header 配置"}
            </button>
            {headersSaved && <span className="success-text" style={{ margin: 0 }}>已保存</span>}
          </div>
        </article>
      </div>

      <article className="card">
        <h3>最近状态</h3>
        <ul>
          <li>最后连接时间：{client?.lastConnectedAt ? formatTime(client.lastConnectedAt) : "尚未连接"}</li>
          <li>消息总数：{messages.length}</li>
          <li>最近状态：{messages[0]?.deliveryStatus ?? "暂无消息"}</li>
        </ul>
      </article>

      <div className="card">
        <h3>最近消息</h3>
        <table className="table">
          <thead>
            <tr>
              <th>ID</th>
              <th>事件</th>
              <th>方法</th>
              <th>验签</th>
              <th>状态</th>
              <th>接收时间</th>
            </tr>
          </thead>
          <tbody>
            {messages.map((message) => (
              <tr key={message.id}>
                <td className="mono" data-label="ID">{message.id}</td>
                <td data-label="事件">{message.eventType}</td>
                <td data-label="方法">{message.httpMethod}</td>
                <td data-label="验签">
                  <span className={`pill ${message.signatureValid ? "online" : "offline"}`}>
                    {message.signatureValid ? "passed" : "failed"}
                  </span>
                </td>
                <td data-label="状态">{message.deliveryStatus}</td>
                <td data-label="接收时间">{formatTime(message.receivedAt)}</td>
              </tr>
            ))}
            {messages.length === 0 ? (
              <tr>
                <td colSpan={6} className="muted">
                  该通道还没有收到消息。
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </section>
  );
}
