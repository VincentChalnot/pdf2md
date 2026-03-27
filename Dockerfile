FROM golang:1.24-bookworm AS builder
WORKDIR /app
COPY . .
RUN go build -o pdf2md .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    poppler-utils && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/pdf2md /usr/local/bin/pdf2md
WORKDIR /data
ENTRYPOINT ["pdf2md"]
