# Stage 1: Build SDKs (required for web file: references)
FROM node:20-alpine AS sdk-builder
WORKDIR /app/sdks/javascript
COPY sdks/javascript/package.json sdks/javascript/package-lock.json ./
RUN npm ci
COPY sdks/javascript/ ./
RUN npm run build

WORKDIR /app/sdks/react
COPY sdks/react/package.json sdks/react/package-lock.json ./
RUN npm ci
COPY sdks/react/ ./
RUN npm run build

# Stage 2: Build React dashboard
FROM node:20-alpine AS web-builder
WORKDIR /app
COPY --from=sdk-builder /app/sdks ./sdks
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
