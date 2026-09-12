FROM golang:1.24.6-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG SERVICE_PACKAGE
ARG VERSION=dev
RUN test -n "${SERVICE_PACKAGE}" && \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/server \
    "${SERVICE_PACKAGE}"

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/server /app/server

VOLUME ["/data/conf"]
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
CMD ["-conf", "/data/conf"]
