import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { formatTime, getMessage, redeliverMessage, type Message } from "../app/api";
import { useAuth } from "../app/auth";

function formatJson(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

export function MessageDetailPage() {
  const { token, user, loading } = useAuth();
  const { messageId } = useParams();
  const [message, setMessage] = useState<Message | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [redelivering, setRedelivering] = useState(false);

  useEffect(() => {
    if (!token || !messageId) {
      setMessage(null);
      return;
    }

    getMessage(token, messageId)
      .then((result) => {
        setMessage(result);
        setError(null);
      })
      .catch((err: Error) => {
        setError(err.message);
      });
  }, [token, messageId]);

  async function handleRedeliver() {
    if (!token || !message) {
      return;
    }
    if (!window.confirm("确认手动重发这条消息吗？")) {
      return;
    }

    setRedelivering(true);
    setError(null);
    try {
      const updated = await redeliverMessage(token, message.id);
      setMessage(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "重发失败");
    } finally {
      setRedelivering(false);
    }
  }

  if (loading) {
    return (
      <section className="page">
        <p className="muted">正在加载会话...</p>
      </section>
    );
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

  if (!messageId) {
    return (
      <section className="page">
        <div className="card">
          <p className="muted">缺少消息 ID。</p>
        </div>
      </section>
    );
  }

  return (
    <section className="page">
      <header className="page-header split">
        <div>
          <p className="eyebrow">消息详情</p>
          <h2>投递审计</h2>
        </div>
        <div className="header-actions">
          <Link className="detail-link" to="/messages">
            返回消息列表
          </Link>
          <button type="button" className="secondary" disabled={redelivering || !message} onClick={handleRedeliver}>
            {redelivering ? "重发中..." : "手动重发"}
          </button>
        </div>
      </header>

      {error ? <p className="error-text">{error}</p> : null}

      {!message ? (
        <div className="card">
          <p className="muted">正在加载消息详情...</p>
        </div>
      ) : (
        <>
          <div className="grid two">
            <div className="card">
              <dl className="details-list">
                <div>
                  <dt>消息 ID</dt>
                  <dd className="mono">{message.id}</dd>
                </div>
                <div>
                  <dt>投递状态</dt>
                  <dd>{message.deliveryStatus}</dd>
                </div>
                <div>
                  <dt>签名校验</dt>
                  <dd>
                    <span className={`pill ${message.signatureValid ? "online" : "offline"}`}>
                      {message.signatureValid ? "通过" : "未通过"}
                    </span>
                  </dd>
                </div>
                <div>
                  <dt>投递次数</dt>
                  <dd>{message.deliveryAttempts}</dd>
                </div>
                <div>
                  <dt>最后错误</dt>
                  <dd>{message.lastError || "-"}</dd>
                </div>
                <div>
                  <dt>接收时间</dt>
                  <dd>{formatTime(message.receivedAt)}</dd>
                </div>
                <div>
                  <dt>投递时间</dt>
                  <dd>{formatTime(message.deliveredAt)}</dd>
                </div>
              </dl>
            </div>
            <div className="card">
              <dl className="details-list">
                <div>
                  <dt>事件类型</dt>
                  <dd>{message.eventType}</dd>
                </div>
                <div>
                  <dt>来源</dt>
                  <dd>{message.sourceLabel}</dd>
                </div>
                <div>
                  <dt>客户端</dt>
                  <dd className="mono">{message.clientId}</dd>
                </div>
                <div>
                  <dt>HTTP Method</dt>
                  <dd>{message.httpMethod}</dd>
                </div>
                <div>
                  <dt>Request Path</dt>
                  <dd className="mono">{message.requestPath}</dd>
                </div>
                <div>
                  <dt>Query String</dt>
                  <dd className="mono">{message.queryString || "-"}</dd>
                </div>
              </dl>
            </div>
          </div>

          <div className="card">
            <div className="payload-header">
              <h3>原始 headers</h3>
            </div>
            <pre className="json-block">{formatJson(message.headersJson)}</pre>
          </div>

          <div className="card">
            <div className="payload-header">
              <h3>原始 payload</h3>
            </div>
            <pre className="json-block">{formatJson(message.payloadJson)}</pre>
          </div>
        </>
      )}
    </section>
  );
}
