# Stage 1: Build React dashboard
FROM node:20-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS go-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /togglerino ./cmd/togglerino

# Stage 3: Minimal runtime image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go-builder /togglerino /usr/local/bin/togglerino
EXPOSE 8080
ENTRYPOINT ["togglerino"]
