FROM node:24-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /spotter ./cmd/spotter

FROM alpine:latest
RUN apk add --no-cache ca-certificates

COPY --from=go-builder /spotter /spotter

VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/spotter"]
