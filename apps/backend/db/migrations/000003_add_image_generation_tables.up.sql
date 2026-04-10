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

CREATE INDEX image_generations_session_created_idx
    ON image_generations (session_id, created_at, id);

CREATE INDEX image_generation_assets_generation_role_sort_idx
    ON image_generation_assets (generation_id, role, sort_order, created_at, id);
