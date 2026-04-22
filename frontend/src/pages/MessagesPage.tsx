import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { useAuth } from "../app/auth";
import { formatTime, getClientMessages, getClients, getMessages, redeliverMessage, type Client, type Message } from "../app/api";

export function MessagesPage() {
  const { token, user, loading } = useAuth();
  const [clients, setClients] = useState<Client[]>([]);
  const [selectedClientID, setSelectedClientID] = useState("all");
  const [messages, setMessages] = useState<Message[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [redeliveringID, setRedeliveringID] = useState<string | null>(null);
  const [signatureFilter, setSignatureFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");

  function loadMessages() {
    if (!token) {
      return;
    }

    const request = selectedClientID === "all" ? getMessages(token) : getClientMessages(token, selectedClientID);

    request
      .then((result) => {
        const filtered = result.items.filter((item) => {
          if (statusFilter !== "all" && item.deliveryStatus !== statusFilter) {
            return false;
          }
          if (signatureFilter === "all") {
            return true;
          }
          if (signatureFilter === "passed") {
            return item.signatureValid;
          }
          return !item.signatureValid;
        });
        setMessages(filtered);
        setError(null);
      })
      .catch((err: Error) => {
        setError(err.message);
      });
  }

  useEffect(() => {
    if (!token) {
      setClients([]);
      setMessages([]);
      return;
    }

    getClients(token)
      .then((result) => {
        setClients(result.items);
      })
      .catch((err: Error) => {
        setError(err.message);
      });
  }, [token]);

  useEffect(() => {
    if (!token) {
      return;
    }
    loadMessages();
  }, [token, selectedClientID, signatureFilter, statusFilter]);

  async function handleRedeliver(messageID: string) {
    if (!token) {
      return;
    }
    if (!window.confirm("确认手动重发这条消息吗？")) {
      return;
    }

    setRedeliveringID(messageID);
    setError(null);
    try {
      const updated = await redeliverMessage(token, messageID);
      setMessages((current) =>
        current.map((item) => (item.id === updated.id ? updated : item)),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "重发失败");
    } finally {
      setRedeliveringID(null);
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

  return (
    <section className="page">
      <header className="page-header split">
        <div>
          <p className="eyebrow">消息</p>
          <h2>最近投递</h2>
        </div>
        <button type="button" className="secondary" onClick={loadMessages}>
          刷新
        </button>
      </header>

      {error ? <p className="error-text">{error}</p> : null}

      <div className="card filter-row">
        <label className="grow">
          <span>选择消息通道</span>
          <select value={selectedClientID} onChange={(event) => setSelectedClientID(event.target.value)}>
            <option value="all">全部通道</option>
            {clients.map((client) => (
              <option key={client.id} value={client.id}>
                {client.name} ({client.clientId})
              </option>
            ))}
          </select>
        </label>
        <label className="grow">
          <span>验签结果</span>
          <select value={signatureFilter} onChange={(event) => setSignatureFilter(event.target.value)}>
            <option value="all">全部</option>
            <option value="passed">仅通过</option>
            <option value="failed">仅失败</option>
          </select>
        </label>
        <label className="grow">
          <span>投递状态</span>
          <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
            <option value="all">全部</option>
            <option value="delivered">已投递</option>
            <option value="delivery_failed">投递失败</option>
            <option value="validation_failed">验签失败</option>
          </select>
        </label>
      </div>

      {messages.length === 0 ? (
        <div className="card">
          <p className="muted">
            {selectedClientID === "all" ? "还没有收到任何 webhook 消息。" : "该通道还没有消息。"}
          </p>
        </div>
      ) : (
        <div className="card">
          <table className="table">
            <thead>
            <tr>
              <th>ID</th>
              <th>事件</th>
              <th>验签</th>
              <th>状态</th>
              <th>重试</th>
              <th>最后错误</th>
              <th>接收时间</th>
              <th>操作</th>
            </tr>
            </thead>
            <tbody>
              {messages.map((message) => (
                <tr key={message.id}>
                  <td className="mono cell-truncate" title={message.id} data-label="ID">{message.id}</td>
                  <td data-label="事件">{message.eventType}</td>
                  <td data-label="验签">
                    <span className={`pill ${message.signatureValid ? "online" : "offline"}`}>
                      {message.signatureValid ? "passed" : "failed"}
                    </span>
                  </td>
                  <td data-label="状态">{message.deliveryStatus}</td>
                  <td data-label="重试">{message.deliveryAttempts}</td>
                  <td className="message-error-cell" data-label="最后错误">{message.lastError || "-"}</td>
                  <td data-label="接收时间">{formatTime(message.receivedAt)}</td>
                  <td data-label="操作">
                    <div className="table-actions">
                      <Link className="detail-link" to={`/messages/${message.id}`}>
                        查看详情
                      </Link>
                      <button
                        type="button"
                        className="link-button"
                        disabled={redeliveringID === message.id}
                        onClick={() => handleRedeliver(message.id)}
                      >
                        {redeliveringID === message.id ? "重发中..." : "手动重发"}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
