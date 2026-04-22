import { Link, useLocation } from "react-router-dom";
import { useState, type PropsWithChildren } from "react";

import { useAuth } from "../app/auth";

export function Shell({ children }: PropsWithChildren) {
  const location = useLocation();
  const { user, signOut } = useAuth();
  const [menuOpen, setMenuOpen] = useState(false);
  const navItems = [
    { to: "/", label: "概览", visible: true },
    { to: "/clients", label: "消息通道", visible: true },
    { to: "/messages", label: "消息", visible: true },
    { to: "/login", label: "登录", visible: !user },
    { to: "/register", label: "注册", visible: !user },
    { to: "/reset-password", label: "找回密码", visible: !user },
    { to: "/docs", label: "文档", visible: true },
    { to: "/admin/users", label: "管理员", visible: user?.role === "admin" },
  ];

  return (
    <div className="layout">
      <header className="mobile-header">
        <span className="mobile-brand">HookForward</span>
        <button type="button" className="hamburger" onClick={() => setMenuOpen(!menuOpen)} aria-label="菜单">
          <span className={`hamburger-icon ${menuOpen ? "open" : ""}`} />
        </button>
      </header>
      <aside className={`sidebar ${menuOpen ? "sidebar-open" : ""}`}>
        <div className="brand">
          <span className="brand-kicker">Webhook Relay</span>
          <h1>HookForward</h1>
        </div>
        <nav className="nav">
          {navItems.filter((item) => item.visible).map((item) => (
            <Link
              key={item.to}
              to={item.to}
              className={location.pathname === item.to ? "nav-link active" : "nav-link"}
              onClick={() => setMenuOpen(false)}
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="sidebar-footer">
          {user ? (
            <>
              <p className="sidebar-user">{user.email}</p>
              <button type="button" className="secondary" onClick={signOut}>
                退出登录
              </button>
            </>
          ) : (
            <p className="sidebar-user muted">未登录</p>
          )}
        </div>
      </aside>
      {menuOpen && <div className="sidebar-overlay" onClick={() => setMenuOpen(false)} />}
      <main className="content">{children}</main>
    </div>
  );
}
