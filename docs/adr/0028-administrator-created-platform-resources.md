---
status: accepted
---

# Share Administrator-created catalog resources

Experts, Skills, and MCP Connectors created by the bootstrap Administrator are Platform Resources visible to every authenticated User. Resources created by an ordinary User remain private to that User. Catalogs expose this distinction directly: Platform Resources appear in the platform section, private resources appear in the current User's `My` section, and Administrator-created resources never appear in a `My` section.

Users may select Platform Resources in Sessions, Workflows, Experts, and Expert Teams, but only the creating Administrator may update, test, or delete them. Platform MCP credentials remain owned and encrypted under the Administrator identity; immutable execution snapshots retain that credential owner so a User can execute the Connector without receiving or re-owning its Secret. Deleting a platform Skill or MCP Connector detaches it from mutable Experts across all owners, while historical snapshots remain unchanged.

This decision supersedes ADR-0026's private-only Skill and MCP catalog rule and ADR-0025's statement that every non-model resource keeps an owner-only read filter. It does not grant the Administrator access to private User resources, Sessions, Workflows, Expert Teams, settings, authorizations, approvals, or execution content.
