FROM golang:1.25-alpine
RUN apk add --no-cache git
WORKDIR /app
COPY go.work go.work.sum ./
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
RUN go work sync
WORKDIR /app/gateway
RUN go build -o gateway main.go
EXPOSE 8080
CMD ["./gateway"]