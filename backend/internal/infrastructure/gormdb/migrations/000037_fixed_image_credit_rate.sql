WITH revised AS (
    INSERT INTO image_model_revisions (
        image_model_id,
        predecessor_id,
        display_name,
        connection_id,
        connection_version,
        connection_name,
        endpoint,
        api_key_ciphertext,
        provider_model_id,
        api_protocol,
        modes,
        sizes,
        qualities,
        formats,
        backgrounds,
        default_size,
        default_quality,
        default_format,
        default_background,
        rates,
        state,
        verified_at,
        created_at,
        updated_at,
        version
    )
    SELECT
        revision.image_model_id,
        revision.id,
        revision.display_name,
        revision.connection_id,
        revision.connection_version,
        revision.connection_name,
        revision.endpoint,
        revision.api_key_ciphertext,
        revision.provider_model_id,
        revision.api_protocol,
        revision.modes,
        revision.sizes,
        CASE
            WHEN revision.endpoint ~ '^https?://[^/]+/api/v1/services/aigc/multimodal-generation/generation/?$'
                THEN '["auto"]'::jsonb
            ELSE revision.qualities
        END,
        CASE
            WHEN revision.endpoint ~ '^https?://[^/]+/api/v1/services/aigc/multimodal-generation/generation/?$'
                THEN '["png"]'::jsonb
            ELSE revision.formats
        END,
        CASE
            WHEN revision.endpoint ~ '^https?://[^/]+/api/v1/services/aigc/multimodal-generation/generation/?$'
                THEN '["opaque"]'::jsonb
            ELSE revision.backgrounds
        END,
        revision.default_size,
        CASE
            WHEN revision.endpoint ~ '^https?://[^/]+/api/v1/services/aigc/multimodal-generation/generation/?$' THEN 'auto'
            ELSE revision.default_quality
        END,
        CASE
            WHEN revision.endpoint ~ '^https?://[^/]+/api/v1/services/aigc/multimodal-generation/generation/?$' THEN 'png'
            ELSE revision.default_format
        END,
        CASE
            WHEN revision.endpoint ~ '^https?://[^/]+/api/v1/services/aigc/multimodal-generation/generation/?$' THEN 'opaque'
            ELSE revision.default_background
        END,
        (
            SELECT jsonb_object_agg(rate.key, to_jsonb(5000::bigint))
            FROM jsonb_each(revision.rates) AS rate
        ),
        revision.state,
        revision.verified_at,
        now(),
        now(),
        revision.version + 1
    FROM image_models model
    JOIN image_model_revisions revision ON revision.id = model.current_revision_id
    RETURNING image_model_id, id
)
UPDATE image_models model
SET current_revision_id = revised.id
FROM revised
WHERE model.id = revised.image_model_id;
