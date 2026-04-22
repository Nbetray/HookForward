import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { useAuth } from "../app/auth";
import { getGitHubAuthStartURL } from "../app/api";

export function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { signIn } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const oauthError = searchParams.get("oauth_error");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);

    try {
      await signIn(email, password);
      navigate("/clients");
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="page narrow">
      <header className="page-header">
        <p className="eyebrow">认证</p>
        <h2>登录</h2>
      </header>

      <form className="card form" onSubmit={handleSubmit}>
        <label>
          <span>邮箱</span>
          <input
            type="email"
            placeholder="you@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </label>
        <label>
          <span>密码</span>
          <input
            type="password"
            placeholder="请输入密码"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        {error ? <p className="error-text">{error}</p> : null}
        {oauthError ? <p className="error-text">GitHub 登录失败: {oauthError}</p> : null}
        <button type="submit" disabled={submitting}>
          {submitting ? "登录中..." : "登录"}
        </button>
        <button
          type="button"
          className="secondary"
          disabled={submitting}
          onClick={() => {
            window.location.href = getGitHubAuthStartURL();
          }}
        >
          使用 GitHub 登录
        </button>
        <p className="muted form-footnote">
          <Link to="/reset-password" className="detail-link">忘记密码</Link>
          {" · "}
          <Link to="/register" className="detail-link">注册新账号</Link>
        </p>
      </form>
    </section>
  );
}
