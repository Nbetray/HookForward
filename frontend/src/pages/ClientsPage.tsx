import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";

import { useAuth } from "../app/auth";
import { createClient, deleteClient, getClients, type Client } from "../app/api";

export function ClientsPage() {
  const { token, user, loading } = useAuth();
  const [clients, setClients] = useState<Client[]>([]);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");

  useEffect(() => {
    if (!token) {
      setClients([]);
      return;
    }

    let cancelled = false;

    getClients(token)
      .then((result) => {
        if (!cancelled) {
          setClients(result.items);
          setFetchError(null);
        }
      })
      .catch((err: Error) => {
        if (!cancelled) {
          setFetchError(err.message);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [token]);

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !name.trim()) {
      return;
    }

    setCreating(true);
    setFetchError(null);

    try {
      const client = await createClient(token, name.trim());
      setClients((current) => [client, ...current]);
      setName("");
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : "创建通道失败");
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(clientID: string) {
    if (!token) {
      return;
    }
    if (!window.confirm("确认删除这个消息通道吗？该通道下的消息也会一起逻辑删除。")) {
      return;
    }

    try {
      await deleteClient(token, clientID);
      setClients((current) => current.filter((item) => item.id !== clientID));
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : "删除通道失败");
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
          <h3>需要先登录</h3>
          <p className="muted">登录后才能查看和创建自己的消息通道。</p>
        </div>
      </section>
    );
  }

  return (
    <section className="page">
      <header className="page-header split">
        <div>
          <p className="eyebrow">消息通道</p>
          <h2>通道列表</h2>
        </div>
        <span className="muted">{user.email}</span>
      </header>

      <form className="card inline-form" onSubmit={handleCreate}>
        <label className="grow">
          <span>新建通道</span>
          <input
            type="text"
            placeholder="例如：生产环境回调"
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <button type="submit" disabled={creating}>
          {creating ? "创建中..." : "创建通道"}
        </button>
      </form>

      {fetchError ? <p className="error-text">{fetchError}</p> : null}

      <div className="grid">
        {clients.map((client) => (
          <article key={client.id} className="card">
            <div className="status-row">
              <h3>{client.name}</h3>
              <span className={`pill ${client.online ? "online" : "offline"}`}>
                {client.online ? "在线" : "离线"}
              </span>
            </div>
            <p className="muted">{client.clientId}</p>
            <p className="mono">{client.webhookUrl}</p>
            <p className="muted">验签开关：{client.verifySignature ? "已开启" : "未开启"}</p>
            {client.clientSecret ? <p className="mono">secret: {client.clientSecret}</p> : null}
            <p>
              <Link to={`/clients/${client.id}`} className="detail-link">
                查看详情
              </Link>
            </p>
            <p>
              <button type="button" className="link-button danger-link" onClick={() => handleDelete(client.id)}>
                删除通道
              </button>
            </p>
          </article>
        ))}
        {clients.length === 0 ? (
          <article className="card">
            <h3>还没有通道</h3>
            <p className="muted">先创建一个通道，系统会自动生成 webhook URL 和 client 配置。</p>
          </article>
        ) : null}
      </div>
    </section>
  );
}
