#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_root}"

go -C backend test ./internal/platformconfig -run 'TestWorkerExecutionConfigurationIsFailClosed|TestValidationRejectsUnsafeDefaults' -count=1
go -C backend test ./internal/biz/execution/application -run 'TestWorkerCancelsProcessorWhenLeaseIsLost|TestWorkerAcknowledgesRequestedInterruption' -count=1
if [[ -z "${EXECUTION_DATABASE_DSN:-}" ]]; then
    if [[ "${REQUIRE_POSTGRES_SLO:-0}" == "1" ]]; then
        echo "EXECUTION_DATABASE_DSN is required for PostgreSQL SLO gates" >&2
        exit 1
    fi
    echo "PostgreSQL SLO gates skipped; set EXECUTION_DATABASE_DSN or REQUIRE_POSTGRES_SLO=1"
else
	go -C backend test ./internal/data/execution/gormrepo -run 'TestGORMRepositoryClaimsTenIndependentSessionsConcurrently|TestGORMRepositoryCancelBeginsWithinTenSeconds|TestGORMRepositoryRunControlLifecycle|TestGORMRepositoryReconcilesExpiredAttempts|TestGORMRepositorySearchesRunsWithinTeamScope' -count=1
	ARTIFACT_DATABASE_DSN="${EXECUTION_DATABASE_DSN}" go -C backend test ./internal/data/artifact/gormrepo -run TestArtifactMetadataLifecycleWithPostgreSQL -count=1
	APPROVAL_DATABASE_DSN="${EXECUTION_DATABASE_DSN}" go -C backend test ./internal/data/approval/gormrepo -run TestRunApprovalStateTransitionsWithPostgreSQL -count=1
	AUDIT_DATABASE_DSN="${EXECUTION_DATABASE_DSN}" go -C backend test ./internal/data/audit/gormrepo -run TestAuditSearchRemainsTeamScopedWithPostgreSQL -count=1
fi
