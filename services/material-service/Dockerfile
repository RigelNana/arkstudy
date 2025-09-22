FROM golang:1.24.7-alpine
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/material-service/go.mod services/material-service/go.sum ./services/material-service/
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY services/user-service/go.mod services/user-service/go.sum ./services/user-service/
COPY services/ocr-service/go.mod services/ocr-service/go.sum ./services/ocr-service/
COPY pkg/metrics/go.mod pkg/metrics/go.sum ./pkg/metrics/
COPY gateway/go.mod gateway/go.sum ./gateway/
COPY proto proto
RUN go work sync && go mod download
COPY services/material-service/ ./services/material-service/
WORKDIR /app/services/material-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o material-service main.go
RUN chmod +x material-service
EXPOSE 50053 8080
CMD ["./material-service"]