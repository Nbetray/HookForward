import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { useAuth } from "../app/auth";

export function GitHubAuthCallbackPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { signInWithToken } = useAuth();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const accessToken = searchParams.get("access_token");
    if (!accessToken) {
      setError("缺少登录令牌");
      return;
    }

    signInWithToken(accessToken)
      .then(() => {
        navigate("/clients", { replace: true });
      })
      .catch((err: Error) => {
        setError(err.message);
      });
  }, [navigate, searchParams, signInWithToken]);

  return (
    <section className="page narrow">
      <header className="page-header">
        <p className="eyebrow">认证</p>
        <h2>GitHub 登录</h2>
      </header>

      <div className="card">
        {error ? (
          <>
            <p className="error-text">{error}</p>
            <p className="muted">
              <Link to="/login" className="detail-link">返回登录页</Link>
            </p>
          </>
        ) : (
          <p className="muted">正在完成 GitHub 登录...</p>
        )}
      </div>
    </section>
  );
}
