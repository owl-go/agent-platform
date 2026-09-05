FROM public.ecr.aws/docker/library/node:22.22.0-bookworm-slim@sha256:dd9d21971ec4395903fa6143c2b9267d048ae01ca6d3ea96f16cb30df6187d94

COPY --chmod=0555 deploy/runtimes/cli-builder/build-cli-bundle.mjs /usr/local/bin/build-cli-bundle.mjs

ENV HOME=/tmp/home NPM_CONFIG_CACHE=/tmp/npm-cache NPM_CONFIG_UPDATE_NOTIFIER=false
WORKDIR /work
USER 65532:65532
ENTRYPOINT ["node", "/usr/local/bin/build-cli-bundle.mjs"]
