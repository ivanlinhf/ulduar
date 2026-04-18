ALTER TABLE presentation_generation_assets
    DROP CONSTRAINT presentation_generation_assets_theme_bundle_source_check,
    DROP CONSTRAINT presentation_generation_assets_input_asset_source_check,
    DROP CONSTRAINT presentation_generation_assets_resolved_metadata_check,
    DROP CONSTRAINT presentation_generation_assets_source_type_check,
    DROP CONSTRAINT presentation_generation_assets_role_check,
    ADD CONSTRAINT presentation_generation_assets_role_check CHECK (role IN ('input', 'output')),
    DROP CONSTRAINT presentation_generation_assets_media_type_check,
    ADD CONSTRAINT presentation_generation_assets_media_type_check CHECK (
        (role = 'input' AND media_type IN ('image/jpeg', 'image/png', 'image/webp', 'application/pdf'))
        OR (role = 'output' AND media_type = 'application/vnd.openxmlformats-officedocument.presentationml.presentation')
    );

ALTER TABLE presentation_generation_assets
    DROP COLUMN source_ref,
    DROP COLUMN source_asset_id,
    DROP COLUMN source_type,
    DROP COLUMN asset_ref;

ALTER TABLE presentation_generations
    DROP COLUMN planner_output_json;
