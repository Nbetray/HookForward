import { Navigate, Route, Routes } from "react-router-dom";

import { useAuth } from "./auth";
import { Shell } from "../components/Shell";
import { DashboardPage } from "../pages/DashboardPage";
import { LoginPage } from "../pages/LoginPage";
import { RegisterPage } from "../pages/RegisterPage";
import { ResetPasswordPage } from "../pages/ResetPasswordPage";
import { ClientsPage } from "../pages/ClientsPage";
import { ClientDetailPage } from "../pages/ClientDetailPage";
import { MessagesPage } from "../pages/MessagesPage";
import { MessageDetailPage } from "../pages/MessageDetailPage";
import { AdminUsersPage } from "../pages/AdminUsersPage";
import { GitHubAuthCallbackPage } from "../pages/GitHubAuthCallbackPage";

export function App() {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <Shell>
        <section className="page">
          <p className="muted">正在加载会话...</p>
        </section>
      </Shell>
    );
  }

  return (
    <Shell>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route
          path="/login"
          element={user ? <Navigate to="/" replace /> : <LoginPage />}
        />
        <Route
          path="/register"
          element={user ? <Navigate to="/" replace /> : <RegisterPage />}
        />
        <Route
          path="/reset-password"
          element={user ? <Navigate to="/" replace /> : <ResetPasswordPage />}
        />
        <Route
          path="/auth/github/callback"
          element={<GitHubAuthCallbackPage />}
        />
        <Route
          path="/clients"
          element={user ? <ClientsPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/clients/:clientId"
          element={user ? <ClientDetailPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/messages"
          element={user ? <MessagesPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/messages/:messageId"
          element={user ? <MessageDetailPage /> : <Navigate to="/login" replace />}
        />
        <Route
          path="/admin/users"
          element={
            user?.role === "admin" ? <AdminUsersPage /> : <Navigate to="/" replace />
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Shell>
  );
}
