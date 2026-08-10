FROM golang:1.24.6-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/server \
    ./app/evie/service/cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/server /app/server

ENV platform_ADMIN_ENV=production

EXPOSE 8000 9000
VOLUME ["/data/conf"]

USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
CMD ["-conf", "/data/conf"]
