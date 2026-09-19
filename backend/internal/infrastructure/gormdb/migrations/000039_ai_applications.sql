CREATE TABLE smart_assistants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    icon text NOT NULL DEFAULT 'sparkles',
    introduction text NOT NULL DEFAULT '',
    scenario text NOT NULL DEFAULT '',
    service_goal text NOT NULL DEFAULT '',
    operating_rules text NOT NULL DEFAULT '',
    response_style text NOT NULL DEFAULT '',
    knowledge_base_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    expert_id uuid,
    expert_team_id uuid,
    digital_human_id uuid,
    share jsonb NOT NULL DEFAULT '{"enabled":false,"width":"100%","height":600}'::jsonb,
    share_token_hash text,
    share_token_revision bigint NOT NULL DEFAULT 0,
    state text NOT NULL DEFAULT 'enabled' CHECK (state IN ('enabled', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1,
    UNIQUE (owner_user_id, name)
);

CREATE INDEX smart_assistants_owner ON smart_assistants(owner_user_id, updated_at DESC);

CREATE TABLE digital_humans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    avatar_object_key text NOT NULL DEFAULT '',
    voice text NOT NULL DEFAULT '',
    language text NOT NULL DEFAULT '',
    expression_style text NOT NULL DEFAULT '',
    scene_description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1,
    UNIQUE (owner_user_id, name)
);

CREATE INDEX digital_humans_owner ON digital_humans(owner_user_id, updated_at DESC);

ALTER TABLE smart_assistants
    ADD CONSTRAINT smart_assistants_digital_human_fk
    FOREIGN KEY (digital_human_id) REFERENCES digital_humans(id) ON DELETE RESTRICT;

CREATE TABLE smart_assistant_faqs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    assistant_id uuid NOT NULL REFERENCES smart_assistants(id) ON DELETE CASCADE,
    question text NOT NULL,
    answer_markdown text NOT NULL,
    display_order integer NOT NULL DEFAULT 0,
    category text NOT NULL DEFAULT '',
    tag text NOT NULL DEFAULT '',
    icon text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1,
    UNIQUE (assistant_id, question)
);

CREATE INDEX smart_assistant_faqs_order ON smart_assistant_faqs(assistant_id, display_order, updated_at);
