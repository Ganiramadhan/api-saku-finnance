package ailog

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/platform/storage"
	"github.com/google/uuid"
)

const scanImagePresignTTL = 6 * time.Hour

type Service interface {
	List(ctx context.Context, userID uuid.UUID, feature string, page, limit int) ([]dto.AIProcessingLogResponse, *dto.PaginationMeta, error)
	ListAll(ctx context.Context, page, limit int) ([]dto.AIProcessingLogResponse, *dto.PaginationMeta, error)
	Record(ctx context.Context, userID uuid.UUID, entry RecordInput) error
}

type RecordInput struct {
	Feature           string
	Status            string
	ConfidenceScore   *float64
	ModelVersion      string
	LatencyMs         int
	ExtractedAmount   *float64
	ExtractedMerchant string
	ExtractedCategory string
	ErrorMessage      string
	RawResponse       string
}

type service struct {
	repo    Repository
	storage storage.Storage
}

func NewService(r Repository, store storage.Storage) Service {
	return &service{repo: r, storage: store}
}

func (s *service) toResp(ctx context.Context, l domain.AIProcessingLog) dto.AIProcessingLogResponse {
	var raw map[string]any
	if l.RawResponse != "" {
		_ = json.Unmarshal([]byte(l.RawResponse), &raw)
	}
	resp := dto.AIProcessingLogResponse{
		ID:                l.ID,
		UserID:            l.UserID,
		Feature:           l.Feature,
		Status:            l.Status,
		ExtractedAmount:   l.ExtractedAmount,
		ExtractedMerchant: l.ExtractedMerchant,
		ExtractedCategory: l.ExtractedCategory,
		ConfidenceScore:   l.ConfidenceScore,
		ModelVersion:      l.ModelVersion,
		LatencyMs:         l.LatencyMs,
		ErrorMessage:      l.ErrorMessage,
		RawResponse:       raw,
		CreatedAt:         l.CreatedAt,
		UpdatedAt:         l.UpdatedAt,
	}
	if l.User != nil {
		resp.UserName = l.User.Name
		resp.UserEmail = l.User.Email
	}
	if s.storage != nil && raw != nil {
		if key, ok := raw["image_key"].(string); ok && key != "" {
			if url, err := s.storage.PresignedURL(ctx, key, scanImagePresignTTL); err == nil {
				resp.ImageURL = url
			} else {
				slog.Warn("ailog: presign image url failed", "log_id", l.ID, "error", err)
			}
		}
	}
	return resp
}

func (s *service) List(ctx context.Context, userID uuid.UUID, feature string, page, limit int) ([]dto.AIProcessingLogResponse, *dto.PaginationMeta, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, total, err := s.repo.ListByUser(userID, feature, page, limit)
	if err != nil {
		return nil, nil, err
	}
	out := make([]dto.AIProcessingLogResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, s.toResp(ctx, l))
	}
	return out, dto.NewMeta(page, limit, total), nil
}

func (s *service) ListAll(ctx context.Context, page, limit int) ([]dto.AIProcessingLogResponse, *dto.PaginationMeta, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, total, err := s.repo.ListAll(page, limit)
	if err != nil {
		return nil, nil, err
	}
	out := make([]dto.AIProcessingLogResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, s.toResp(ctx, l))
	}
	return out, dto.NewMeta(page, limit, total), nil
}

func (s *service) Record(_ context.Context, userID uuid.UUID, in RecordInput) error {
	l := domain.AIProcessingLog{
		UserID:            userID,
		Feature:           in.Feature,
		Status:            in.Status,
		ConfidenceScore:   in.ConfidenceScore,
		ModelVersion:      in.ModelVersion,
		LatencyMs:         in.LatencyMs,
		ExtractedAmount:   in.ExtractedAmount,
		ExtractedMerchant: in.ExtractedMerchant,
		ExtractedCategory: in.ExtractedCategory,
		ErrorMessage:      in.ErrorMessage,
		RawResponse:       in.RawResponse,
	}
	return s.repo.Create(&l)
}
