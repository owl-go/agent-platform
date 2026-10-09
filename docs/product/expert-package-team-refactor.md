# Expert Packages and Expert Team Refactor

Status: design in progress. Three foundational directions confirmed on 2026-10-09; remaining decisions and final design review pending. No implementation or deployment is asserted.

## Confirmed directions

1. Both Experts and Expert Teams support standalone packages, import, and export. The neutral package entry is `.plugin/plugin.json`, with role definitions in `agents/`, optional bundled Skills in `skills/`, and optional profile images in `avatars/`. User-facing terminology remains Expert and Expert Team. The package contains no account credentials.
2. Each Expert's Markdown guidance is the single authoritative instruction document. Creation assistance may generate or edit that document. Introduction remains independent and display-only; a form and Markdown document must not retain separate authoritative versions of the same instruction.
3. Expert Team collaboration is coordinated by a Team Lead. The lead chooses members according to the task, delegates work, and produces the official response. The platform controls actual scheduling, limits concurrency and delegation, enforces consumption limits, and isolates member contexts. A prompt alone cannot establish that these controls exist.
4. Project documentation, package directory names, metadata extensions, and examples use neutral vocabulary. No supplier-specific names or reference links are included in these documents.

## Current implementation boundary

The implementation on the design baseline stores Experts as structured fields and catalog-resource references. Expert Teams have stable member identities and invoke every member in fixed order, with the final member providing the official response. Expert and Expert Team management does not yet provide the proposed package import/export contract, authoritative Markdown guidance, or Team Lead delegation.

This proposal changes established contracts. [ADR-0046](../adr/0046-portable-experts-and-lead-coordinated-teams.md) records the confirmed direction and the intended replacement scope for [ADR-0022](../adr/0022-platform-managed-sequential-expert-teams.md) and [ADR-0026](../adr/0026-separate-expert-guidance-from-execution-and-connectors.md). The root [glossary](../../CONTEXT.md) records the resolved domain meanings; that vocabulary is not implementation evidence. Existing product and technical contracts continue to describe current behavior until this design is finalized and their replacement scope is explicitly recorded.

## Open decisions

- Legacy Expert conversion, existing team adoption, and historical snapshot compatibility.
- Package version identity, install/update conflicts, and the scope of supported import formats.
- Whether bundled team roles reference catalog Experts or own isolated definitions.
- Bundled Skill lifecycle, external Connector dependency resolution, and authorization recovery.
- Team Lead/member permissions, context handoff, and delegation protocol.
- Numerical concurrency, delegation, timeout, and consumption limits.
- Parallel Workspace writes, result handoff, partial failure, cancellation, retry, and external side effects.
- Profile images, editing/creation flows, and shared-resource ownership scope.
- Implementation scope, required validation, and final shared-understanding review.

## Evidence

Read-only inspection established the existing contracts; no application tests, database integration checks, Runtime image checks, or production conformance have been executed for this proposal. Documentation checks are reported with each documentation commit and do not establish execution capability.
