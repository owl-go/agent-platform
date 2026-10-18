ALTER TABLE workflows
    ADD COLUMN execution_coordination jsonb,
    ADD COLUMN execution_team_profile jsonb,
    ADD CONSTRAINT workflows_execution_coordination_object CHECK (
        execution_coordination IS NULL OR execution_coordination = 'null'::jsonb OR jsonb_typeof(execution_coordination) = 'object'
    ),
    ADD CONSTRAINT workflows_execution_team_profile_object CHECK (
        execution_team_profile IS NULL OR execution_team_profile = 'null'::jsonb OR jsonb_typeof(execution_team_profile) = 'object'
    );
COMMENT ON COLUMN workflows.execution_coordination IS 'Frozen strategy copied together with Session member definitions; null retains the historical ordered-stage strategy.';
