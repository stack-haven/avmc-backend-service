FROM golang:1.24.6-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 默认值必须指向真实存在的服务：原默认值 ./app/platform/admin/cmd/server
# 是 platform/admin → platform/service 改名前的残留，直接 `docker build .`
# 会在 go build 阶段报 "no Go files"。app.mk 的 `make docker` 会显式传入该参数。
ARG SERVICE_PACKAGE=./app/platform/service/cmd/server
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/server \
    "${SERVICE_PACKAGE}"

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/server /app/server

ENV platform_ADMIN_ENV=production

EXPOSE 8000 9000
VOLUME ["/data/conf"]

USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
CMD ["-conf", "/data/conf"]
