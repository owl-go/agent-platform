# 13 — Enable Feishu CLI with one User application

**What to build:** Let a User enable the official Feishu CLI by following its registration flow once, creating exactly one Feishu developer application for that Agent Workspace User.

**Blocked by:** 11 — Execute low-risk CLI commands through the common Wrapper.

**Status:** ready-for-agent

- [ ] The available Feishu Connector Definition uses a fixed reviewed version and integrity of the official `@larksuite/cli` package.
- [ ] Browsing the Feishu Connector requires no authorization; choosing Enable starts the official application-registration/device flow and exposes a link for required User action.
- [ ] Successful registration binds the returned App ID and App Secret to one User-private Feishu CLI Application using encrypted, write-only storage.
- [ ] A database uniqueness boundary and idempotent application workflow ensure retries and concurrent enablement return the same User application rather than creating duplicates.
- [ ] Enablement requests only a platform-reviewed subset of officially review-free identity and diagnostic scopes, with no business-data or write scope.
- [ ] The product displays the application name actually returned by Feishu and provides a developer-console link for optional manual renaming.
- [ ] Another User receives a separate application, while an Administrator cannot inspect the first User's App ID, App Secret, authorization content, or registration payload.
- [ ] Registration cancellation, timeout, malformed provider response, retry, concurrent completion, Secret canary, owner isolation, API, and responsive UI tests use a fake Feishu port.
