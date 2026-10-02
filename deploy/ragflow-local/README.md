# Local RAGFlow for Agent Workspace

This stack uses RAGFlow v0.24.0, pinned at `sha256:508cc283f5d61d1e565820a8e0ae8d98f9a8ac4b3b8cf7aefc5f0093e4bb4c50`, under amd64 emulation on Apple Silicon. Native Ollama runs `bge-m3` on the Mac. Embedding does not use a remote model provider. RAGFlow uses its supported OpenDAL/MySQL backend for internal document copies; platform originals still belong to the existing private Object Store. Answer generation remains a separate platform Provider Model invocation.

Keep Docker Desktop running and allow enough memory/disk for Elasticsearch, MySQL and RAGFlow. The UI listens only on `http://127.0.0.1:19380`; the restricted gateway listens on `http://127.0.0.1:19381`. Dependency ports are not published. Host Ollama can stay on its default loopback listener; Docker Desktop reaches it through `host.docker.internal`.

## Start

Install/start Ollama and download the model:

```bash
brew install ollama
brew services start ollama
ollama pull bge-m3
# Confirm the digest is 7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab.
ollama cp bge-m3 bge-m3:aw-790764642607
```

Generate distinct values for `MYSQL_PASSWORD`, `ELASTIC_PASSWORD`, `REDIS_PASSWORD` and `RAGFLOW_SECRET_KEY` in a protected env file outside the checkout. Never commit that file. Copy the deployment files outside the checkout so later Git operations cannot replace active bind-mounted configuration, then start from that durable directory:

```bash
mkdir -p "$HOME/.local/share/agent-platform-ragflow/deployment"
cp deploy/ragflow-local/* "$HOME/.local/share/agent-platform-ragflow/deployment/"
docker compose --env-file "$HOME/.local/share/agent-platform-ragflow/local.env" \
  -f "$HOME/.local/share/agent-platform-ragflow/deployment/compose.yaml" up -d
```

Register a dedicated local RAGFlow account, verify that `bge-m3:aw-790764642607@Ollama` is configured under Embedding, and generate an API token in its settings. Registration is local; the gateway never exposes it. Test dataset creation, parsing and retrieval before enabling the platform provider.

## Tunnel

With an unused ngrok endpoint:

```bash
ngrok http http://127.0.0.1:19381 --inspect=false
```

The gateway forwards only `/api/v1/datasets`, dataset documents/chunks and `/api/v1/retrieval`. RAGFlow validates the Bearer token; login, registration, the UI, administrator routes, chat/agent execution and other paths return 404. Never tunnel the UI port or Ollama port. Do not enable endpoint pooling with an unrelated existing service: that load-balances traffic instead of routing knowledge requests.

If an existing ngrok endpoint is already online, use a separate account/endpoint or an explicitly authorized local reverse proxy that routes the exact Knowledge API paths to port 19381 while retaining the existing service routes. A tunnel URL change requires updating both server API and Worker configuration; keep the deployment identity unchanged while the same local data volumes are retained.

The optional `compose.shared.yaml` adds a loopback entry on port 19382 for an existing model relay on port 3000. It routes only the exact Knowledge SDK paths to RAGFlow and preserves other paths, streaming and WebSocket upgrades on the model relay. Start it with both Compose files. After explicit authorization to change the existing tunnel, retarget that tunnel to `http://127.0.0.1:19382`; the model relay container does not need to restart. Starting the optional gateway alone does not change existing public routing.

## Server configuration and validation

Add the `retrieval` block from `docs/technical/knowledge-base-rag.md` to the shared platform YAML and the four RAGFLOW variables to the protected server env file. `RAGFLOW_DEPLOYMENT_ID` identifies this durable installation, and `RAGFLOW_EMBEDDING_MODEL=bge-m3:aw-790764642607@Ollama`. Keep credentials in the trusted API/Worker boundary.

An optional live conformance test reads a protected JSON file with `Endpoint`, `APIKey`, `DeploymentID` and `EmbeddingModel`. `WORKSPACE_TEST_POSTGRES_DSN` must point to a disposable PostgreSQL test service capable of creating temporary databases. Run:

```bash
RAGFLOW_TEST_CONFIG_FILE=/path/to/protected/test-config.json \
WORKSPACE_TEST_POSTGRES_DSN=postgres://... \
go -C backend test ./internal/data/workspace/gormrepo -run TestRAGFlowLiveKnowledgeLoop -count=1 -v
```

Absent environment skips that test and is not live evidence. It checks real parse/readiness, Chinese retrieval, replacement, frozen generations, private access, deletion/restore and expired source/provider cleanup. Complete platform acceptance additionally needs an authenticated upload/search, a grounded Workflow Run and a grounded Smart Assistant conversation. Integrate and release through `main_temp` using the guarded platform deploy script and record backup, migration, service health and black-box evidence.

Stopping the Mac, Docker, Ollama or ngrok makes knowledge retrieval unavailable and causes affected platform requests to fail explicitly. Restarting with the same volumes preserves mappings. Do not delete volumes while retained sources or frozen Runs depend on them.
