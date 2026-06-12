FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api-gateway ./services/api-gateway

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
COPY --from=build /out/api-gateway /usr/local/bin/api-gateway
EXPOSE 8080 8081
ENTRYPOINT ["/usr/local/bin/api-gateway"]
