# syntax=docker/dockerfile:1

FROM node:22-bookworm-slim AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend ./
RUN npm run build:web

FROM golang:1.27-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
COPY third_party ./third_party
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist ./frontend/dist
ENV CGO_ENABLED=0
RUN go build -tags goweb -trimpath -ldflags "-s -w" -o /out/go-stock-web .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    chromium \
    fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /out/go-stock-web /app/go-stock-web
COPY --from=frontend /src/frontend/dist /app/frontend/dist
ENV TZ=Asia/Shanghai \
    WEB_ADDR=:8080 \
    WEB_STATIC_DIR=/app/frontend/dist \
    GO_STOCK_ROOT_DIR=/app
EXPOSE 8080
VOLUME ["/app/data", "/app/memory", "/app/skills", "/app/logs"]
CMD ["/app/go-stock-web"]
