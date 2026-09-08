---
status: accepted
---

# Call Image Models outside Runtime Engines

AI Creation invokes Administrator-configured Image Models directly through an Images protocol rather than routing image generation through Claude Code, Codex, or another Runtime Engine. An Image Model reuses a global Model Provider Connection's protected Endpoint and API Key but owns its image-specific model identifier, supported request options, and Image Credit Rates; this keeps structured image parameters and per-image settlement outside the text-conversation execution contract while avoiding duplicate provider credentials.

User-requested Prompt Optimization also invokes an Administrator-approved Provider Model directly through its configured text protocol so the User can choose the optimization model without creating or changing a Runtime Engine session. Image Generation Records therefore have their own immutable lifecycle and Generated Images instead of masquerading as Session responses, Runs, or Artifacts. Runtime Engines remain responsible for conversational and Workflow execution, and Provider Models remain free of a generic model-type classification.
