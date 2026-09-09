---
status: accepted
---

# Use independent AI Creation credentials

Image Models and Prompt Optimization own API configuration that is independent from the Workspace Model Provider catalog. An Image Model stores an exact model identifier, API Endpoint, and write-only encrypted API Key. Prompt Optimization stores exactly one model identifier, API Endpoint, write-only encrypted API Key, and Administrator-authored optimization instruction.

This decision supersedes ADR-0029 only where it says that Image Models or Prompt Optimization reuse a Model Provider Connection or Provider Model. Direct invocation outside Runtime Engines remains unchanged. Read APIs expose only whether a key is configured, and migrations do not copy existing provider credentials into AI Creation. Existing connection-backed Image Models must be edited with a new key and tested before they can be enabled.

Output size, quality, count, format, and background are User request choices rather than Administrator configuration. The first release uses a fixed platform rate of 1.00 Credit for each successfully generated image so rate management does not appear in Image Generation settings.
