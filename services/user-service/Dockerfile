FROM golang:1.24.7-alpine
RUN apk add --no-cache git
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/material-service/go.mod services/material-service/go.sum ./services/material-service/
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY proto/ ./proto/
COPY services/user-service/ ./services/user-service/
WORKDIR /app/services/user-service
RUN go work sync && go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o user-service main.go
RUN chmod +x user-service
EXPOSE 50052 8080
CMD ["./user-service"]