ALTER TABLE presentation_generations
    ADD COLUMN planner_output_json JSONB;

ALTER TABLE presentation_generation_assets
    ADD COLUMN asset_ref TEXT,
    ADD COLUMN source_type TEXT,
    ADD COLUMN source_asset_id UUID,
    ADD COLUMN source_ref TEXT;

ALTER TABLE presentation_generation_assets
    DROP CONSTRAINT presentation_generation_assets_role_check,
    ADD CONSTRAINT presentation_generation_assets_role_check CHECK (role IN ('input', 'resolved', 'output')),
    ADD CONSTRAINT presentation_generation_assets_source_type_check CHECK (
        source_type IS NULL OR source_type IN ('input_asset', 'theme_bundle')
    ),
    ADD CONSTRAINT presentation_generation_assets_resolved_metadata_check CHECK (
        (role = 'resolved' AND asset_ref IS NOT NULL AND source_type IS NOT NULL)
        OR (role <> 'resolved' AND asset_ref IS NULL AND source_type IS NULL AND source_asset_id IS NULL AND source_ref IS NULL)
    ),
    ADD CONSTRAINT presentation_generation_assets_input_asset_source_check CHECK (
        source_type <> 'input_asset' OR source_asset_id IS NOT NULL
    ),
    ADD CONSTRAINT presentation_generation_assets_theme_bundle_source_check CHECK (
        source_type <> 'theme_bundle' OR source_ref IS NOT NULL
    ),
    DROP CONSTRAINT presentation_generation_assets_media_type_check,
    ADD CONSTRAINT presentation_generation_assets_media_type_check CHECK (
        (role IN ('input', 'resolved') AND media_type IN ('image/jpeg', 'image/png', 'image/webp', 'application/pdf'))
        OR (role = 'output' AND media_type = 'application/vnd.openxmlformats-officedocument.presentationml.presentation')
    );
