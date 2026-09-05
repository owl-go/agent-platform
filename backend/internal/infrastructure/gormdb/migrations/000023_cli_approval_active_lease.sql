DROP INDEX cli_command_approvals_one_active_per_stage;

CREATE UNIQUE INDEX cli_command_approvals_one_active_per_stage
    ON cli_command_approvals (stage_id)
    WHERE state IN ('pending', 'approved');
