FROM public.ecr.aws/docker/library/node:22.22.0-bookworm-slim@sha256:dd9d21971ec4395903fa6143c2b9267d048ae01ca6d3ea96f16cb30df6187d94

ARG CLAUDE_CODE_VERSION=2.1.233

RUN apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 install -y --no-install-recommends ca-certificates curl git jq openssh-client python3-pip ripgrep \
    && python3 -m pip install --break-system-packages --no-cache-dir "uv==0.8.13" \
    && npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
    && npm cache clean --force \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 agent \
    && useradd --uid 65532 --gid 65532 --create-home --home-dir /home/agent agent \
    && install -d -m 0750 -o 65532 -g 65532 /workspace

COPY --chmod=0755 deploy/runtimes/common/runtime-entrypoint.sh /usr/local/bin/runtime-entrypoint
COPY --chmod=0755 deploy/runtimes/common/agent-cli.mjs /usr/local/bin/agent-cli

ENV DISABLE_AUTOUPDATER=1 \
    HOME=/tmp/runtime-home \
    NPM_CONFIG_UPDATE_NOTIFIER=false

WORKDIR /workspace
USER 65532:65532

LABEL org.opencontainers.image.title="agent-platform-claude-runtime" \
      org.opencontainers.image.version="2.1.233"

ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "claude"]
