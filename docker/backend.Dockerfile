FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/ascp-server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

COPY --from=builder /app/ascp-server .
COPY config/config.yaml ./config/config.yaml

EXPOSE 8080
CMD ["./ascp-server"]
