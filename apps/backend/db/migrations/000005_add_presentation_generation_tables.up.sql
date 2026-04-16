CREATE TABLE presentation_generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_sessions (id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
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
    sort_order BIGINT NOT NULL DEFAULT 0,
    blob_path TEXT NOT NULL,
    media_type TEXT NOT NULL,
    filename TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT presentation_generation_assets_role_check CHECK (role IN ('input', 'output')),
    CONSTRAINT presentation_generation_assets_sort_order_check CHECK (sort_order >= 0),
    CONSTRAINT presentation_generation_assets_size_bytes_check CHECK (size_bytes > 0),
    CONSTRAINT presentation_generation_assets_media_type_check CHECK (
        (role = 'input' AND media_type IN ('image/jpeg', 'image/png', 'image/webp', 'application/pdf'))
        OR (role = 'output' AND media_type = 'application/vnd.openxmlformats-officedocument.presentationml.presentation')
    )
);

CREATE INDEX presentation_generations_session_created_idx
    ON presentation_generations (session_id, created_at, id);

CREATE INDEX presentation_generation_assets_generation_role_sort_idx
    ON presentation_generation_assets (generation_id, role, sort_order, created_at, id);
