# ---- Build ----
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server .

# ---- Runtime ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app \
    && mkdir -p /app \
    && chown app /app
WORKDIR /app
COPY --from=build /out/server /app/server
USER app
ENV APP_ENV=production \
    PORT=8081
EXPOSE 8081
CMD ["/app/server"]
