package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
)

type Service interface {
	List(ctx context.Context, q dto.AuditLogListQuery) ([]dto.AuditLogResponse, *dto.PaginationMeta, error)
	Get(ctx context.Context, id uuid.UUID) (*dto.AuditLogResponse, error)
	ForRecord(ctx context.Context, table, recordID string, page, limit int) ([]dto.AuditLogResponse, *dto.PaginationMeta, error)
}

type service struct{ repo Repository }

func NewService(r Repository) Service { return &service{repo: r} }

func toResp(a domain.AuditLog) dto.AuditLogResponse {
	return dto.AuditLogResponse{
		ID:               a.ID,
		ActorID:          a.ActorID,
		DBUser:           a.DBUser,
		Action:           a.Action,
		Table:            a.Table,
		RecordID:         a.RecordID,
		OldData:          rawJSON(a.OldData),
		NewData:          rawJSON(a.NewData),
		ChangedData:      rawJSON(a.ChangedData),
		IPAddress:        a.IPAddress,
		UserAgent:        a.UserAgent,
		RequestID:        a.RequestID,
		IsDirectDBChange: a.IsDirectDBChange,
		CreatedAt:        a.CreatedAt,
	}
}

func rawJSON(s *string) json.RawMessage {
	if s == nil || *s == "" {
		return nil
	}
	return json.RawMessage(*s)
}

func parseQueryTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func (s *service) List(_ context.Context, q dto.AuditLogListQuery) ([]dto.AuditLogResponse, *dto.PaginationMeta, error) {
	f := ListFilter{
		Table:      q.Table,
		RecordID:   q.RecordID,
		RequestID:  q.RequestID,
		Action:     q.Action,
		DirectOnly: q.DirectOnly,
		From:       parseQueryTime(q.From),
		To:         parseQueryTime(q.To),
		Page:       q.Page,
		Limit:      q.Limit,
	}
	if q.ActorID != "" {
		if id, err := uuid.Parse(q.ActorID); err == nil {
			f.ActorID = &id
		}
	}
	rows, total, err := s.repo.List(f)
	if err != nil {
		return nil, nil, err
	}
	out := make([]dto.AuditLogResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toResp(r))
	}
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return out, dto.NewMeta(page, limit, total), nil
}

func (s *service) Get(_ context.Context, id uuid.UUID) (*dto.AuditLogResponse, error) {
	row, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	r := toResp(*row)
	return &r, nil
}

func (s *service) ForRecord(_ context.Context, table, recordID string, page, limit int) ([]dto.AuditLogResponse, *dto.PaginationMeta, error) {
	rows, total, err := s.repo.ForRecord(table, recordID, page, limit)
	if err != nil {
		return nil, nil, err
	}
	out := make([]dto.AuditLogResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toResp(r))
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return out, dto.NewMeta(page, limit, total), nil
}
