FROM golang:1.24.7-alpine
RUN apk add --no-cache git
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
COPY services/material-service/go.mod services/material-service/go.sum ./services/material-service/
COPY services/ocr-service/go.mod services/ocr-service/go.sum ./services/ocr-service/
COPY services/quiz-service/go.mod services/quiz-service/go.sum ./services/quiz-service/
COPY pkg/metrics/go.mod pkg/metrics/go.sum ./pkg/metrics/
COPY services/asr-service/go.mod services/asr-service/go.sum ./services/asr-service/
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY proto proto
COPY pkg pkg
RUN go work sync && go mod download
COPY services/auth-service/ ./services/auth-service/
WORKDIR /app/services/auth-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o auth-service main.go
RUN chmod +x auth-service
EXPOSE 50051 8080
CMD ["./auth-service"]