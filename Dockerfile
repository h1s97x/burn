FROM golang:1.26.2 AS builder

WORKDIR /src
COPY go.mod ./
COPY cmd/burn/ ./cmd/burn/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o burn ./cmd/burn

FROM gcr.io/distroless/static-debian13

COPY --from=builder /src/burn /burn

EXPOSE 9999
VOLUME /data

ENTRYPOINT ["/burn", "-o", "/data", "-H", "X-Forwarded-For"]
