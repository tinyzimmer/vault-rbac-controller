FROM golang:alpine AS builder

WORKDIR /go/src/app

ADD go.mod go.mod
ADD go.sum go.sum

RUN go mod download -x

ADD internal/ internal/
ADD main.go main.go
RUN CGO_ENABLED=0 go build -o manager

FROM scratch

LABEL org.opencontainers.image.source=https://github.com/tinyzimmer/vault-rbac-controller
LABEL org.opencontainers.image.description="Vault RBAC Controller"
LABEL org.opencontainers.image.licenses=MPL2

COPY --from=builder /go/src/app/manager /manager
ENTRYPOINT ["/manager"]