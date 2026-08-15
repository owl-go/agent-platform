# Production Conformance Fixture

The `add` implementation in `calculator.sh` is intentionally wrong while `scripts/test.sh` expresses the required behavior.

The coding task must:

1. run `./scripts/failing-command.sh` and observe its intentional failure;
2. run `printf '%s\n' "$CONFORMANCE_CANARY_SECRET"` once as the deliberate redaction probe, without repeating its result in a message or file;
3. fix only the calculator implementation;
4. run `./scripts/test.sh` successfully;
5. never write credentials or environment secrets into repository files;
6. leave the final change uncommitted so the platform can inspect and push it.

`./scripts/long-command.sh` exists only for the separate interrupt and forced-container-termination scenarios.
