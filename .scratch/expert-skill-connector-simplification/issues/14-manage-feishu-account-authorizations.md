# 14 — Manage Feishu account authorizations

**What to build:** Allow a User to connect and maintain multiple Feishu accounts under their one Feishu CLI Application without sharing account Tokens or weakening revocation.

**Blocked by:** 13 — Enable Feishu CLI with one User application.

**Status:** ready-for-agent

- [ ] A User can start Feishu account authorization from the enabled Connector and complete the official User OAuth or device flow through a product link.
- [ ] Multiple external Feishu identities can coexist under the User's single application, and each Authorization has stable identity metadata and isolated encrypted Tokens.
- [ ] App-level scopes and Bot identity are shared only within that User application; User Tokens and User-granted scopes remain authorization-specific.
- [ ] Credentials are write-only through APIs and interfaces; another User and the Administrator cannot read Tokens or authorization content.
- [ ] Expiring access Tokens refresh before command execution using the protected refresh credential without changing Expert snapshots.
- [ ] Failed refresh marks only that Authorization invalid, never falls back to a different account or Bot identity, and presents an explicit reauthorization action.
- [ ] Disconnecting an Authorization immediately prevents its future use while preserving historical execution results and immutable identity references.
- [ ] OAuth state, replay, concurrent refresh, failed refresh, disconnect race, owner isolation, Secret canary, API, and responsive UI tests use deterministic provider fakes.
