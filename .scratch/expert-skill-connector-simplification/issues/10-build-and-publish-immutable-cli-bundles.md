# 10 — Build and publish immutable CLI bundles

**What to build:** Turn an approved draft Definition into a verified, immutable execution bundle outside User Runs and expose availability only for combinations backed by evidence.

**Blocked by:** 09 — Create Administrator CLI Connector Definitions.

**Status:** ready-for-agent

- [ ] Publishing transitions a Definition through draft, building, testing, and available or failed states with persisted progress and failure reasons.
- [ ] The Builder runs isolated from API and Runtime processes without User credentials or User-owned content.
- [ ] The Builder fetches only the exact package version, verifies npm integrity, resolves the declared executable, and records the final lowercase SHA-256.
- [ ] The immutable bundle is stored privately through the configured Object Storage provider using a logical Object Key.
- [ ] Runtime execution can receive the bundle only as a checksum-verified read-only mount and never installs npm packages during a User Run.
- [ ] Availability is recorded for an exact bundle digest and Runtime image RepoDigest; parsing or successful build alone does not imply compatibility.
- [ ] A new upstream version compares generated commands and scopes with the prior reviewed schema and leaves additions unavailable until explicitly accepted.
- [ ] An Administrator can disable an available Definition without deleting its history or User-private state.
- [ ] Builder fake, object-store conformance, lifecycle, integrity, schema-diff, retry, cancellation, and Administrator UI tests cover success and failure paths.
