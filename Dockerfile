# Build
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/slack-orchestrator ./cmd/slack-orchestrator

# Run
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/slack-orchestrator /app/slack-orchestrator
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/slack-orchestrator"]
