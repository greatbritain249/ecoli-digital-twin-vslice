FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/ecoli-server ./cmd/server

FROM alpine:3.21
RUN addgroup -S ecoli && adduser -S ecoli -G ecoli
WORKDIR /app
COPY --from=builder /out/ecoli-server /usr/local/bin/ecoli-server
COPY apps/web/ ./apps/web/
COPY data/catalog.json data/scene.json data/replay-example.json ./data/
ENV ECOLI_ADDR=:8080 ECOLI_ROOT=/app
USER ecoli
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/ecoli-server"]
