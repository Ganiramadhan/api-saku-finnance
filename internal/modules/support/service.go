package support

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/platform/storage"
	"github.com/google/uuid"
)

const supportAttachmentURLTTL = 24 * time.Hour

type Service interface {
	List(ctx context.Context, userID uuid.UUID, role, status string) ([]dto.SupportTicketResponse, error)
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateSupportTicketRequest) (*dto.SupportTicketResponse, error)
	Reply(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID, req dto.ReplySupportTicketRequest) (*dto.SupportTicketResponse, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, req dto.UpdateSupportTicketStatusRequest) (*dto.SupportTicketResponse, error)
}

type service struct {
	repo    Repository
	storage storage.Storage
}

func NewService(repo Repository, store storage.Storage) Service {
	return &service{repo: repo, storage: store}
}

func (s *service) List(ctx context.Context, userID uuid.UUID, role, status string) ([]dto.SupportTicketResponse, error) {
	var filterUser *uuid.UUID
	if role != "admin" && role != "super_admin" {
		filterUser = &userID
	}
	rows, err := s.repo.List(filterUser, status)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SupportTicketResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.ticketToResp(ctx, row))
	}
	return out, nil
}

func (s *service) Create(ctx context.Context, userID uuid.UUID, req dto.CreateSupportTicketRequest) (*dto.SupportTicketResponse, error) {
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	if strings.TrimSpace(req.Message) == "" && strings.TrimSpace(req.AttachmentKey) == "" {
		return nil, domain.ErrInvalidInput
	}
	ticket := domain.SupportTicket{
		ID:       uuid.New(),
		UserID:   userID,
		Subject:  strings.TrimSpace(req.Subject),
		Category: strings.TrimSpace(req.Category),
		Priority: priority,
		Status:   "open",
	}
	message := domain.SupportMessage{
		ID:     uuid.New(),
		UserID: userID,
		Role:   "user",
		Body:   strings.TrimSpace(req.Message),
	}
	applySupportAttachment(&message, req.AttachmentKey, req.AttachmentName, req.AttachmentType)
	if err := s.repo.Create(&ticket, &message); err != nil {
		return nil, err
	}
	row, err := s.repo.Find(ticket.ID)
	if err != nil {
		return nil, err
	}
	resp := s.ticketToResp(ctx, *row)
	return &resp, nil
}

func (s *service) Reply(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID, req dto.ReplySupportTicketRequest) (*dto.SupportTicketResponse, error) {
	if strings.TrimSpace(req.Message) == "" && strings.TrimSpace(req.AttachmentKey) == "" {
		return nil, domain.ErrInvalidInput
	}
	ticket, err := s.repo.Find(id)
	if err != nil {
		return nil, err
	}
	msgRole := "user"
	status := "open"
	if role == "admin" || role == "super_admin" {
		msgRole = "admin"
		status = "waiting_user"
	} else if ticket.UserID != userID {
		return nil, domain.ErrForbidden
	}
	message := domain.SupportMessage{ID: uuid.New(), TicketID: id, UserID: userID, Role: msgRole, Body: strings.TrimSpace(req.Message)}
	applySupportAttachment(&message, req.AttachmentKey, req.AttachmentName, req.AttachmentType)
	if err := s.repo.AddMessage(&message, status); err != nil {
		return nil, err
	}
	row, err := s.repo.Find(id)
	if err != nil {
		return nil, err
	}
	resp := s.ticketToResp(ctx, *row)
	return &resp, nil
}

func (s *service) UpdateStatus(ctx context.Context, id uuid.UUID, req dto.UpdateSupportTicketStatusRequest) (*dto.SupportTicketResponse, error) {
	if err := s.repo.UpdateStatus(id, req.Status); err != nil {
		return nil, err
	}
	row, err := s.repo.Find(id)
	if err != nil {
		return nil, err
	}
	resp := s.ticketToResp(ctx, *row)
	return &resp, nil
}

func (s *service) ticketToResp(ctx context.Context, t domain.SupportTicket) dto.SupportTicketResponse {
	userName, userEmail := "", ""
	if t.User != nil {
		userName = t.User.Name
		userEmail = t.User.Email
	}
	messages := make([]dto.SupportMessageResponse, 0, len(t.Messages))
	for _, msg := range t.Messages {
		attachmentURL := ""
		if s.storage != nil && msg.AttachmentKey != "" {
			if url, err := s.storage.PresignedURL(ctx, msg.AttachmentKey, supportAttachmentURLTTL); err == nil {
				attachmentURL = url
			}
		}
		messages = append(messages, dto.SupportMessageResponse{
			ID: msg.ID, TicketID: msg.TicketID, UserID: msg.UserID, Role: msg.Role, Body: msg.Body,
			AttachmentKey: msg.AttachmentKey, AttachmentName: msg.AttachmentName, AttachmentType: msg.AttachmentType, AttachmentURL: attachmentURL,
			CreatedAt: msg.CreatedAt,
		})
	}
	return dto.SupportTicketResponse{
		ID: t.ID, UserID: t.UserID, UserName: userName, UserEmail: userEmail, Subject: t.Subject,
		Category: t.Category, Priority: t.Priority, Status: t.Status, Messages: messages,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func applySupportAttachment(message *domain.SupportMessage, key, name, contentType string) {
	message.AttachmentKey = strings.TrimSpace(key)
	message.AttachmentName = strings.TrimSpace(name)
	message.AttachmentType = strings.TrimSpace(contentType)
	if message.AttachmentKey != "" && message.AttachmentName == "" {
		message.AttachmentName = "attachment"
	}
}

func supportAttachmentFolder(userID uuid.UUID) string {
	return fmt.Sprintf("Support/Attachments/%s", userID.String())
}
