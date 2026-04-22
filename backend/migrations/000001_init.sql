-- users
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    password_hash TEXT NOT NULL DEFAULT '',
    auth_source TEXT NOT NULL DEFAULT 'email',
    display_name TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- user auth providers
CREATE TABLE IF NOT EXISTS user_auth_providers (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    provider_username TEXT NOT NULL DEFAULT '',
    provider_email TEXT NOT NULL DEFAULT '',
    access_token_encrypted TEXT NOT NULL DEFAULT '',
    refresh_token_encrypted TEXT NOT NULL DEFAULT '',
    token_expires_at TIMESTAMPTZ,
    scope TEXT NOT NULL DEFAULT '',
    profile_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_user_id),
    UNIQUE (user_id, provider)
);

-- clients
CREATE TABLE IF NOT EXISTS clients (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    client_id TEXT NOT NULL UNIQUE,
    client_secret TEXT NOT NULL,
    webhook_token TEXT NOT NULL UNIQUE,
    webhook_secret TEXT NOT NULL DEFAULT '',
    webhook_url TEXT NOT NULL,
    status TEXT NOT NULL,
    last_connected_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, name)
);

-- messages
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT '',
    source_label TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL DEFAULT '',
    http_method TEXT NOT NULL DEFAULT 'POST',
    request_path TEXT NOT NULL DEFAULT '',
    query_string TEXT NOT NULL DEFAULT '',
    delivery_status TEXT NOT NULL,
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    headers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    delivery_attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_user_client_created_at
    ON messages (user_id, client_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_messages_status_created_at
    ON messages (delivery_status, created_at DESC);
