FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chatbox-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate-keycloak-roles ./cmd/migrate-keycloak-roles
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate-mongo-postgres ./cmd/migrate-mongo-postgres
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/check-postgres ./cmd/check-postgres

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/chatbox-api /app/chatbox-api
COPY --from=build /out/migrate-keycloak-roles /app/migrate-keycloak-roles
COPY --from=build /out/migrate-mongo-postgres /app/migrate-mongo-postgres
COPY --from=build /out/check-postgres /app/check-postgres
USER app
EXPOSE 8080
ENTRYPOINT ["/app/chatbox-api"]
