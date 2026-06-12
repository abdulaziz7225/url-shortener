FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/persistency-controller ./services/persistency-controller

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
COPY --from=build /out/persistency-controller /usr/local/bin/persistency-controller
EXPOSE 9090 9091
ENTRYPOINT ["/usr/local/bin/persistency-controller"]
