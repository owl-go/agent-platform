ALTER TABLE smart_assistants
    ADD COLUMN last_validated_at timestamptz,
    ADD COLUMN validated_version bigint NOT NULL DEFAULT 0;

-- Existing unrestricted shares predate the controlled-publication contract.
-- Revoke them instead of silently presenting them as reviewed. Owners may
-- publish again after acknowledging data handling, setting origins and a cap.
UPDATE smart_assistants
SET share = jsonb_set(share, '{enabled}', 'false'::jsonb, true),
    share_token_hash = NULL,
    share_token_revision = share_token_revision + 1,
    updated_at = now(),
    version = version + 1
WHERE COALESCE((share ->> 'enabled')::boolean, false)
  AND (
      COALESCE(jsonb_array_length(share -> 'allowed_origins'), 0) = 0
      OR COALESCE((share ->> 'daily_call_limit')::integer, 0) <= 0
      OR NOT COALESCE((share ->> 'data_processing_acknowledged')::boolean, false)
  );

CREATE INDEX smart_assistants_publication_validation
    ON smart_assistants(owner_user_id, last_validated_at DESC)
    WHERE last_validated_at IS NOT NULL;
