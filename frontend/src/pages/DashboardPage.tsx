import { useEffect, useState } from "react";
import { useAuth } from "../app/auth";
import { getDashboardStats, type DashboardStats } from "../app/api";

const STATUS_LABELS: Record<string, string> = {
  delivered: "已投递",
  delivery_failed: "投递失败",
  received: "已接收",
  delivering: "投递中",
  queued: "排队中",
  validated: "已校验",
  validation_failed: "校验失败",
};

const STATUS_COLORS: Record<string, string> = {
  delivered: "#0f766e",
  delivery_failed: "#b42318",
  received: "#2563eb",
  delivering: "#d97706",
  queued: "#7c3aed",
  validated: "#059669",
  validation_failed: "#dc2626",
};

export function DashboardPage() {
  const { user, loading, token } = useAuth();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    getDashboardStats(token)
      .then((s) => {
        s.daily = s.daily || [];
        s.byStatus = s.byStatus || [];
        setStats(s);
      })
      .catch((e) => setError(e.message));
  }, [token]);

  const maxDaily = stats ? Math.max(...stats.daily.map((d) => d.count), 1) : 1;

  return (
    <section className="page">
      <header className="page-header">
        <p className="eyebrow">控制台</p>
        <h2>概览</h2>
      </header>

      <div className="hero-card">
        <div>
          <p className="hero-label">当前会话</p>
          <h3>{loading ? "正在加载..." : user ? user.email : "未登录"}</h3>
        </div>
        <p className="muted">
          {user
            ? `当前角色：${user.role}，认证来源：${user.authSource}`
            : "先登录，再创建消息通道并获取 webhook 与 client 配置。"}
        </p>
      </div>

      {error && <p className="error-text">{error}</p>}

      {stats && (
        <>
          <div className="grid four">
            <div className="stat-card">
              <p className="stat-label">消息总数</p>
              <p className="stat-value">{stats.totalMessages}</p>
            </div>
            <div className="stat-card delivered">
              <p className="stat-label">已投递</p>
              <p className="stat-value">{stats.delivered}</p>
            </div>
            <div className="stat-card failed">
              <p className="stat-label">投递失败</p>
              <p className="stat-value">{stats.failed}</p>
            </div>
            <div className="stat-card">
              <p className="stat-label">通道</p>
              <p className="stat-value">
                {stats.onlineClients}
                <span className="stat-sub"> / {stats.totalClients}</span>
              </p>
              <p className="stat-hint">在线 / 总数</p>
            </div>
          </div>

          <div className="grid two">
            <div className="card">
              <h3 className="chart-title">近 7 天消息量</h3>
              <div className="bar-chart">
                {stats.daily.map((d) => (
                  <div key={d.date} className="bar-col">
                    <span className="bar-count">{d.count}</span>
                    <div
                      className="bar"
                      style={{ height: `${(d.count / maxDaily) * 100}%` }}
                    />
                    <span className="bar-label">{d.date.slice(5)}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="card">
              <h3 className="chart-title">状态分布</h3>
              {stats.byStatus.length === 0 ? (
                <p className="muted">暂无消息</p>
              ) : (
                <div className="status-bars">
                  {stats.byStatus.map((s) => (
                    <div key={s.status} className="status-row-chart">
                      <span className="status-label">
                        {STATUS_LABELS[s.status] || s.status}
                      </span>
                      <div className="status-track">
                        <div
                          className="status-fill"
                          style={{
                            width: `${(s.count / stats.totalMessages) * 100}%`,
                            background:
                              STATUS_COLORS[s.status] || "#94a3b8",
                          }}
                        />
                      </div>
                      <span className="status-count">{s.count}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </>
      )}
    </section>
  );
}
