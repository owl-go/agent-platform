FROM public.ecr.aws/docker/library/python:3.13.11-slim-bookworm@sha256:20080e807bfc404f8450b185cf0fc95d553462673598549613735f70a5b4d5d0

ARG HERMES_VERSION=0.19.0

RUN apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 install -y --no-install-recommends ca-certificates curl git jq openssh-client ripgrep \
    && python -m pip install --no-cache-dir "hermes-agent==${HERMES_VERSION}" \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 agent \
    && useradd --uid 65532 --gid 65532 --create-home --home-dir /home/agent agent

COPY --chmod=0755 common/runtime-entrypoint.sh /usr/local/bin/runtime-entrypoint

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
