---
status: accepted
---

# Support Endpoint-selected Image Transports and fixed Image pricing

The Image Provider port keeps one platform contract while its direct adapter selects the wire protocol from the Administrator-configured Endpoint. OpenAI-compatible base Endpoints use OpenAI Images. An Endpoint whose normalized path is exactly `/api/v1/services/aigc/multimodal-generation/generation` uses Alibaba Model Studio's native synchronous image protocol. This extends ADR-0029 without introducing a provider, connection, or protocol field into Administrator settings; Image Models remain the independent model ID, Endpoint, and write-only API Key decided by ADR-0031.

The platform charges 50 Credits for each successfully Generated Image, independent of model, size, quality, format, or background. Submission reserves 50 Credits multiplied by requested count, settlement charges only validated outputs, and immutable records keep their frozen historical rate. This supersedes ADR-0031's first-release rate of 1.00 Credit. Prompt Optimization remains a separate free direct Chat Completions request and does not create Credit Consumption.
