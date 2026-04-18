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
    input_tokens BIGINT,
    output_tokens BIGINT,
    total_tokens BIGINT,
    status TEXT NOT NULL DEFAULT 'pending',
    error_code TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT chat_runs_status_check CHECK (status IN ('pending', 'streaming', 'completed', 'failed'))
);

CREATE TABLE image_generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_sessions (id) ON DELETE CASCADE,
    mode TEXT NOT NULL,
    prompt TEXT NOT NULL,
    resolution_key TEXT NOT NULL,
    width BIGINT NOT NULL,
    height BIGINT NOT NULL,
    requested_image_count BIGINT NOT NULL,
    provider_name TEXT NOT NULL,
    provider_model TEXT NOT NULL,
    provider_job_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    CONSTRAINT image_generations_mode_check CHECK (mode IN ('text_to_image', 'image_edit')),
    CONSTRAINT image_generations_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    CONSTRAINT image_generations_width_check CHECK (width > 0),
    CONSTRAINT image_generations_height_check CHECK (height > 0),
    CONSTRAINT image_generations_requested_image_count_check CHECK (requested_image_count > 0)
);

CREATE TABLE image_generation_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    generation_id UUID NOT NULL REFERENCES image_generations (id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    sort_order BIGINT NOT NULL DEFAULT 0,
    blob_path TEXT NOT NULL,
    media_type TEXT NOT NULL,
    filename TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 TEXT NOT NULL,
    width BIGINT,
    height BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT image_generation_assets_role_check CHECK (role IN ('input', 'output')),
    CONSTRAINT image_generation_assets_sort_order_check CHECK (sort_order >= 0),
    CONSTRAINT image_generation_assets_size_bytes_check CHECK (size_bytes > 0),
    CONSTRAINT image_generation_assets_width_check CHECK (width IS NULL OR width > 0),
    CONSTRAINT image_generation_assets_height_check CHECK (height IS NULL OR height > 0)
);

CREATE TABLE presentation_generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_sessions (id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    planner_output_json JSONB,
    dialect_json JSONB,
    provider_name TEXT NOT NULL,
    provider_model TEXT NOT NULL,
    provider_job_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    CONSTRAINT presentation_generations_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed'))
);

CREATE TABLE presentation_generation_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    generation_id UUID NOT NULL REFERENCES presentation_generations (id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    asset_ref TEXT,
    source_type TEXT,
    source_asset_id UUID,
    source_ref TEXT,
    sort_order BIGINT NOT NULL DEFAULT 0,
    blob_path TEXT NOT NULL,
    media_type TEXT NOT NULL,
    filename TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT presentation_generation_assets_role_check CHECK (role IN ('input', 'resolved', 'output')),
    CONSTRAINT presentation_generation_assets_source_type_check CHECK (
        source_type IS NULL OR source_type IN ('input_asset', 'theme_bundle')
    ),
    CONSTRAINT presentation_generation_assets_resolved_metadata_check CHECK (
        (role = 'resolved' AND asset_ref IS NOT NULL AND source_type IS NOT NULL)
        OR (role <> 'resolved' AND asset_ref IS NULL AND source_type IS NULL AND source_asset_id IS NULL AND source_ref IS NULL)
    ),
    CONSTRAINT presentation_generation_assets_input_asset_source_check CHECK (
        source_type <> 'input_asset' OR source_asset_id IS NOT NULL
    ),
    CONSTRAINT presentation_generation_assets_theme_bundle_source_check CHECK (
        source_type <> 'theme_bundle' OR source_ref IS NOT NULL
    ),
    CONSTRAINT presentation_generation_assets_sort_order_check CHECK (sort_order >= 0),
    CONSTRAINT presentation_generation_assets_size_bytes_check CHECK (size_bytes > 0),
    CONSTRAINT presentation_generation_assets_media_type_check CHECK (
        (role IN ('input', 'resolved') AND media_type IN ('image/jpeg', 'image/png', 'image/webp', 'application/pdf'))
        OR (role = 'output' AND media_type = 'application/vnd.openxmlformats-officedocument.presentationml.presentation')
    )
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

CREATE INDEX image_generations_session_created_idx
    ON image_generations (session_id, created_at, id);

CREATE INDEX image_generation_assets_generation_role_sort_idx
    ON image_generation_assets (generation_id, role, sort_order, created_at, id);

CREATE INDEX presentation_generations_session_created_idx
    ON presentation_generations (session_id, created_at, id);

CREATE INDEX presentation_generation_assets_generation_role_sort_idx
    ON presentation_generation_assets (generation_id, role, sort_order, created_at, id);
