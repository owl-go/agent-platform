# 15 — Execute Feishu operations as User or Bot

**What to build:** Execute reviewed Feishu capabilities with explicit external identity and least-privilege recovery, including a separate User decision for every high-risk operation.

**Blocked by:** 12 — Support high-risk command approvals and User Action Waits; 14 — Manage Feishu account authorizations.

**Status:** ready-for-agent

- [ ] Reviewed Feishu capability groups map allowed command patterns to supported User, Bot, or dual identity, required scopes, risk, Egress, and target extraction.
- [ ] When an operation supports both identities, its approval asks the User to choose User or Bot and binds that choice into the one-use command digest.
- [ ] User execution uses only the selected Feishu account Authorization and never infers User authority from application or Bot credentials.
- [ ] Missing User scopes start an explicit OAuth recovery flow for that Authorization before a new approval can execute.
- [ ] Missing Bot scopes, application publication, or other application prerequisites return a direct Feishu recovery link and do not attempt the command.
- [ ] Business scopes are requested only for an enabled capability or concrete operation and never added implicitly during initial Connector enablement.
- [ ] Approved commands execute through the common Wrapper with current credentials; rejected, expired, changed, disabled, or under-scoped commands never start the Feishu process.
- [ ] Operation output is structured, bounded, redacted, and attributed to the selected external identity in User-visible activity and audit metadata.
- [ ] Fixture and provider-fake tests cover User-only, Bot-only, dual identity, missing scope, recovery, approval replay, parameter mutation, Secret canaries, and desktop/mobile flows.
