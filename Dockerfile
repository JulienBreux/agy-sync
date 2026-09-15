# Build stage
FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/agy-sync .

# Runtime stage
FROM alpine:3

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 -S agysync && \
    adduser -u 1000 -S agysync -G agysync

COPY --from=builder /bin/agy-sync /bin/agy-sync

USER agysync:agysync

ENTRYPOINT ["/bin/agy-sync"]
