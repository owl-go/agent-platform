-- Credits are an anti-abuse entitlement. Amounts are stored as hundredths of
-- one Credit, and multipliers are stored as millionths to avoid float math.

CREATE TABLE credit_accounts (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    credit_day date NOT NULL,
    credit_day_timezone text NOT NULL,
    daily_allocation_hundredths bigint NOT NULL DEFAULT 60000 CHECK (daily_allocation_hundredths >= 0),
    daily_remaining_hundredths bigint NOT NULL DEFAULT 60000,
    persistent_hundredths bigint NOT NULL DEFAULT 0,
    today_consumed_hundredths bigint NOT NULL DEFAULT 0 CHECK (today_consumed_hundredths >= 0),
    pending_daily_allocation_hundredths bigint CHECK (pending_daily_allocation_hundredths >= 0),
    pending_effective_day date,
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK ((pending_daily_allocation_hundredths IS NULL) = (pending_effective_day IS NULL))
);

CREATE TABLE credit_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    entry_type text NOT NULL CHECK (entry_type IN ('daily_allocation', 'daily_expiry', 'consumption', 'redemption', 'adjustment')),
    amount_hundredths bigint NOT NULL,
    daily_delta_hundredths bigint NOT NULL DEFAULT 0,
    persistent_delta_hundredths bigint NOT NULL DEFAULT 0,
    resulting_balance_hundredths bigint NOT NULL,
    credit_day date NOT NULL,
    source text,
    reason text,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (amount_hundredths = daily_delta_hundredths + persistent_delta_hundredths)
);

CREATE UNIQUE INDEX credit_ledger_idempotent_source
    ON credit_ledger (user_id, source) WHERE source IS NOT NULL;
CREATE INDEX credit_ledger_owner_history ON credit_ledger (user_id, created_at DESC, id DESC);

CREATE TABLE model_credit_rate_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_type text,
    api_protocol text,
    provider_model_id text,
    input_multiplier_micros bigint NOT NULL CHECK (input_multiplier_micros >= 0),
    output_multiplier_micros bigint NOT NULL CHECK (output_multiplier_micros >= 0),
    fallback_hundredths bigint NOT NULL CHECK (fallback_hundredths >= 0),
    predecessor_id uuid REFERENCES model_credit_rate_revisions(id) ON DELETE RESTRICT,
    created_by_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    superseded_at timestamptz,
    CHECK (
        (provider_type IS NULL AND api_protocol IS NULL AND provider_model_id IS NULL)
        OR (provider_type IS NOT NULL AND api_protocol IS NOT NULL AND provider_model_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX model_credit_rate_current_default
    ON model_credit_rate_revisions ((true))
    WHERE provider_type IS NULL AND superseded_at IS NULL;
CREATE UNIQUE INDEX model_credit_rate_current_exact
    ON model_credit_rate_revisions (provider_type, api_protocol, provider_model_id)
    WHERE provider_type IS NOT NULL AND superseded_at IS NULL;

INSERT INTO model_credit_rate_revisions (
    input_multiplier_micros, output_multiplier_micros, fallback_hundredths
) VALUES (1000000, 1000000, 1000);

CREATE TABLE credit_stage_admissions (
    source text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    execution_id text NOT NULL,
    stage_position integer NOT NULL CHECK (stage_position > 0),
    credit_day date NOT NULL,
    credit_day_timezone text NOT NULL,
    rate_revision_id uuid NOT NULL REFERENCES model_credit_rate_revisions(id) ON DELETE RESTRICT,
    input_multiplier_micros bigint NOT NULL CHECK (input_multiplier_micros >= 0),
    output_multiplier_micros bigint NOT NULL CHECK (output_multiplier_micros >= 0),
    fallback_hundredths bigint NOT NULL CHECK (fallback_hundredths >= 0),
    started_at timestamptz NOT NULL,
    settled_at timestamptz,
    UNIQUE (execution_id, stage_position)
);

-- The row is the cross-entry-point per-User model-invocation lease. It is
-- removed only by settlement or explicit pre-model abort.
CREATE TABLE credit_execution_leases (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    source text NOT NULL UNIQUE REFERENCES credit_stage_admissions(source) ON DELETE RESTRICT,
    acquired_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE redemption_code_batches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    code_count integer NOT NULL CHECK (code_count BETWEEN 1 AND 100),
    value_hundredths bigint NOT NULL CHECK (value_hundredths > 0),
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE redemption_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id uuid NOT NULL REFERENCES redemption_code_batches(id) ON DELETE RESTRICT,
    code_identifier text NOT NULL UNIQUE,
    verifier bytea NOT NULL,
    voided_at timestamptz,
    redeemed_by_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
    redeemed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((redeemed_by_user_id IS NULL) = (redeemed_at IS NULL))
);

CREATE INDEX redemption_codes_batch ON redemption_codes (batch_id, created_at, id);

ALTER TABLE session_messages ADD COLUMN credit_consumption jsonb;
ALTER TABLE runs ADD COLUMN credit_consumption jsonb;

INSERT INTO credit_accounts (
    user_id, credit_day, credit_day_timezone, daily_allocation_hundredths,
    daily_remaining_hundredths, persistent_hundredths, today_consumed_hundredths
)
SELECT users.id,
       (now() AT TIME ZONE COALESCE(settings.timezone, 'Asia/Shanghai'))::date,
       COALESCE(settings.timezone, 'Asia/Shanghai'),
       60000, 60000, 0, 0
FROM users
LEFT JOIN personal_settings settings ON settings.user_id = users.id
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO credit_ledger (
    user_id, entry_type, amount_hundredths, daily_delta_hundredths,
    resulting_balance_hundredths, credit_day, source, reason
)
SELECT account.user_id, 'daily_allocation', 60000, 60000, 60000,
       account.credit_day, 'daily:' || account.credit_day::text, 'initial rollout allocation'
FROM credit_accounts account
ON CONFLICT (user_id, source) WHERE source IS NOT NULL DO NOTHING;
