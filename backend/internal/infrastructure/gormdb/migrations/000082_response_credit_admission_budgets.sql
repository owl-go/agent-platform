CREATE TABLE credit_response_budgets (
    user_id uuid NOT NULL REFERENCES users(id),
    response_id text NOT NULL CHECK (length(response_id) BETWEEN 1 AND 200),
    budget_hundredths bigint NOT NULL CHECK (budget_hundredths >= 0),
    PRIMARY KEY (user_id, response_id)
);
ALTER TABLE credit_stage_admissions
    ADD COLUMN response_id text NOT NULL DEFAULT '',
    ADD COLUMN response_budget_hundredths bigint NOT NULL DEFAULT 0 CHECK (response_budget_hundredths >= 0);
CREATE INDEX credit_stage_admissions_response ON credit_stage_admissions(user_id, response_id) WHERE response_id <> '';

ALTER TABLE personal_settings ADD COLUMN team_credit_budget_hundredths bigint NOT NULL DEFAULT 0 CHECK (team_credit_budget_hundredths BETWEEN 0 AND 1000000000000);
