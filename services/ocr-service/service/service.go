package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/RigelNana/arkstudy/proto/ai"
	"github.com/RigelNana/arkstudy/services/ocr-service/config"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type OCRService struct {
	ai.UnimplementedAIServiceServer
	cfg         *config.Config
	minio       *minio.Client
	statusStore map[string]*ai.TaskStatusResponse
	// 缓存最终 OCR 结果，键为 task_id
	ocrStore map[string]*ai.OCRResponse
}

func NewOCRService(cfg *config.Config) (*OCRService, error) {
	mc, err := minio.New(cfg.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKeyID, cfg.MinIO.SecretAccessKey, ""),
		Secure: cfg.MinIO.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &OCRService{cfg: cfg, minio: mc, statusStore: map[string]*ai.TaskStatusResponse{}, ocrStore: map[string]*ai.OCRResponse{}}, nil
}

func (s *OCRService) getTask(taskID string) *ai.TaskStatusResponse {
	if t, ok := s.statusStore[taskID]; ok {
		return t
	}
	t := &ai.TaskStatusResponse{TaskId: taskID, Status: ai.TaskStatus_QUEUED, Message: "queued", Progress: 0}
	s.statusStore[taskID] = t
	return t
}

func (s *OCRService) ProcessOCR(ctx context.Context, req *ai.OCRRequest) (*ai.OCRResponse, error) {
	// 允许无 task_id 场景：生成一个新的任务 ID
	if req.TaskId == "" {
		req.TaskId = uuid.NewString()
	}

	// 如果已有完成结果，直接返回缓存
	if res, ok := s.ocrStore[req.TaskId]; ok {
		return res, nil
	}

	// 获取任务状态（可能是 QUEUED/PROCESSING）
	t := s.getTask(req.TaskId)

	// 没有 file_url 时，不重复启动任务，直接返回当前状态
	if strings.TrimSpace(req.FileUrl) == "" {
		return &ai.OCRResponse{TaskId: req.TaskId, Status: t.Status}, nil
	}

	// 若任务尚未开始或处于排队，则启动；若已在处理，直接返回处理中的状态
	if t.Status == ai.TaskStatus_QUEUED || t.Status == ai.TaskStatus_FAILED {
		t.Status = ai.TaskStatus_PROCESSING
		t.Message = "downloading"
		// 异步执行，避免阻塞调用方
		go s.runOCRTask(req, t)
	}

	return &ai.OCRResponse{TaskId: req.TaskId, Status: t.Status}, nil
}

func (s *OCRService) ProcessASR(ctx context.Context, req *ai.ASRRequest) (*ai.ASRResponse, error) {
	// 该服务不负责 ASR，返回未实现/不支持
	t := s.getTask(req.TaskId)
	t.Status = ai.TaskStatus_FAILED
	t.Message = "ASR is handled by dedicated service"
	t.ErrorMessage = "not supported in ocr-service"
	return &ai.ASRResponse{TaskId: req.TaskId, Status: ai.TaskStatus_FAILED, ErrorMessage: "not supported in ocr-service"}, nil
}

func (s *OCRService) GetTaskStatus(ctx context.Context, req *ai.TaskStatusRequest) (*ai.TaskStatusResponse, error) {
	if req.TaskId == "" {
		req.TaskId = uuid.New().String()
	}
	return s.getTask(req.TaskId), nil
}

// runOCRTask 执行下载与 PaddleOCR 推理
func (s *OCRService) runOCRTask(req *ai.OCRRequest, t *ai.TaskStatusResponse) {
	defer func() {
		// 确保进度最大化
		if t.Status == ai.TaskStatus_COMPLETED && t.Progress < 1.0 {
			t.Progress = 1
		}
	}()

	// 1) 下载文件字节
	data, filename, err := s.fetchFile(req.FileUrl)
	if err != nil {
		t.Status = ai.TaskStatus_FAILED
		t.ErrorMessage = fmt.Sprintf("download error: %v", err)
		t.Message = "download failed"
		return
	}
	t.Message = "ocr running"
	t.Progress = 0.3

	// 2) 调用 PaddleOCR HTTP 接口
	txt, boxes, conf, err := s.callPaddleOCR(data, filename)
	if err != nil {
		t.Status = ai.TaskStatus_FAILED
		t.ErrorMessage = fmt.Sprintf("paddleocr error: %v", err)
		t.Message = "ocr failed"
		return
	}

	// 3) 更新任务完成
	t.Status = ai.TaskStatus_COMPLETED
	t.Message = "done"
	t.Progress = 1.0

	// 缓存最终结果
	s.ocrStore[req.TaskId] = &ai.OCRResponse{
		TaskId:     req.TaskId,
		Status:     ai.TaskStatus_COMPLETED,
		Text:       txt,
		Confidence: conf,
		Boxes:      boxes,
	}
}

