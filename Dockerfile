FROM node:24-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.23-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3
RUN adduser -D -H -u 65532 app
WORKDIR /app
COPY --from=api /out/server ./server
COPY --from=web /web/dist ./web
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/server", "-addr", ":8080", "-web", "/app/web"]
