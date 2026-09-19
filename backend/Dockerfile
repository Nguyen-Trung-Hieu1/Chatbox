FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chatbox-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pod-watcher ./cmd/pod-watcher

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/chatbox-api /app/chatbox-api
COPY --from=build /out/pod-watcher /app/pod-watcher
USER app
EXPOSE 8080
ENTRYPOINT ["/app/chatbox-api"]
