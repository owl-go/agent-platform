FROM node:24.15.0-bookworm-slim

ARG OPENCLAW_VERSION=2026.7.1-2

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git jq openssh-client ripgrep \
    && npm install --global "openclaw@${OPENCLAW_VERSION}" \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 agent \
    && useradd --uid 65532 --gid 65532 --create-home --home-dir /home/agent agent

COPY --chmod=0755 common/runtime-entrypoint.sh /usr/local/bin/runtime-entrypoint

ENV HOME=/tmp/runtime-home \
    NPM_CONFIG_UPDATE_NOTIFIER=false \
    OPENCLAW_NO_UPDATE=1

WORKDIR /workspace
USER 65532:65532

LABEL org.opencontainers.image.title="agent-platform-openclaw-runtime" \
      org.opencontainers.image.version="2026.7.1-2"

ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "openclaw"]
