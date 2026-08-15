FROM node:22.22.0-bookworm-slim

ARG CODEX_VERSION=0.147.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git jq openssh-client ripgrep \
    && npm install --global "@openai/codex@${CODEX_VERSION}" \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 agent \
    && useradd --uid 65532 --gid 65532 --create-home --home-dir /home/agent agent

COPY --chmod=0755 common/runtime-entrypoint.sh /usr/local/bin/runtime-entrypoint

ENV CODEX_DISABLE_UPDATE_CHECK=1 \
    CODEX_HOME=/tmp/runtime-home/.codex \
    HOME=/tmp/runtime-home \
    NPM_CONFIG_UPDATE_NOTIFIER=false

WORKDIR /workspace
USER 65532:65532

LABEL org.opencontainers.image.title="agent-platform-codex-runtime" \
      org.opencontainers.image.version="0.147.0"

ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "codex"]
