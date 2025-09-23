package grpc

import (
	"context"
	"fmt"
	"log"

	"github.com/RigelNana/arkstudy/proto/asr"
	"github.com/RigelNana/arkstudy/services/asr-service/models"
	"github.com/RigelNana/arkstudy/services/asr-service/service"
	"github.com/google/uuid"
)

// ASRServer implements the ASR gRPC service
type ASRServer struct {
	asr.UnimplementedASRServiceServer
	asrService *service.ASRService
}

// NewASRServer creates a new ASR gRPC server
func NewASRServer(asrService *service.ASRService) *ASRServer {
	return &ASRServer{
		asrService: asrService,
	}
}

// ProcessVideo handles video processing for ASR
func (s *ASRServer) ProcessVideo(ctx context.Context, req *asr.ProcessVideoRequest) (*asr.ProcessVideoResponse, error) {
	log.Printf("Processing video for material ID: %d", req.MaterialId)

	// Create ASR request
	asrReq := &models.ASRRequest{
		MaterialID: fmt.Sprintf("%d", req.MaterialId),
		VideoURL:   req.VideoUrl,
		// Note: UserID is not provided in the proto, need to add it or use a default
		UserID: uuid.New(), // Using a new UUID for now
	}

	response, err := s.asrService.ProcessVideo(asrReq)
	if err != nil {
		log.Printf("Error processing video: %v", err)
		return &asr.ProcessVideoResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Convert segments to proto format
	protoSegments := make([]*asr.ASRSegment, len(response.Segments))
	for i, segment := range response.Segments {
		confidence := float32(0.0)
		if segment.Confidence != nil {
			confidence = float32(*segment.Confidence)
		}

		protoSegments[i] = &asr.ASRSegment{
			Id:               uint64(i + 1), // Use index as ID since UUID can't convert to uint64
			MaterialId:       req.MaterialId,
			StartTime:        float32(segment.StartTime),
			EndTime:          float32(segment.EndTime),
			Text:             segment.Text,
			Confidence:       confidence,
			EmbeddingVector:  "", // Convert from pq.Float64Array if needed
			CreatedAt:        segment.CreatedAt.String(),
			UpdatedAt:        segment.UpdatedAt.String(),
		}
	}

	return &asr.ProcessVideoResponse{
		Success:  response.Success,
		Message:  response.Message,
		Segments: protoSegments,
	}, nil
}

// GetSegments retrieves ASR segments for a material
func (s *ASRServer) GetSegments(ctx context.Context, req *asr.GetSegmentsRequest) (*asr.GetSegmentsResponse, error) {
	log.Printf("Getting segments for material ID: %d", req.MaterialId)

	materialIDStr := fmt.Sprintf("%d", req.MaterialId)
	segments, err := s.asrService.GetSegmentsByMaterialID(materialIDStr)
	if err != nil {
		log.Printf("Error getting segments: %v", err)
		return &asr.GetSegmentsResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Convert segments to proto format
	protoSegments := make([]*asr.ASRSegment, len(segments))
	for i, segment := range segments {
		confidence := float32(0.0)
		if segment.Confidence != nil {
			confidence = float32(*segment.Confidence)
		}

		protoSegments[i] = &asr.ASRSegment{
			Id:               uint64(i + 1), // Use index as ID since UUID can't convert to uint64
			MaterialId:       req.MaterialId,
			StartTime:        float32(segment.StartTime),
			EndTime:          float32(segment.EndTime),
			Text:             segment.Text,
			Confidence:       confidence,
			EmbeddingVector:  "", // Convert from pq.Float64Array if needed
			CreatedAt:        segment.CreatedAt.String(),
			UpdatedAt:        segment.UpdatedAt.String(),
		}
	}

	return &asr.GetSegmentsResponse{
		Success:  true,
		Message:  "Segments retrieved successfully",
		Segments: protoSegments,
	}, nil
}

// SearchSegments searches ASR segments
func (s *ASRServer) SearchSegments(ctx context.Context, req *asr.SearchSegmentsRequest) (*asr.SearchSegmentsResponse, error) {
	log.Printf("Searching segments with query: %s", req.Query)

	// Create search request
	searchReq := &models.SearchASRRequest{
		Query:  req.Query,
		UserID: uuid.New(), // Using a new UUID for now, should come from context
		TopK:   10,         // Default value
	}

	if req.MaterialId > 0 {
		searchReq.MaterialID = fmt.Sprintf("%d", req.MaterialId)
	}

	if req.Limit > 0 {
		searchReq.TopK = int(req.Limit)
	}

	response, err := s.asrService.SearchSegments(searchReq)
	if err != nil {
		log.Printf("Error searching segments: %v", err)
		return &asr.SearchSegmentsResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Convert segments to proto format
	protoSegments := make([]*asr.ASRSegment, len(response.Results))
	for i, result := range response.Results {
		segment := result.Segment
		confidence := float32(0.0)
		if segment.Confidence != nil {
			confidence = float32(*segment.Confidence)
		}

		protoSegments[i] = &asr.ASRSegment{
			Id:               uint64(i + 1), // Use index as ID since UUID can't convert to uint64
			MaterialId:       req.MaterialId,
			StartTime:        float32(segment.StartTime),
			EndTime:          float32(segment.EndTime),
			Text:             segment.Text,
			Confidence:       confidence,
			EmbeddingVector:  "", // Convert from pq.Float64Array if needed
			CreatedAt:        segment.CreatedAt.String(),
			UpdatedAt:        segment.UpdatedAt.String(),
		}
	}

	return &asr.SearchSegmentsResponse{
		Success:  response.Success,
		Message:  response.Message,
		Segments: protoSegments,
	}, nil
}

// HealthCheck provides health status
func (s *ASRServer) HealthCheck(ctx context.Context, req *asr.HealthCheckRequest) (*asr.HealthCheckResponse, error) {
	return &asr.HealthCheckResponse{
		Status:  "healthy",
		Message: "ASR service is running",
	}, nil
}