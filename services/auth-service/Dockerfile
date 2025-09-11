FROM golang:1.24.7-alpine
RUN apk add --no-cache git
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY proto proto
RUN go work sync && go mod download
COPY services/auth-service/ ./services/auth-service/
WORKDIR /app/services/auth-service
RUN go build -o auth-service main.go
EXPOSE 50051 8080
CMD ["./auth-service"]