FROM node:22.22.0-bookworm-slim

ARG CLAUDE_CODE_VERSION=2.1.233

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git jq openssh-client ripgrep \
    && npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 agent \
    && useradd --uid 65532 --gid 65532 --create-home --home-dir /home/agent agent

COPY --chmod=0755 common/runtime-entrypoint.sh /usr/local/bin/runtime-entrypoint

ENV DISABLE_AUTOUPDATER=1 \
    HOME=/tmp/runtime-home \
    NPM_CONFIG_UPDATE_NOTIFIER=false

WORKDIR /workspace
USER 65532:65532

LABEL org.opencontainers.image.title="agent-platform-claude-runtime" \
      org.opencontainers.image.version="2.1.233"

ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "claude"]
