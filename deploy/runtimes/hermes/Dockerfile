FROM public.ecr.aws/docker/library/python:3.13.11-slim-bookworm@sha256:20080e807bfc404f8450b185cf0fc95d553462673598549613735f70a5b4d5d0

ARG HERMES_VERSION=0.19.0
ARG UV_VERSION=0.8.13

RUN apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 install -y --no-install-recommends ca-certificates curl git jq nodejs npm openssh-client ripgrep \
    && python -m pip install --no-cache-dir "hermes-agent[mcp,anthropic]==${HERMES_VERSION}" "uv==${UV_VERSION}" \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 agent \
    && useradd --uid 65532 --gid 65532 --create-home --home-dir /home/agent agent \
    && install -d -m 0750 -o 65532 -g 65532 /workspace

COPY --chmod=0755 deploy/runtimes/common/runtime-entrypoint.sh /usr/local/bin/runtime-entrypoint
COPY --chmod=0755 deploy/runtimes/common/agent-cli.mjs /usr/local/bin/agent-cli

ENV HOME=/tmp/runtime-home \
    HERMES_NO_UPDATE_CHECK=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /workspace
USER 65532:65532

LABEL org.opencontainers.image.title="agent-platform-hermes-runtime" \
      org.opencontainers.image.version="0.19.0"

ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "hermes"]
