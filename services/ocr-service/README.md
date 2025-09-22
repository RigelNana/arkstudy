# ocr-service

一个专注图像/文档 OCR 的 gRPC 微服务，使用外部 PaddleOCR HTTP API 完成识别；本服务不负责 ASR（语音转写），ASR 有独立服务。

与仓库内其他服务风格一致，支持从 MinIO 或预签名 URL 下载文件，异步执行 OCR 并通过 GetTaskStatus 查询任务状态。后续将与 material-service 通过 Kafka 做事件联动。

## 功能（MVP）
- gRPC API：沿用 `proto/ai/ai.proto`（ProcessOCR / GetTaskStatus；ProcessASR 返回不支持）
- 调用 PaddleOCR HTTP 接口（默认使用 `/predict/ocr_system` 风格）
- MinIO 集成：支持 s3://bucket/object 或 HTTP(S) 直链下载
- 内存任务状态：QUEUED/PROCESSING/COMPLETED/FAILED，便于后续替换 Redis/Kafka

## 配置（环境变量）
- OCR_GRPC_ADDR（默认 50055）
- MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY, MINIO_BUCKET_NAME, MINIO_USE_SSL=false
- PADDLE_OCR_ENDPOINT（例如 http://paddleocr:8868/predict/ocr_system）
- PADDLE_OCR_TIMEOUT（秒，默认 20）

## 运行
```
go build ./services/ocr-service/...
./services/ocr-service/ocr-service
```

或通过 docker-compose 启动整个系统。

