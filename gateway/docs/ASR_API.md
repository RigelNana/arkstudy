# Gateway ASR API集成

Gateway现在已经集成了ASR（自动语音识别）服务的API路由。所有ASR相关的请求都需要通过gateway进行认证。

## 集成的ASR API路由

### 1. 处理视频文件（语音识别）
```bash
POST /api/asr/process
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
    "material_id": "video-001",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "video_url": "https://example.com/video.mp4",
    "language": "zh"
}
```

**说明**: 提交视频文件进行语音识别处理，获取转录文本和时间轴信息

### 2. 获取材料的ASR片段
```bash
GET /api/asr/segments/{material_id}
Authorization: Bearer <jwt_token>
```

**说明**: 获取指定材料的所有ASR转录片段，按时间顺序排列

### 3. 搜索ASR内容
```bash
POST /api/asr/search
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
    "query": "深度学习算法",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "material_id": "video-001",
    "top_k": 5,
    "min_score": 0.7
}
```

**说明**: 在用户的ASR内容中进行语义搜索

### 4. ASR服务健康检查
```bash
GET /api/asr/health
Authorization: Bearer <jwt_token>
```

**说明**: 检查ASR服务的健康状态

## 使用示例

### 完整的视频处理流程

1. **用户登录获取JWT**:
```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password"}'
```

2. **上传视频并进行ASR处理**:
```bash
JWT_TOKEN="your-jwt-token"

curl -X POST http://localhost:8080/api/asr/process \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{
    "material_id": "lecture-video-001",
    "user_id": "550e8400-e29b-41d4-a716-446655440000", 
    "video_url": "https://example.com/lecture.mp4",
    "language": "zh"
  }'
```

3. **获取转录结果**:
```bash
curl -X GET http://localhost:8080/api/asr/segments/lecture-video-001 \
  -H "Authorization: Bearer $JWT_TOKEN"
```

4. **搜索视频内容**:
```bash
curl -X POST http://localhost:8080/api/asr/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{
    "query": "机器学习算法原理",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "material_id": "lecture-video-001",
    "top_k": 3
  }'
```

## 与其他服务的协作

### material-service ➡️ gateway ➡️ asr-service
material-service可以通过gateway调用ASR服务处理上传的视频文件：

```go
// material-service中的示例代码
func (s *MaterialService) ProcessVideoASR(materialID, videoURL string, userID uuid.UUID) error {
    asrRequest := ASRRequest{
        MaterialID: materialID,
        UserID:     userID,
        VideoURL:   videoURL,
        Language:   "zh",
    }
    
    // 通过gateway调用ASR服务
    return s.callGatewayASR("/api/asr/process", asrRequest)
}
```

### quiz-service集成
quiz-service可以利用ASR结果生成基于视频内容的题目：

```bash
# 1. 获取视频的ASR转录内容
curl -X GET http://localhost:8080/api/asr/segments/video-001

# 2. 基于ASR内容生成题目
curl -X POST http://localhost:8080/api/quiz/generate \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{
    "material_id": "video-001",
    "question_count": 3,
    "difficulty": "medium", 
    "question_types": ["multiple_choice", "fill_blank"]
  }'
```

## 安全考虑

1. **认证**: 所有ASR API都需要有效的JWT token
2. **授权**: 用户只能访问自己的ASR数据
3. **速率限制**: Gateway可以对ASR请求进行速率限制
4. **文件大小**: ASR服务限制视频文件大小（默认100MB）

## 监控和日志

Gateway会记录所有ASR请求的日志，包括：
- 请求时间和用户ID
- 处理状态和响应时间
- 错误信息和重试次数

通过Prometheus可以监控ASR API的使用情况和性能指标。

## 故障处理

当ASR服务不可用时，Gateway会返回适当的错误响应：

```json
{
    "success": false,
    "message": "ASR service unavailable"
}
```

用户可以稍后重试或联系管理员。