// fetchFile 支持两种 URL：
// - 预签名 HTTP(S) 直链
// - s3://bucket/object
func (s *OCRService) fetchFile(fileURL string) ([]byte, string, error) {
	if strings.HasPrefix(fileURL, "s3://") {
		u, err := url.Parse(fileURL)
		if err != nil {
			return nil, "", err
		}
		bucket := u.Host
		object := strings.TrimPrefix(u.Path, "/")
		if bucket == "" {
			bucket = s.cfg.MinIO.BucketName
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		obj, err := s.minio.GetObject(ctx, bucket, object, minio.GetObjectOptions{})
		if err != nil {
			return nil, "", err
		}
		defer obj.Close()
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, obj); err != nil {
			return nil, "", err
		}
		filename := path.Base(object)
		return buf.Bytes(), filename, nil
	}

	// 直接 HTTP 下载
	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Get(fileURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return nil, "", err
	}
	// 从 URL 推断文件名
	u, _ := url.Parse(fileURL)
	filename := path.Base(u.Path)
	if filename == "" {
		filename = "image"
	}
	return buf.Bytes(), filename, nil
}

// PaddleOCR 返回结构（以 ocr_system 为例）
// 形如：[{"res": [[ [x1,y1],[x2,y2],[x3,y3],[x4,y4] ], (text, score) ], ...}]
// 不同部署会有差异，提供一个常见解析路径，同时容错。
type paddleOCRItem struct {
	Res [][]interface{} `json:"res"`
}

func (s *OCRService) callPaddleOCR(data []byte, filename string) (text string, boxes []*ai.BoundingBox, avgConfidence float32, err error) {
	if s.cfg.Paddle.Endpoint == "" {
		return "", nil, 0, fmt.Errorf("PaddleOCR endpoint not configured")
	}

	// 构建 multipart/form-data
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fw, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return "", nil, 0, err
	}
	if _, err := fw.Write(data); err != nil {
		return "", nil, 0, err
	}
	// 某些实现需要 extra params, 允许 options 透传，后续扩展
	writer.Close()

	httpClient := &http.Client{Timeout: time.Duration(s.cfg.Paddle.TimeoutSecond) * time.Second}
	req, err := http.NewRequest("POST", s.cfg.Paddle.Endpoint, &body)
	if err != nil {
		return "", nil, 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", nil, 0, fmt.Errorf("paddle http %d: %s", resp.StatusCode, string(b))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, 0, err
	}

	// 尝试解析为通用数组
	var arr []map[string]interface{}
	if err := json.Unmarshal(raw, &arr); err != nil {
		// 有些实现直接返回对象
		var obj map[string]interface{}
		if err2 := json.Unmarshal(raw, &obj); err2 != nil {
			return "", nil, 0, fmt.Errorf("decode resp: %v; raw=%s", err, string(raw))
		}
		arr = []map[string]interface{}{obj}
	}

	// 提取文字、置信度和框
	var texts []string
	var sumConf float64
	var count int
	var outBoxes []*ai.BoundingBox

	for _, it := range arr {
		// 常见字段命名："res" 或 "data"
		var res interface{}
		if v, ok := it["res"]; ok {
			res = v
		} else if v, ok := it["data"]; ok {
			res = v
		}
		list, _ := res.([]interface{})
		for _, item := range list {
			// item 通常包含 [ polygon_points, [text, score] ]
			one, ok := item.([]interface{})
			if !ok || len(one) == 0 {
				continue
			}
			// 文字与分数
			var oText string
			var oScore float64
			// 坐标转为 bbox（粗略取最小包围矩形）
			var bb *ai.BoundingBox

			for _, sub := range one {
				switch vv := sub.(type) {
				case string:
					oText = vv
				case float64:
					// 某些实现把 score 单独给出
					if vv >= 0 && vv <= 1 {
						oScore = vv
					}
				case []interface{}:
					// 可能是 [[x,y],[x,y]..]
					xs := []float64{}
					ys := []float64{}
					for _, p := range vv {
						pp, ok := p.([]interface{})
						if !ok || len(pp) < 2 {
							continue
						}
						x, _ := pp[0].(float64)
						y, _ := pp[1].(float64)
						xs = append(xs, x)
						ys = append(ys, y)
					}
					if len(xs) >= 1 && len(ys) >= 1 {
						minX, maxX := xs[0], xs[0]
						minY, maxY := ys[0], ys[0]
						for i := 1; i < len(xs); i++ {
							if xs[i] < minX {
								minX = xs[i]
							}
							if xs[i] > maxX {
								maxX = xs[i]
							}
						}
						for i := 1; i < len(ys); i++ {
							if ys[i] < minY {
								minY = ys[i]
							}
							if ys[i] > maxY {
								maxY = ys[i]
							}
						}
						bb = &ai.BoundingBox{X: float32(minX), Y: float32(minY), Width: float32(maxX - minX), Height: float32(maxY - minY)}
					}
				}
			}
			if oText != "" {
				texts = append(texts, oText)
			}
			if bb != nil {
				bb.Text = oText
				bb.Confidence = float32(oScore)
				outBoxes = append(outBoxes, bb)
			}
			if oScore > 0 {
				sumConf += oScore
				count++
			}
		}
	}

	var avg float32
	if count > 0 {
		avg = float32(sumConf / float64(count))
	}
	return strings.Join(texts, "\n"), outBoxes, avg, nil
}
