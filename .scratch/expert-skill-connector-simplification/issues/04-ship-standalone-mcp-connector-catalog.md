# 04 — Ship the standalone MCP Connector catalog

**What to build:** Complete the Connector tab so Users can create and validate private MCP Connectors without treating them as Expert-owned Extensions.

**Blocked by:** 03 — Ship the standalone Skill catalog.

**Status:** ready-for-agent

- [ ] The Connector tab lists available MCP Connectors without requiring authorization merely to browse them.
- [ ] A User can create, edit, test, and delete only their own MCP Connectors.
- [ ] Streamable HTTP accepts only valid HTTPS endpoints and write-only optional Bearer credentials.
- [ ] stdio accepts only fixed-version `npx` or `uvx` packages with structured arguments and environment; `latest`, Shell commands, and arbitrary executables fail closed.
- [ ] Testing executes in the isolated Runtime boundary and never in the API process or on the host.
- [ ] Only a successfully tested current revision can later be selected for an Expert.
- [ ] The public MCP contract uses Connector terminology and the standalone Connector route while the legacy route remains temporarily available for migration.
- [ ] Desktop/mobile UI, API, domain, isolation, Secret, and cross-owner tests cover both transports and invalid inputs.
