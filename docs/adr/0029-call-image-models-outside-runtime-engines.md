---
status: accepted
---

# Call Image Models outside Runtime Engines

AI Creation invokes Administrator-configured Image Models directly through an Images protocol rather than routing image generation through Claude Code, Codex, or another Runtime Engine. ADR-0031 supersedes this document's original credential-reuse decision: Image Models and Prompt Optimization now own independent Endpoint and API Key settings and do not reference the Model Provider catalog.

User-requested Prompt Optimization also invokes the independently configured text model directly without creating or changing a Runtime Engine session. Image Generation Records therefore have their own immutable lifecycle and Generated Images instead of masquerading as Session responses, Runs, or Artifacts. Runtime Engines remain responsible for conversational and Workflow execution, and Provider Models remain free of a generic model-type classification.
