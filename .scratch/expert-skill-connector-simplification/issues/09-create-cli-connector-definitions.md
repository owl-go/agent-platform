# 09 — Create Administrator CLI Connector Definitions

**What to build:** Let an Administrator describe a governed third-party CLI without granting ordinary Users or submitted packages authority to define installation, authentication, or execution scripts.

**Blocked by:** 04 — Ship the standalone MCP Connector catalog.

**Status:** ready-for-agent

- [ ] An Administrator can create, inspect, edit, and delete a draft CLI Connector Definition; an ordinary User can only read definitions intended for catalog discovery.
- [ ] A Definition requires an exact npm package and version, expected integrity, supported architecture, and an executable selected from package `bin` metadata.
- [ ] A Definition selects a built-in authentication driver and cannot contain a Shell installer, authentication script, Wrapper script, or arbitrary executable path.
- [ ] Structured capability groups declare supported identities, argv patterns, risk, scopes, Egress, timeouts, and recommended Skills.
- [ ] Invalid package coordinates, mutable versions such as `latest`, ambiguous executables, unsafe argument patterns, and unsupported authentication fail closed.
- [ ] Draft definitions are not selectable by Experts and cannot execute.
- [ ] Administrator and User catalog views clearly distinguish lifecycle state and mutation authority.
- [ ] Authorization, validation, optimistic-concurrency, audit, API, and responsive UI tests cover Administrator and ordinary User behavior.
