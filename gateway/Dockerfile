FROM golang:1.24.7-alpine
RUN apk add --no-cache git
WORKDIR /app
COPY go.work go.work.sum ./
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
COPY services/material-service/go.mod services/material-service/go.sum ./services/material-service/
COPY services/ocr-service/go.mod services/ocr-service/go.sum ./services/ocr-service/
COPY proto proto
COPY gateway gateway
RUN go work sync
WORKDIR /app/gateway
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gateway main.go
RUN chmod +x gateway
EXPOSE 8080
CMD ["./gateway"]