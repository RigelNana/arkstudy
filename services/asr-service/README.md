# ASR Service

ASR（Automatic Speech Recognition）服务，用于处理视频文件的语音识别和转录。

## 功能特性

- 🎥 **视频处理**：接收来自material-service的视频文件
- 🔊 **音轨提取**：使用FFmpeg从视频中提取音轨
- 🎙️ **语音识别**：集成OpenAI Whisper API进行高质量语音转文字
- ⏰ **时间轴分析**：获取每个文本段的精确时间戳
- 🔍 **语义搜索**：将ASR结果存入向量数据库，支持语义搜索
- 📊 **多格式支持**：支持MP4、AVI、MOV、MKV、WebM等视频格式

## API接口

### 1. 处理视频文件
```bash
POST /api/v1/asr/process
Content-Type: application/json

{
    "material_id": "video-001",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "video_url": "https://example.com/video.mp4",
    "language": "zh"  // 可选，语言提示
}
```

### 2. 获取ASR片段
```bash
GET /api/v1/asr/segments/{material_id}
```

### 3. 搜索ASR内容
```bash
POST /api/v1/asr/search
Content-Type: application/json

{
    "query": "深度学习",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "material_id": "video-001",  // 可选
    "top_k": 5,
    "min_score": 0.7
}
```

### 4. 健康检查
```bash
GET /api/v1/health
```

## 环境变量配置

```bash
# 数据库配置
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=arkdb

# 服务配置
ASR_PORT=50057
GRPC_PORT=50057
ASR_HOST=0.0.0.0

# OpenAI配置
OPENAI_API_KEY=your-api-key
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=whisper-1

# FFmpeg配置
FFMPEG_BINARY_PATH=ffmpeg
TEMP_DIR=/tmp/asr
AUDIO_FORMAT=wav

# 存储配置
MAX_FILE_SIZE=104857600  # 100MB
ALLOWED_FORMATS=mp4,avi,mov,mkv,webm
```

## 技术栈

- **语言**: Go 1.24
- **框架**: Gin (HTTP), GORM (ORM)
- **数据库**: PostgreSQL + pgvector
- **语音识别**: OpenAI Whisper API
- **音视频处理**: FFmpeg
- **容器化**: Docker + Kubernetes

## 架构流程

1. **接收请求**: material-service发送视频处理请求
2. **下载视频**: 从提供的URL下载视频文件
3. **音轨提取**: 使用FFmpeg提取音频（WAV格式，16kHz，单声道）
4. **语音识别**: 调用Whisper API进行转录，获取文本和时间戳
5. **数据存储**: 将ASR结果存入PostgreSQL，支持向量搜索
6. **清理资源**: 删除临时文件

## 数据模型

### ASRSegment表结构
```sql
CREATE TABLE asr_segments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    material_id VARCHAR(255) NOT NULL,
    user_id UUID NOT NULL,
    segment_index INTEGER NOT NULL,
    start_time FLOAT NOT NULL,
    end_time FLOAT NOT NULL,
    text TEXT NOT NULL,
    confidence FLOAT,
    embedding FLOAT[],
    language VARCHAR(10),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 部署

### 本地开发
```bash
# 1. 安装依赖
go mod tidy

# 2. 配置环境变量
cp .env.example .env
# 编辑.env文件

# 3. 运行服务
go run main.go
```

### Docker部署
```bash
# 构建镜像
docker build -t asr-service:latest .

# 运行容器
docker run -p 50057:50057 \
  -e DB_HOST=postgres \
  -e OPENAI_API_KEY=your-key \
  asr-service:latest
```

### Kubernetes部署
```bash
# 应用配置
kubectl apply -f k8s/

# 检查状态
kubectl get pods -l app=asr-service
```

## 与其他服务的集成

### material-service集成
material-service可以通过HTTP API调用asr-service：

```go
type ASRRequest struct {
    MaterialID string `json:"material_id"`
    UserID     uuid.UUID `json:"user_id"`
    VideoURL   string `json:"video_url"`
    Language   string `json:"language,omitempty"`
}
```

### llm-service集成
ASR结果可以被llm-service用于：
- 基于视频内容生成题目
- 语义搜索和内容检索
- 知识图谱构建

## 性能优化建议

1. **并发处理**: 支持多个视频同时处理
2. **缓存机制**: 对已处理的视频进行缓存
3. **分块处理**: 对长视频进行分段处理
4. **压缩存储**: 对音频文件进行压缩
5. **批量插入**: 批量插入数据库记录

## 故障排查

### 常见问题

1. **FFmpeg未找到**
   ```bash
   # 安装FFmpeg
   sudo apt-get install ffmpeg  # Ubuntu/Debian
   brew install ffmpeg          # macOS
   ```

2. **Whisper API调用失败**
   - 检查API密钥是否正确
   - 确认网络连接正常
   - 验证音频文件格式

3. **数据库连接失败**
   - 检查数据库连接参数
   - 确认pgvector扩展已安装
   - 验证数据库权限

### 日志级别
- `DEBUG`: 详细的处理流程
- `INFO`: 一般信息和处理结果
- `WARN`: 非致命错误
- `ERROR`: 严重错误和异常

## 贡献指南

1. Fork项目
2. 创建特性分支
3. 提交更改
4. 推送到分支
5. 创建Pull Request

## 许可证

MIT License