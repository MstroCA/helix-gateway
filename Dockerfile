FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s -X main.version=$(date +%Y%m%d)" -o helix ./cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /app/helix /helix
EXPOSE 8090 9090
ENTRYPOINT ["/helix"]
