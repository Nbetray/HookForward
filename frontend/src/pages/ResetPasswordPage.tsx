import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";

import { resetPassword, sendResetCode } from "../app/api";

export function ResetPasswordPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [sendingCode, setSendingCode] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  async function handleSendCode() {
    if (!email.trim()) {
      setError("请先输入邮箱");
      return;
    }

    setSendingCode(true);
    setError(null);
    setSuccess(null);

    try {
      const result = await sendResetCode(email.trim());
      setSuccess(result.debugCode ? `验证码已发送，开发环境验证码：${result.debugCode}` : "验证码已发送，请检查邮箱");
    } catch (err) {
      setError(err instanceof Error ? err.message : "发送验证码失败");
    } finally {
      setSendingCode(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess(null);

    try {
      await resetPassword(email.trim(), code.trim(), password);
      navigate("/login");
    } catch (err) {
      setError(err instanceof Error ? err.message : "重置密码失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="page narrow">
      <header className="page-header">
        <p className="eyebrow">认证</p>
        <h2>找回密码</h2>
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
          <span>验证码</span>
          <div className="inline-row">
            <input
              type="text"
              placeholder="6 位验证码"
              value={code}
              onChange={(event) => setCode(event.target.value)}
            />
            <button type="button" className="secondary" onClick={handleSendCode} disabled={sendingCode}>
              {sendingCode ? "发送中..." : "发送验证码"}
            </button>
          </div>
        </label>
        <label>
          <span>新密码</span>
          <input
            type="password"
            placeholder="输入新密码"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        {error ? <p className="error-text">{error}</p> : null}
        {success ? <p className="success-text">{success}</p> : null}
        <button type="submit" disabled={submitting}>
          {submitting ? "提交中..." : "重置密码"}
        </button>
        <p className="muted form-footnote">
          想起密码了？<Link to="/login" className="detail-link">返回登录</Link>
        </p>
      </form>
    </section>
  );
}
