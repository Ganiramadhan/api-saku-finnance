package splitbill

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
)

type Service interface {
	List(ctx context.Context, ownerID uuid.UUID) ([]dto.SplitBillResponse, error)
	Get(ctx context.Context, ownerID, id uuid.UUID) (*dto.SplitBillResponse, error)
	Create(ctx context.Context, ownerID uuid.UUID, req dto.CreateSplitBillRequest) (*dto.SplitBillResponse, error)
	Update(ctx context.Context, ownerID, id uuid.UUID, req dto.UpdateSplitBillRequest) (*dto.SplitBillResponse, error)
	Delete(ctx context.Context, ownerID, id uuid.UUID) error
	MarkParticipantPaid(ctx context.Context, ownerID, billID, partID uuid.UUID, paid bool) error
	BuildShare(ctx context.Context, ownerID, id uuid.UUID, phone string) (*dto.SplitBillShareResponse, error)
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func toParticipantResp(p domain.SplitBillParticipant) dto.SplitBillParticipantResponse {
	return dto.SplitBillParticipantResponse{
		ID:     p.ID,
		Name:   p.Name,
		Phone:  p.Phone,
		Amount: p.Amount,
		PaidAt: p.PaidAt,
	}
}

func toBillResp(b domain.SplitBill) dto.SplitBillResponse {
	parts := make([]dto.SplitBillParticipantResponse, 0, len(b.Participants))
	for _, p := range b.Participants {
		parts = append(parts, toParticipantResp(p))
	}
	currency := b.Currency
	if currency == "" {
		currency = "IDR"
	}
	return dto.SplitBillResponse{
		ID:           b.ID,
		OwnerUserID:  b.OwnerUserID,
		Title:        b.Title,
		TotalAmount:  b.TotalAmount,
		Currency:     currency,
		Notes:        b.Notes,
		Participants: parts,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
}

func mapParticipants(items []dto.SplitBillParticipantInput) []domain.SplitBillParticipant {
	out := make([]domain.SplitBillParticipant, 0, len(items))
	for _, it := range items {
		out = append(out, domain.SplitBillParticipant{
			Name:   strings.TrimSpace(it.Name),
			Phone:  strings.TrimSpace(it.Phone),
			Amount: it.Amount,
		})
	}
	return out
}

func (s *service) List(_ context.Context, ownerID uuid.UUID) ([]dto.SplitBillResponse, error) {
	rows, err := s.repo.List(ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SplitBillResponse, 0, len(rows))
	for _, b := range rows {
		out = append(out, toBillResp(b))
	}
	return out, nil
}

func (s *service) Get(_ context.Context, ownerID, id uuid.UUID) (*dto.SplitBillResponse, error) {
	b, err := s.repo.FindByID(ownerID, id)
	if err != nil {
		return nil, err
	}
	resp := toBillResp(*b)
	return &resp, nil
}

func (s *service) Create(_ context.Context, ownerID uuid.UUID, req dto.CreateSplitBillRequest) (*dto.SplitBillResponse, error) {
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "IDR"
	}
	bill := &domain.SplitBill{
		OwnerUserID:  ownerID,
		Title:        strings.TrimSpace(req.Title),
		TotalAmount:  req.TotalAmount,
		Currency:     currency,
		Notes:        strings.TrimSpace(req.Notes),
		Participants: mapParticipants(req.Participants),
	}
	if err := s.repo.Create(bill); err != nil {
		return nil, err
	}
	// Re-read with relations.
	full, err := s.repo.FindByID(ownerID, bill.ID)
	if err != nil {
		return nil, err
	}
	resp := toBillResp(*full)
	return &resp, nil
}

func (s *service) Update(_ context.Context, ownerID, id uuid.UUID, req dto.UpdateSplitBillRequest) (*dto.SplitBillResponse, error) {
	existing, err := s.repo.FindByID(ownerID, id)
	if err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = existing.Currency
	}
	existing.Title = strings.TrimSpace(req.Title)
	existing.TotalAmount = req.TotalAmount
	existing.Currency = currency
	existing.Notes = strings.TrimSpace(req.Notes)

	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceParticipants(existing.ID, mapParticipants(req.Participants)); err != nil {
		return nil, err
	}
	full, err := s.repo.FindByID(ownerID, existing.ID)
	if err != nil {
		return nil, err
	}
	resp := toBillResp(*full)
	return &resp, nil
}

func (s *service) Delete(_ context.Context, ownerID, id uuid.UUID) error {
	return s.repo.Delete(ownerID, id)
}

func (s *service) MarkParticipantPaid(_ context.Context, ownerID, billID, partID uuid.UUID, paid bool) error {
	return s.repo.MarkParticipantPaid(ownerID, billID, partID, paid)
}

func fmtAmount(n float64, ccy string) string {
	if strings.EqualFold(ccy, "IDR") {
		s := fmt.Sprintf("%.0f", n)
		out := []byte{}
		count := 0
		for i := len(s) - 1; i >= 0; i-- {
			if count == 3 {
				out = append([]byte{'.'}, out...)
				count = 0
			}
			out = append([]byte{s[i]}, out...)
			count++
		}
		return "Rp " + string(out)
	}
	return fmt.Sprintf("%s %.2f", ccy, n)
}

var phoneNonDigit = regexp.MustCompile(`[^0-9]`)

func normalizePhone(raw string) string {
	digits := phoneNonDigit.ReplaceAllString(raw, "")
	if strings.HasPrefix(digits, "0") {
		digits = "62" + digits[1:]
	}
	return digits
}

func (s *service) BuildShare(_ context.Context, ownerID, id uuid.UUID, phone string) (*dto.SplitBillShareResponse, error) {
	bill, err := s.repo.FindByID(ownerID, id)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString("*Split Bill: " + bill.Title + "*\n")
	sb.WriteString("Total: " + fmtAmount(bill.TotalAmount, bill.Currency) + "\n")
	if bill.Notes != "" {
		sb.WriteString("Catatan: " + bill.Notes + "\n")
	}
	sb.WriteString("\n*Rincian per orang:*\n")
	for i, p := range bill.Participants {
		status := ""
		if p.PaidAt != nil {
			status = " ✅"
		}
		sb.WriteString(fmt.Sprintf("%d. %s — %s%s\n", i+1, p.Name, fmtAmount(p.Amount, bill.Currency), status))
	}
	sb.WriteString("\nDikirim via SAKU 💸")

	text := sb.String()
	encoded := url.QueryEscape(text)

	waURL := "https://wa.me/?text=" + encoded
	if p := normalizePhone(phone); p != "" {
		waURL = "https://wa.me/" + p + "?text=" + encoded
	}

	return &dto.SplitBillShareResponse{Text: text, WhatsApp: waURL}, nil
}
