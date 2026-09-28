CREATE TABLE credit_policy (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    default_daily_allocation_hundredths bigint NOT NULL CHECK (default_daily_allocation_hundredths >= 0),
    warning_threshold_percent integer NOT NULL CHECK (warning_threshold_percent BETWEEN 1 AND 99),
    redemption_codes_enabled boolean NOT NULL DEFAULT false,
    updated_by_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);

INSERT INTO credit_policy(default_daily_allocation_hundredths, warning_threshold_percent, redemption_codes_enabled)
VALUES (60000, 80, false);

COMMENT ON TABLE credit_policy IS 'Enterprise-wide defaults for new Credit accounts and optional channel-only Redemption Codes.';
