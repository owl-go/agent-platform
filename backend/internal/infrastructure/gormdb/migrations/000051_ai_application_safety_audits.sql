CREATE TABLE IF NOT EXISTS ai_application_safety_audits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assistant_id uuid NOT NULL REFERENCES smart_assistants(id) ON DELETE CASCADE,
    access_source text NOT NULL CHECK (access_source IN ('authenticated', 'public')),
    classification text NOT NULL CHECK (classification IN ('allow', 'refuse')),
    policy_version text NOT NULL,
    credit_outcome text NOT NULL DEFAULT 'not_charged',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ai_application_safety_audits_lookup
    ON ai_application_safety_audits(assistant_id, created_at DESC);
