CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE chat_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chat_sessions_status_check CHECK (status IN ('active'))
);

CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_sessions (id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content_json JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    model_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chat_messages_role_check CHECK (role IN ('system', 'user', 'assistant')),
    CONSTRAINT chat_messages_status_check CHECK (status IN ('pending', 'completed', 'failed'))
);

CREATE TABLE chat_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_sessions (id) ON DELETE CASCADE,
    message_id UUID NOT NULL REFERENCES chat_messages (id) ON DELETE CASCADE,
    blob_path TEXT NOT NULL,
    media_type TEXT NOT NULL,
    filename TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 TEXT NOT NULL,
    provider_file_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chat_attachments_size_bytes_check CHECK (size_bytes > 0)
);

CREATE TABLE chat_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_sessions (id) ON DELETE CASCADE,
    user_message_id UUID NOT NULL REFERENCES chat_messages (id) ON DELETE CASCADE,
    assistant_message_id UUID REFERENCES chat_messages (id) ON DELETE SET NULL,
    provider_response_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error_code TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT chat_runs_status_check CHECK (status IN ('pending', 'streaming', 'completed', 'failed'))
);

CREATE INDEX chat_messages_session_created_idx
    ON chat_messages (session_id, created_at, id);

CREATE INDEX chat_attachments_session_created_idx
    ON chat_attachments (session_id, created_at, id);

CREATE INDEX chat_attachments_message_idx
    ON chat_attachments (message_id);

CREATE UNIQUE INDEX chat_runs_user_message_id_idx
    ON chat_runs (user_message_id);

CREATE INDEX chat_runs_session_started_idx
    ON chat_runs (session_id, started_at, id);

CREATE UNIQUE INDEX chat_runs_assistant_message_id_idx
    ON chat_runs (assistant_message_id)
    WHERE assistant_message_id IS NOT NULL;
