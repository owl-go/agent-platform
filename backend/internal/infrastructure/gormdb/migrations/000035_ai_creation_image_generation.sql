CREATE TABLE image_models (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    current_revision_id uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE image_model_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    image_model_id uuid NOT NULL REFERENCES image_models(id) ON DELETE RESTRICT,
    predecessor_id uuid REFERENCES image_model_revisions(id) ON DELETE RESTRICT,
    display_name text NOT NULL,
    connection_id uuid NOT NULL REFERENCES model_provider_connections(id) ON DELETE RESTRICT,
    connection_version bigint NOT NULL CHECK (connection_version > 0),
    connection_name text NOT NULL,
    provider_model_id text NOT NULL,
    api_protocol text NOT NULL CHECK (api_protocol = 'openai_images'),
    modes jsonb NOT NULL,
    sizes jsonb NOT NULL,
    qualities jsonb NOT NULL,
    formats jsonb NOT NULL,
    backgrounds jsonb NOT NULL,
    default_size text NOT NULL,
    default_quality text NOT NULL,
    default_format text NOT NULL,
    default_background text NOT NULL,
    rates jsonb NOT NULL,
    state text NOT NULL CHECK (state IN ('unverified', 'available', 'disabled', 'deleted')),
    verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL CHECK (version > 0)
);

ALTER TABLE image_models
    ADD CONSTRAINT image_models_current_revision
    FOREIGN KEY (current_revision_id) REFERENCES image_model_revisions(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX image_model_current_revision ON image_model_revisions (image_model_id, id);
CREATE INDEX image_model_available_catalog ON image_model_revisions (state, updated_at DESC);

CREATE TABLE prompt_optimization_candidates (
    provider_model_id uuid PRIMARY KEY REFERENCES provider_models(id) ON DELETE CASCADE,
    api_protocol text NOT NULL CHECK (api_protocol IN ('openai_responses', 'openai_chat_completions')),
    created_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE image_generation_preferences (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    image_model_id uuid REFERENCES image_models(id) ON DELETE SET NULL,
    options jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE image_reference_uploads (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    object_key text NOT NULL UNIQUE,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    media_type text NOT NULL CHECK (media_type IN ('image/png', 'image/jpeg', 'image/webp')),
    encoded_size bigint NOT NULL CHECK (encoded_size > 0 AND encoded_size <= 20971520),
    width integer NOT NULL CHECK (width > 0),
    height integer NOT NULL CHECK (height > 0),
    expires_at timestamptz NOT NULL,
    bound_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((width::bigint * height::bigint) <= 64000000)
);

CREATE INDEX image_reference_upload_expiry ON image_reference_uploads (expires_at) WHERE bound_at IS NULL;

CREATE TABLE image_generation_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id text NOT NULL,
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    image_model_id uuid NOT NULL REFERENCES image_models(id) ON DELETE RESTRICT,
    image_model_revision_id uuid NOT NULL REFERENCES image_model_revisions(id) ON DELETE RESTRICT,
    model_snapshot jsonb NOT NULL,
    original_prompt text NOT NULL,
    submitted_prompt text NOT NULL,
    request_snapshot jsonb NOT NULL,
    requested_count integer NOT NULL CHECK (requested_count BETWEEN 1 AND 4),
    validated_count integer NOT NULL DEFAULT 0 CHECK (validated_count BETWEEN 0 AND 4),
    reservation_hundredths bigint NOT NULL CHECK (reservation_hundredths >= 0),
    consumption_hundredths bigint NOT NULL DEFAULT 0 CHECK (consumption_hundredths >= 0),
    reservation_credit_day date NOT NULL,
    state text NOT NULL CHECK (state IN ('pending', 'running', 'succeeded', 'partially_succeeded', 'failed', 'cancelled', 'outcome_unknown')),
    dispatch_marked boolean NOT NULL DEFAULT false,
    cancellation_requested boolean NOT NULL DEFAULT false,
    safe_error text,
    lease_expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    completed_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL CHECK (version > 0)
);

CREATE UNIQUE INDEX image_generation_one_active_per_user
    ON image_generation_records (owner_user_id)
    WHERE state IN ('pending', 'running') AND deleted_at IS NULL;
CREATE UNIQUE INDEX image_generation_idempotency
    ON image_generation_records (owner_user_id, request_id);
CREATE INDEX image_generation_owner_history
    ON image_generation_records (owner_user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX image_generation_work_queue
    ON image_generation_records (created_at, id)
    WHERE state = 'pending' AND deleted_at IS NULL;

CREATE TABLE image_generation_events (
    record_id uuid NOT NULL REFERENCES image_generation_records(id) ON DELETE CASCADE,
    sequence bigint NOT NULL CHECK (sequence > 0),
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (record_id, sequence)
);

CREATE TABLE image_generation_references (
    record_id uuid NOT NULL REFERENCES image_generation_records(id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 1 AND 10),
    object_key text NOT NULL UNIQUE,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    media_type text NOT NULL CHECK (media_type IN ('image/png', 'image/jpeg', 'image/webp')),
    encoded_size bigint NOT NULL CHECK (encoded_size > 0 AND encoded_size <= 20971520),
    width integer NOT NULL CHECK (width > 0),
    height integer NOT NULL CHECK (height > 0),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (record_id, position),
    CHECK ((width::bigint * height::bigint) <= 64000000)
);

CREATE TABLE generated_images (
    record_id uuid NOT NULL REFERENCES image_generation_records(id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 1 AND 4),
    object_key text NOT NULL UNIQUE,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    media_type text NOT NULL CHECK (media_type IN ('image/png', 'image/jpeg', 'image/webp')),
    encoded_size bigint NOT NULL CHECK (encoded_size > 0 AND encoded_size <= 26214400),
    width integer NOT NULL CHECK (width > 0),
    height integer NOT NULL CHECK (height > 0),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (record_id, position),
    CHECK ((width::bigint * height::bigint) <= 64000000)
);

CREATE TABLE image_credit_reservations (
    record_id uuid PRIMARY KEY REFERENCES image_generation_records(id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    credit_day date NOT NULL,
    credit_day_timezone text NOT NULL,
    amount_hundredths bigint NOT NULL CHECK (amount_hundredths >= 0),
    daily_reserved_hundredths bigint NOT NULL CHECK (daily_reserved_hundredths >= 0),
    persistent_reserved_hundredths bigint NOT NULL CHECK (persistent_reserved_hundredths >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    settled_at timestamptz,
    CHECK (amount_hundredths = daily_reserved_hundredths + persistent_reserved_hundredths)
);

CREATE UNIQUE INDEX image_credit_one_active_per_user
    ON image_credit_reservations (user_id) WHERE settled_at IS NULL;

CREATE TABLE ai_creation_object_cleanup (
    object_key text PRIMARY KEY,
    expires_at timestamptz NOT NULL
);
CREATE INDEX ai_creation_object_cleanup_expiry ON ai_creation_object_cleanup (expires_at);
