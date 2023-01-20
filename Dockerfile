FROM scratch

LABEL org.opencontainers.image.source=https://github.com/tinyzimmer/vault-rbac-controller
LABEL org.opencontainers.image.description="Vault RBAC Controller"
LABEL org.opencontainers.image.licenses=MPL2

ARG TARGETARCH amd64
ADD dist/vault-rbac-controller_linux_${TARGETARCH} /manager

ENTRYPOINT ["/manager"]