FROM golang:1.24.7-alpine
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/quiz-service/go.mod services/quiz-service/go.sum ./services/quiz-service/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
COPY services/material-service/go.mod services/material-service/go.sum ./services/material-service/
COPY services/ocr-service/go.mod services/ocr-service/go.sum ./services/ocr-service/
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY proto/go.mod proto/go.sum ./proto/
COPY pkg/metrics/go.mod pkg/metrics/go.sum ./pkg/metrics/
COPY services/asr-service/go.mod services/asr-service/go.sum ./services/asr-service/
COPY proto proto
COPY pkg/ ./pkg/
RUN go work sync && go mod download
COPY services/quiz-service/ ./services/quiz-service/
WORKDIR /app/services/quiz-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o quiz-service main.go
RUN chmod +x quiz-service
EXPOSE 50056
CMD ["./quiz-service"]