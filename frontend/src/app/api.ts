const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";

export function formatTime(value: string | null | undefined): string {
  if (!value) return "-";
  const d = new Date(value);
  if (isNaN(d.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export function getGitHubAuthStartURL() {
  return `${API_BASE_URL}/api/v1/auth/github/start`;
}

export type LoginResponse = {
  accessToken: string;
  expiresAt: string;
  user: User;
};

export type SendCodeResponse = {
  sent: boolean;
  debugCode?: string;
};

export type User = {
  id: string;
  email: string;
  emailVerified: boolean;
  authSource: string;
  displayName: string;
  avatarUrl: string;
  role: string;
  status: string;
  lastLoginAt: string | null;
};

export type AdminUserListResponse = {
  items: User[];
};

export type Client = {
  id: string;
  name: string;
  clientId: string;
  clientSecret?: string;
  webhookToken: string;
  webhookSecret?: string;
  verifySignature: boolean;
  signatureHeader: string;
  signatureAlgorithm: string;
  eventTypeHeader: string;
  webhookUrl: string;
  wsEndpoint: string;
  status: string;
  online: boolean;
  lastConnectedAt: string | null;
  createdAt: string;
};

export type Message = {
  id: string;
  clientId: string;
  source: string;
  sourceLabel: string;
  eventType: string;
  httpMethod: string;
  requestPath: string;
  queryString: string;
  deliveryStatus: string;
  signatureValid: boolean;
  headersJson: string;
  payloadJson: string;
  deliveryAttempts: number;
  lastError: string;
  receivedAt: string;
  deliveredAt: string | null;
};

type APIErrorPayload = {
  error?: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });

  if (!response.ok) {
    const payload = (await response.json().catch(() => ({}))) as APIErrorPayload;
    throw new Error(payload.error || "request failed");
  }

  return response.json() as Promise<T>;
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export async function sendRegisterCode(email: string): Promise<SendCodeResponse> {
  return request<SendCodeResponse>("/api/v1/auth/register/send-code", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export async function register(email: string, code: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>("/api/v1/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, code, password }),
  });
}

export async function sendResetCode(email: string): Promise<SendCodeResponse> {
  return request<SendCodeResponse>("/api/v1/auth/password/send-code", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export async function resetPassword(email: string, code: string, password: string): Promise<{ reset: boolean }> {
  return request<{ reset: boolean }>("/api/v1/auth/password/reset", {
    method: "POST",
    body: JSON.stringify({ email, code, password }),
  });
}

export type DailyMessageCount = { date: string; count: number };
export type StatusCount = { status: string; count: number };

export type DashboardStats = {
  totalClients: number;
  onlineClients: number;
  totalMessages: number;
  delivered: number;
  failed: number;
  pending: number;
  daily: DailyMessageCount[];
  byStatus: StatusCount[];
};

export async function getDashboardStats(token: string): Promise<DashboardStats> {
  return request<DashboardStats>("/api/v1/dashboard/stats", {
    headers: { Authorization: `Bearer ${token}` },
  });
}

export async function getMe(token: string): Promise<{ user: User }> {
  return request<{ user: User }>("/api/v1/me", {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export async function getAdminUsers(token: string): Promise<AdminUserListResponse> {
  return request<AdminUserListResponse>("/api/v1/admin/users", {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export async function updateAdminUserStatus(token: string, userID: string, status: string): Promise<{ user: User }> {
  return request<{ user: User }>(`/api/v1/admin/users/${userID}/status`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ status }),
  });
}

export async function getClients(token: string): Promise<{ items: Client[] }> {
  return request<{ items: Client[] }>("/api/v1/clients", {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export async function createClient(token: string, name: string): Promise<Client> {
  return request<Client>("/api/v1/clients", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ name }),
  });
}

export async function deleteClient(token: string, clientID: string): Promise<{ status: string; clientId: string }> {
  return request<{ status: string; clientId: string }>(`/api/v1/clients/${clientID}`, {
    method: "DELETE",
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export async function getClient(token: string, clientID: string): Promise<Client> {
  return request<Client>(`/api/v1/clients/${clientID}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export async function updateClientSecurity(token: string, clientID: string, verifySignature: boolean): Promise<Client> {
  return request<Client>(`/api/v1/clients/${clientID}/security`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ verifySignature }),
  });
}

export async function updateClientHeaders(token: string, clientID: string, signatureHeader: string, signatureAlgorithm: string, eventTypeHeader: string): Promise<Client> {
  return request<Client>(`/api/v1/clients/${clientID}/headers`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ signatureHeader, signatureAlgorithm, eventTypeHeader }),
  });
}

export async function getMessages(token: string): Promise<{ items: Message[] }> {
  return request<{ items: Message[] }>("/api/v1/messages", {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export async function getClientMessages(token: string, clientID: string): Promise<{ items: Message[] }> {
  return request<{ items: Message[] }>(`/api/v1/clients/${clientID}/messages`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export async function getMessage(token: string, messageID: string): Promise<Message> {
  return request<Message>(`/api/v1/messages/${messageID}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export async function redeliverMessage(token: string, messageID: string): Promise<Message> {
  return request<Message>(`/api/v1/messages/${messageID}/redeliver`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
}
