# ---- frontend build ----
FROM node:22-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- go build ----
FROM golang:1.26-alpine AS go-builder
WORKDIR /src
COPY . .
COPY --from=web-builder /src/internal/console/web/dist ./internal/console/web/dist
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/vexo .

# ---- runtime ----
FROM alpine:3.20
RUN adduser -D -u 10001 vexo
COPY --from=go-builder /out/vexo /usr/local/bin/vexo

ENV VEXO_DATA_DIR=/data \
    VEXO_API_ADDR=:9090 \
    VEXO_CONSOLE_ADDR=:9091 \
    VEXO_P2P_ADDR=:3001

RUN mkdir -p /data && chown vexo:vexo /data
USER vexo
VOLUME ["/data"]
EXPOSE 9090 9091 3001

ENTRYPOINT ["vexo"]
