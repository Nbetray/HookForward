import { useEffect, useState } from "react";

import { formatTime, getAdminUsers, updateAdminUserStatus, type User } from "../app/api";
import { useAuth } from "../app/auth";

export function AdminUsersPage() {
  const { token, user, loading } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [updatingUserID, setUpdatingUserID] = useState<string | null>(null);

  function loadUsers() {
    if (!token) {
      return;
    }

    getAdminUsers(token)
      .then((result) => {
        setUsers(result.items);
        setError(null);
      })
      .catch((err: Error) => {
        setError(err.message);
      });
  }

  useEffect(() => {
    if (!token) {
      setUsers([]);
      return;
    }
    loadUsers();
  }, [token]);

  async function handleToggleStatus(target: User) {
    if (!token) {
      return;
    }

    const nextStatus = target.status === "active" ? "disabled" : "active";
    setUpdatingUserID(target.id);
    setError(null);

    try {
      const result = await updateAdminUserStatus(token, target.id, nextStatus);
      setUsers((current) =>
        current.map((item) => (item.id === result.user.id ? result.user : item)),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新用户状态失败");
    } finally {
      setUpdatingUserID(null);
    }
  }

  if (loading) {
    return (
      <section className="page">
        <p className="muted">正在加载会话...</p>
      </section>
    );
  }

  if (!user || !token || user.role !== "admin") {
    return (
      <section className="page">
        <div className="card">
          <p className="muted">仅管理员可访问。</p>
        </div>
      </section>
    );
  }

  return (
    <section className="page">
      <header className="page-header split">
        <div>
          <p className="eyebrow">管理员</p>
          <h2>用户管理</h2>
        </div>
        <button type="button" className="secondary" onClick={loadUsers}>
          刷新
        </button>
      </header>

      {error ? <p className="error-text">{error}</p> : null}

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>邮箱</th>
              <th>昵称</th>
              <th>角色</th>
              <th>认证来源</th>
              <th>状态</th>
              <th>最近登录</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {users.map((item) => {
              const toggling = updatingUserID === item.id;
              const nextAction = item.status === "active" ? "禁用" : "启用";
              const isSelf = item.id === user.id;

              return (
                <tr key={item.id}>
                  <td data-label="邮箱">{item.email}</td>
                  <td data-label="昵称">{item.displayName || "-"}</td>
                  <td data-label="角色">{item.role}</td>
                  <td data-label="认证来源">{item.authSource}</td>
                  <td data-label="状态">
                    <span className={`pill ${item.status === "active" ? "online" : "offline"}`}>
                      {item.status}
                    </span>
                  </td>
                  <td data-label="最近登录">{item.lastLoginAt ? formatTime(item.lastLoginAt) : "从未登录"}</td>
                  <td data-label="操作">
                    <button
                      type="button"
                      className={item.status === "active" ? "link-button danger-link" : "link-button"}
                      disabled={toggling || isSelf}
                      onClick={() => handleToggleStatus(item)}
                    >
                      {toggling ? "处理中..." : nextAction}
                    </button>
                  </td>
                </tr>
              );
            })}
            {users.length === 0 ? (
              <tr>
                <td colSpan={7} className="muted">
                  暂无用户。
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </section>
  );
}
