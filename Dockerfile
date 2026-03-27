FROM golang:1.23-alpine AS builder
RUN apk add --no-cache poppler-utils
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o pdf2md ./cmd/pdf2md

FROM alpine:3.19
RUN apk add --no-cache poppler-utils
COPY --from=builder /app/pdf2md /usr/local/bin/pdf2md
ENTRYPOINT ["pdf2md"]
