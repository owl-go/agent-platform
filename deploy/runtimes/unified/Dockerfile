FROM public.ecr.aws/docker/library/node:24.15.0-bookworm-slim@sha256:4e6b70dd6cbfc88c8157ba19aa3d9f9cce6ba4703576d55459e45efcbc9c5f5d AS node

FROM public.ecr.aws/docker/library/python:3.13.11-slim-bookworm@sha256:20080e807bfc404f8450b185cf0fc95d553462673598549613735f70a5b4d5d0

ARG CLAUDE_CODE_VERSION=2.1.233
ARG CODEX_VERSION=0.147.0
ARG HERMES_VERSION=0.19.0
ARG OPENCLAW_VERSION=2026.7.1-2
ARG PI_VERSION=0.84.4
ARG UV_VERSION=0.8.13

# The Python base is required by Hermes. Copy the pinned Node 24 toolchain
# rather than installing an unpinned distribution through apt.
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules

RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx \
    && apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 install -y --no-install-recommends ca-certificates curl git jq openssh-client ripgrep \
    && python -m pip install --no-cache-dir "hermes-agent[mcp,anthropic]==${HERMES_VERSION}" "uv==${UV_VERSION}" \
    && npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" "@openai/codex@${CODEX_VERSION}" "openclaw@${OPENCLAW_VERSION}" \
    && npm install --global --ignore-scripts "@earendil-works/pi-coding-agent@${PI_VERSION}" \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 agent \
    && useradd --uid 65532 --gid 65532 --create-home --home-dir /home/agent agent \
    && install -d -m 0750 -o 65532 -g 65532 /workspace

COPY --chmod=0755 deploy/runtimes/common/runtime-entrypoint.sh /usr/local/bin/runtime-entrypoint
COPY --chmod=0755 deploy/runtimes/common/agent-cli.cjs /usr/local/bin/agent-cli

ENV CODEX_DISABLE_UPDATE_CHECK=1 \
    CODEX_HOME=/tmp/runtime-home/.codex \
    DISABLE_AUTOUPDATER=1 \
    HERMES_NO_UPDATE_CHECK=1 \
    HOME=/tmp/runtime-home \
    NPM_CONFIG_UPDATE_NOTIFIER=false \
    OPENCLAW_NO_UPDATE=1 \
    PI_OFFLINE=1 \
    PI_SKIP_VERSION_CHECK=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /workspace
USER 65532:65532

LABEL org.opencontainers.image.title="agent-platform-runtime" \
      org.opencontainers.image.version="1.0.0"

ENTRYPOINT ["/usr/local/bin/runtime-entrypoint"]
