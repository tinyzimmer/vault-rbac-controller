FROM scratch

LABEL org.opencontainers.image.source=https://github.com/tinyzimmer/vault-rbac-controller
LABEL org.opencontainers.image.description="Vault RBAC Controller"
LABEL org.opencontainers.image.licenses=MPL2

ARG TARGETOS linux
ARG TARGETARCH amd64
ADD dist/vault-rbac-controller_${TARGETOS}_${TARGETARCH} /manager

ENTRYPOINT ["/manager"]