FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bot ./...

FROM alpine:3.19
RUN apk add --no-cache ca-certificates && adduser -D botuser
USER botuser
WORKDIR /app
ENV STATE_FILE=/data/state.json
VOLUME ["/data"]
COPY --from=build /bot /usr/local/bin/bot
CMD ["bot"]
