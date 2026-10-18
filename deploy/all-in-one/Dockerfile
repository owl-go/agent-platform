FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/golang:1.25-bookworm AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend .
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go install -trimpath -ldflags="-s -w" github.com/minio/minio@RELEASE.2025-10-15T17-29-55Z \
    && if [ -f "/go/bin/${TARGETOS}_${TARGETARCH}/minio" ]; then cp "/go/bin/${TARGETOS}_${TARGETARCH}/minio" /out/minio; else cp /go/bin/minio /out/minio; fi \
    && cp /go/pkg/mod/github.com/minio/minio@*/LICENSE /out/minio.LICENSE

FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/node:24.15.0-bookworm-slim@sha256:4e6b70dd6cbfc88c8157ba19aa3d9f9cce6ba4703576d55459e45efcbc9c5f5d AS web
WORKDIR /src
COPY frontend/package.json frontend/pnpm-lock.yaml ./frontend/
RUN npm install --global pnpm@11.2.0 && pnpm --dir frontend install --frozen-lockfile
COPY frontend ./frontend
RUN pnpm --dir frontend typecheck && pnpm --dir frontend build

FROM public.ecr.aws/docker/library/postgres:17-bookworm@sha256:3645570cccdfa447589da9f57dd740faa29b30938e861289a5574b6ca6b03826 AS postgres
FROM quay.io/keycloak/keycloak@sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6 AS identity
FROM public.ecr.aws/docker/library/docker:29-cli@sha256:6fae2214c3121a28d5aa639d09032f61310b7c9c75ffbad1025f261a1b621800 AS docker

# This named context is the existing unified Runtime Dockerfile, not another
# deployed service. buildx bake resolves and reuses the same pinned CLI layers.
FROM runtime
USER 0:0
RUN apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 install -y --no-install-recommends \
       nginx-light libicu72 libxml2 libxslt1.1 liblz4-1 libzstd1 libreadline8 \
       libssl3 libldap-2.5-0 libpam0g libkrb5-3 libnuma1 liburing2 libnss3 libstdc++6 zlib1g \
    && groupadd --gid 65531 postgres && useradd --uid 65531 --gid 65531 --no-create-home postgres \
    && install -d -m 0700 /data /run/agent-platform \
    && rm -rf /var/lib/apt/lists/*
COPY --from=postgres /usr/lib/postgresql/17 /usr/lib/postgresql/17
COPY --from=postgres /usr/share/postgresql/17 /usr/share/postgresql/17
COPY --from=postgres /usr/share/postgresql/postgresql.conf.sample /usr/share/postgresql/postgresql.conf.sample
COPY --from=postgres /usr/lib/*-linux-gnu/libpq.so.5* /usr/local/lib/
RUN ldconfig
COPY --from=identity /opt/keycloak /opt/keycloak
COPY --from=identity /usr/lib/jvm/java-21-openjdk-* /opt/java
COPY --from=identity /etc/java /etc/java
COPY --from=identity /etc/pki /etc/pki
COPY --from=identity /etc/crypto-policies /etc/crypto-policies
COPY --from=identity /usr/share/crypto-policies /usr/share/crypto-policies
COPY --from=identity /usr/share/javazi-1.8 /usr/share/javazi-1.8
COPY --from=identity /usr/share/licenses /usr/share/licenses
COPY --from=docker /usr/local/bin/docker /usr/bin/docker
COPY --from=build /out/api /out/worker /out/minio /usr/local/bin/
RUN /usr/lib/postgresql/17/bin/postgres --version && /usr/lib/postgresql/17/bin/initdb --version \
    && /usr/lib/postgresql/17/bin/psql --version && /opt/java/bin/java -version \
    && test -r /usr/share/postgresql/17/postgresql.conf.sample \
    && test -r /opt/java/conf/security/java.security && test -r /opt/java/lib/security/cacerts \
    && test -r /etc/crypto-policies/back-ends/java.config && test -r /opt/java/lib/tzdb.dat \
    && JAVA_HOME=/opt/java /opt/keycloak/bin/kc.sh --help >/dev/null \
    && /usr/bin/docker --version && /usr/local/bin/minio --version
RUN JAVA_HOME=/opt/java KC_HTTP_RELATIVE_PATH=/identity JAVA_OPTS_APPEND="-Xms128m -Xmx768m" \
    /opt/keycloak/bin/kc.sh build --db=postgres
COPY --from=web /src/frontend/dist /opt/agent-platform/web
COPY resources.json /opt/agent-platform/resources/resources.json
COPY connectors /opt/agent-platform/resources/connectors
COPY skills /opt/agent-platform/resources/skills
COPY experts /opt/agent-platform/resources/experts
COPY deploy/platform/config/keycloak-realm.json /opt/agent-platform/keycloak-realm.json
COPY deploy/platform/themes/agent-workspace /opt/keycloak/themes/agent-workspace
COPY frontend/src/design-tokens.css /opt/keycloak/themes/agent-workspace/login/resources/css/design-tokens.css
COPY deploy/all-in-one/*.py /opt/agent-platform/
COPY --chmod=0755 deploy/all-in-one/docker-shim /opt/agent-platform/docker-bin/docker
COPY --from=build /out/minio.LICENSE /usr/share/licenses/minio/LICENSE
ENV AGENT_WORKSPACE_RESOURCE_ROOT=/opt/agent-platform/resources \
    JAVA_HOME=/opt/java \
    PATH=/usr/lib/postgresql/17/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ARG SOURCE_REVISION
LABEL org.opencontainers.image.title="Agent Workspace all-in-one" \
      org.opencontainers.image.source="https://github.com/owl-go/agent-platform" \
      org.opencontainers.image.revision=$SOURCE_REVISION
EXPOSE 8080
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=30s --timeout=10s --start-period=600s --retries=3 \
    CMD ["python3", "/opt/agent-platform/supervise.py", "health"]
WORKDIR /opt/agent-platform
ENTRYPOINT ["python3", "/opt/agent-platform/supervise.py"]
