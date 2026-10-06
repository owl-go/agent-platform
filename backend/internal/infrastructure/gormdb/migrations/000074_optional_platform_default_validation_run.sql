-- Configuration saves no longer require an execution proof. Keep historical
-- evidence references readable, while new saves leave this field unset.
ALTER TABLE platform_execution_defaults ALTER COLUMN validation_run_id DROP NOT NULL;
