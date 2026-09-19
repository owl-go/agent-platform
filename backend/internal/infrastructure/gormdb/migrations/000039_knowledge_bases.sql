CREATE TABLE knowledge_bases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform boolean NOT NULL DEFAULT false,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    visibility text NOT NULL CHECK (visibility IN ('private', 'public')),
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (platform OR visibility = 'private')
);

CREATE UNIQUE INDEX knowledge_bases_owner_name ON knowledge_bases(owner_user_id, name) WHERE deleted_at IS NULL;
CREATE INDEX knowledge_bases_catalog ON knowledge_bases(platform, visibility, deleted_at, updated_at DESC);

CREATE TABLE knowledge_categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    name text NOT NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE UNIQUE INDEX knowledge_categories_name ON knowledge_categories(knowledge_base_id, name) WHERE deleted_at IS NULL;

CREATE TABLE knowledge_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    category_id uuid REFERENCES knowledge_categories(id) ON DELETE SET NULL,
    name text NOT NULL,
    source_type text NOT NULL CHECK (source_type IN ('upload', 'url')),
    source_uri text,
    normalized_source text NOT NULL,
    state text NOT NULL CHECK (state IN ('accepted', 'processing', 'ready', 'failed', 'blocked')),
    error text NOT NULL DEFAULT '',
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK ((source_type = 'upload' AND source_uri IS NULL) OR (source_type = 'url' AND source_uri IS NOT NULL))
);

CREATE UNIQUE INDEX knowledge_documents_source ON knowledge_documents(knowledge_base_id, category_id, normalized_source) NULLS NOT DISTINCT WHERE deleted_at IS NULL;
CREATE INDEX knowledge_documents_catalog ON knowledge_documents(knowledge_base_id, category_id, deleted_at, updated_at DESC);

CREATE TABLE knowledge_document_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id uuid NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    revision integer NOT NULL CHECK (revision > 0),
    object_key text NOT NULL,
    sha256 text NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    content_type text NOT NULL,
    state text NOT NULL CHECK (state IN ('accepted', 'processing', 'ready', 'failed', 'blocked')),
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    ready_at timestamptz,
    UNIQUE (document_id, revision)
);

CREATE INDEX knowledge_document_revisions_state ON knowledge_document_revisions(document_id, state, revision DESC);

CREATE TABLE knowledge_ingestion_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    revision_id uuid NOT NULL REFERENCES knowledge_document_revisions(id) ON DELETE CASCADE,
    idempotency_key text NOT NULL UNIQUE,
    state text NOT NULL CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'blocked', 'cancelled')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    lease_expires_at timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX knowledge_ingestion_jobs_queue ON knowledge_ingestion_jobs(state, next_attempt_at, lease_expires_at);

CREATE TABLE knowledge_index_generations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    generation bigint NOT NULL CHECK (generation > 0),
    state text NOT NULL CHECK (state IN ('building', 'ready', 'failed', 'deleted')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (knowledge_base_id, generation)
);

CREATE INDEX knowledge_index_generations_current ON knowledge_index_generations(knowledge_base_id, state, generation DESC);

CREATE TABLE workflow_knowledge_bases (
    workflow_id uuid NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id) ON DELETE RESTRICT,
    PRIMARY KEY (workflow_id, knowledge_base_id)
);

ALTER TABLE workflows ADD COLUMN knowledge_base_ids jsonb NOT NULL DEFAULT '[]'::jsonb;
