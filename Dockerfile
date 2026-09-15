# syntax=docker/dockerfile:1

# ---- frontend build ----
FROM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# Outputs to ../internal/webui/dist (see web/vite.config.js), i.e. /src/internal/webui/dist
RUN npm run build

# ---- go build ----
FROM golang:1.27-alpine AS backend
# ca-certificates: needed at runtime for the alert webhook's outbound HTTPS
# calls (Slack/Discord); alpine doesn't ship it by default.
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=frontend /src/internal/webui/dist ./internal/webui/dist
# Both the SQLite (modernc.org/sqlite) and PostgreSQL (pgx) drivers are pure
# Go, so this builds fully static with CGO disabled — no libc needed at
# runtime, which is what lets the final stage be `scratch`.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chronosmonitor ./cmd/server

# ---- runtime ----
FROM scratch
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=backend /out/chronosmonitor /chronosmonitor

ENV CHRONOS_PORT=8080
ENV CHRONOS_DB_PATH=/data/chronos.db
ENV GIN_MODE=release
EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/chronosmonitor"]
