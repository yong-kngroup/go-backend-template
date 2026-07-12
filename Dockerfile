# syntax=docker/dockerfile:1

FROM golang:1.26.2-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG APP=server
RUN case "$APP" in server|worker|cron|migrate) ;; *) echo "unsupported APP: $APP" >&2; exit 1 ;; esac \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/$APP

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app -h /app app

WORKDIR /app

COPY --from=builder /out/app ./app
COPY --from=builder /src/internal/config/config.yaml ./config.yaml
COPY --from=builder /src/db/migrations ./db/migrations

USER app

EXPOSE 8080 8081 8082

ENTRYPOINT ["/app/app"]
CMD ["-config", "/app/config.yaml"]
