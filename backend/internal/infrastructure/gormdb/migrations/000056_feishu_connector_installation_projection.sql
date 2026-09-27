-- Project verified, already-published Feishu packages into the unified model.
-- The function is called once during migration and after every Feishu publication
-- change so a publication created after startup cannot permanently skip migration.
CREATE OR REPLACE FUNCTION project_verified_feishu_connector_installations() RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    WITH official AS (
        SELECT publication.package_source, publication.active_revision_id
        FROM connector_package_publications publication
        JOIN connector_revisions revision ON revision.id = publication.active_revision_id
        WHERE publication.package_source = 'feishu'
          AND publication.state = 'available'
          AND revision.mode = 'cli'
          AND revision.runtime_policy->'cli'->>'authentication_driver' = 'feishu'
          AND COALESCE(revision.runtime_policy->>'legacy_projection', 'false') <> 'true'
    ), legacy_users AS (
        SELECT DISTINCT enablement.owner_user_id
        FROM cli_connector_enablements enablement
        JOIN cli_connector_definitions definition ON definition.id = enablement.definition_id
        WHERE enablement.state = 'enabled'
          AND definition.state = 'available'
          AND definition.authentication_driver = 'feishu'
    )
    INSERT INTO connector_installations (id, owner_user_id, package_source, active_revision_id, state, version, updated_at)
    SELECT gen_random_uuid(), legacy.owner_user_id, official.package_source, official.active_revision_id, 'active', 1, now()
    FROM legacy_users legacy
    CROSS JOIN official
    ON CONFLICT (owner_user_id, package_source) DO NOTHING;

    INSERT INTO connector_provider_applications (
        owner_user_id, installation_id, provider_application_id_ciphertext,
        provider_application_secret_ciphertext, provider_name,
        developer_console_url, created_at, updated_at, version
    )
    SELECT application.owner_user_id, installation.id,
           application.provider_application_id_ciphertext,
           application.provider_application_secret_ciphertext,
           application.provider_name, application.developer_console_url,
           application.created_at, application.updated_at, application.version
    FROM feishu_cli_applications application
    JOIN connector_installations installation
      ON installation.owner_user_id = application.owner_user_id
     AND installation.package_source = 'feishu'
    ON CONFLICT (owner_user_id) DO NOTHING;

    WITH migrated AS (
        INSERT INTO connector_authorizations (
            id, owner_user_id, installation_id, identity_ref,
            external_identity_id, external_display_name, scopes,
            credential_ciphertext, credential_aad, credential_format,
            refresh_credential_ciphertext, refresh_credential_aad,
            state, expires_at, version, updated_at
        )
        SELECT gen_random_uuid(), legacy_authorization.owner_user_id, installation.id,
               'user', legacy_authorization.external_identity_id, legacy_authorization.external_display_name,
               legacy_authorization.scopes, legacy_authorization.token_ciphertext,
               'feishu-cli-authorization-token:' || legacy_authorization.owner_user_id::text || ':' || legacy_authorization.enablement_id::text || ':' || legacy_authorization.external_identity_id,
               'access_token', legacy_authorization.refresh_token_ciphertext,
               CASE WHEN legacy_authorization.refresh_token_ciphertext IS NULL THEN '' ELSE 'feishu-cli-authorization-token:' || legacy_authorization.owner_user_id::text || ':' || legacy_authorization.enablement_id::text || ':' || legacy_authorization.external_identity_id || ':refresh' END,
               'active', legacy_authorization.expires_at,
               legacy_authorization.version, legacy_authorization.updated_at
        FROM cli_connector_authorizations legacy_authorization
        JOIN connector_installations installation
          ON installation.owner_user_id = legacy_authorization.owner_user_id
         AND installation.package_source = 'feishu'
        WHERE legacy_authorization.identity = 'user'
          AND legacy_authorization.state = 'active'
          AND legacy_authorization.token_ciphertext IS NOT NULL
          AND (legacy_authorization.expires_at IS NULL OR legacy_authorization.expires_at > now())
          AND NOT EXISTS (
              SELECT 1 FROM connector_authorizations existing
              WHERE existing.installation_id = installation.id
                AND existing.identity_ref = 'user'
                AND existing.external_identity_id = legacy_authorization.external_identity_id
                AND existing.credential_aad = 'feishu-cli-authorization-token:' || legacy_authorization.owner_user_id::text || ':' || legacy_authorization.enablement_id::text || ':' || legacy_authorization.external_identity_id
          )
        RETURNING id, installation_id, updated_at
    ), selected AS (
        SELECT DISTINCT ON (installation_id) id, installation_id
        FROM (
            SELECT id, installation_id, updated_at FROM migrated
            UNION ALL
            SELECT id, installation_id, updated_at
            FROM connector_authorizations
            WHERE identity_ref = 'user' AND state = 'active'
        ) candidates
        ORDER BY installation_id, updated_at DESC, id DESC
    )
    UPDATE connector_installations installation
    SET authorization_id = selected.id,
        updated_at = now(),
        version = installation.version + 1
    FROM selected
    WHERE installation.id = selected.installation_id
      AND installation.authorization_id IS NULL;

    -- Future snapshots bind the installation. Historical snapshots retain the
    -- immutable legacy definition they originally executed.
    WITH mapping AS (
        SELECT DISTINCT ON (enablement.owner_user_id) enablement.owner_user_id, definition.id::text AS definition_id,
               installation.id::text AS installation_id
        FROM cli_connector_enablements enablement
        JOIN cli_connector_definitions definition ON definition.id = enablement.definition_id
        JOIN connector_installations installation
          ON installation.owner_user_id = enablement.owner_user_id
         AND installation.package_source = 'feishu'
        WHERE enablement.state = 'enabled'
          AND definition.authentication_driver = 'feishu'
          AND installation.authorization_id IS NOT NULL
        ORDER BY enablement.owner_user_id, enablement.updated_at DESC, enablement.id DESC
    ), rewritten AS (
        SELECT expert.id,
               COALESCE(jsonb_agg(
                   CASE WHEN item.value = mapping.definition_id THEN to_jsonb(mapping.installation_id) ELSE to_jsonb(item.value) END
                   ORDER BY item.ordinal
               ), '[]'::jsonb) AS ids
        FROM experts expert
        JOIN mapping ON expert.owner_user_id = mapping.owner_user_id
        CROSS JOIN LATERAL jsonb_array_elements_text(expert.cli_connector_definition_ids) WITH ORDINALITY AS item(value, ordinal)
        WHERE expert.cli_connector_definition_ids ? mapping.definition_id
          AND NOT expert.cli_connector_definition_ids ? mapping.installation_id
        GROUP BY expert.id
    )
    UPDATE experts expert
    SET cli_connector_definition_ids = rewritten.ids,
        updated_at = now(),
        version = expert.version + 1
    FROM rewritten
    WHERE expert.id = rewritten.id;
END;
$$;

CREATE OR REPLACE FUNCTION project_feishu_connector_publication_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.package_source = 'feishu' AND NEW.state = 'available' THEN
        PERFORM project_verified_feishu_connector_installations();
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER connector_feishu_publication_projection
AFTER INSERT OR UPDATE OF active_revision_id, state ON connector_package_publications
FOR EACH ROW EXECUTE FUNCTION project_feishu_connector_publication_change();

SELECT project_verified_feishu_connector_installations();
