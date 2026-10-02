---
status: accepted
---

# RAGFlow ingestion and immutable Knowledge Index Generations

RAGFlow replaces the retired external Retrieval Provider in ADR-0040 (`0040-remove-external-retrieval-provider.md`). Knowledge ownership, authorization, source Object Storage, revisions, generations and citations remain platform-owned. The trusted API/Worker adapter uses RAGFlow v0.24.0 dataset, document parsing and retrieval APIs; Runtime containers and browsers never receive its credential.

One private dataset belongs to each Knowledge Base and deployment identity. Every immutable Document Revision has a separate provider document, recorded in trusted mapping storage. Upload alone is insufficient: parsing must reach DONE with a positive chunk count and an actually visible, available indexed chunk before the Worker commits Ready state, a new generation, its immutable revision manifest and the succeeded job in one transaction. Retry resumes an existing uploaded/processing document. Leases recover processing jobs after restart, and bounded automatic retries preserve a previous Ready revision.

The manifest deliberately retains previous provider documents. Overwriting one mutable dataset or deleting the previous revision immediately would make a queued Run silently read replacement content. Retrieval specifies both dataset_ids and the frozen manifest's document_ids. Every candidate is resolved to a manifest member under current private/Department/Platform access and active source checks. Interactive preview and Smart Assistants select the latest Ready generation; Workflow Runs can use a retained older generation. Missing generations or deployment mappings fail closed.

Deletion immediately denies retrieval through platform source validation. Provider documents and original source bytes remain available for the thirty-day restore window; cleanup removes expired provider mappings before unreferenced original source keys. Restore after expiry is rejected. A new deployment identity cannot reuse old mappings as proof of indexing; changing endpoint alone is permitted only when it reaches the same RAGFlow data, such as a renewed ngrok tunnel.

A local Apple Silicon deployment may run the pinned amd64 RAGFlow image under Docker Desktop emulation and use native Ollama `bge-m3` embeddings through host.docker.internal. Only a restricted, authenticated dataset/retrieval gateway is exposed by ngrok; RAGFlow login, registration, administrator UI and agent execution remain local. This deployment depends on the Mac and tunnel remaining available and is not evidence of server-local high availability.

Contract/database tests and live end-to-end evidence are recorded separately. Selecting this provider or starting its containers never substitutes for upload-to-index-to-hit, no-hit, replacement, frozen generations, source authorization, deletion/restore, cleanup and outage checks.
