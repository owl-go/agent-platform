---
status: accepted
---

# Administrator-managed global Model Catalog

Model Provider Connections and Provider Models form one platform-wide Model Catalog. Every authenticated User reads the same available catalog and may reference its Provider Models from Personal Settings and Experts, while only the bootstrap Administrator may create, update, refresh, or delete a connection or manually add a Provider Model. Personal per-Runtime default selections remain User-owned references rather than copies of catalog entries.

This supersedes the User-owned Model Provider Connection boundary in the original Agent Workspace requirements. Provider credentials remain encrypted at rest and retain the credential owner's encryption scope during migration, but that scope is an internal cryptographic concern rather than resource ownership. A Worker resolves the versioned credential and its encryption scope before execution so a User can safely invoke a globally available model without receiving the API Key.

Deleting a global connection checks references across every User's Personal Settings, mutable Experts, and continuable Session or Run Conversation snapshots. The operation fails closed while any reference remains. Other User-owned resources keep their existing owner filters, and Administrator privileges do not grant access to another User's private content.
