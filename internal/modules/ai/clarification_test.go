package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
)

func TestCategorizeClarificationAsksForMissingAmountWithoutDroppingContext(t *testing.T) {
	out := dto.CategorizeResponse{
		MerchantName: "Sumopods",
		Category:     "Tagihan",
		Type:         "expense",
		Transactions: []dto.CategorizeItem{{
			MerchantName: "Sumopods",
			Description:  "VPS Ganipedia",
		}},
	}

	fields, question := categorizeClarification(out, "Bayar tagihan VPS Ganipedia di Sumopods", "id")

	if len(fields) != 1 || fields[0] != "amount" {
		t.Fatalf("expected missing amount, got %#v", fields)
	}
	if !strings.Contains(question, "Sumopods") || !strings.Contains(question, "Nominal") {
		t.Fatalf("expected contextual clarification question, got %q", question)
	}
}

func TestBuildTodayTransactionAnswerUsesExactSharedTransactionData(t *testing.T) {
	rows := []domain.Transaction{
		{Amount: 500000, Type: "expense", MerchantName: "Sumopods"},
		{Amount: 2500000, Type: "income", MerchantName: "Client"},
	}

	answer := buildTodayTransactionAnswer(
		rows,
		2,
		"id",
		time.Date(2026, 6, 20, 0, 0, 0, 0, time.FixedZone("WIB", 7*60*60)),
		false,
	)

	for _, want := range []string{"2 transaksi", "Rp 2.500.000", "Rp 500.000", "Rp 2.000.000", "Sumopods"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("expected answer to contain %q, got %q", want, answer)
		}
	}
}

func TestTodayTransactionIntentIncludesTotalTransactionQuestion(t *testing.T) {
	if !isTodayTransactionQuestion("berapa total transaksi aku hari ini") {
		t.Fatal("expected total transaction question to use deterministic today summary")
	}
}

func TestFilterTransactionsForLocalDayUsesJakartaCalendarDate(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 6, 20, 0, 0, 0, 0, jakarta)
	rows := []domain.Transaction{
		{Amount: 10000, Type: "expense", TransactionDate: time.Date(2026, 6, 19, 17, 0, 0, 0, time.UTC)},
		{Amount: 32000, Type: "expense", TransactionDate: time.Date(2026, 6, 20, 4, 0, 0, 0, time.UTC)},
		{Amount: 25000, Type: "expense", TransactionDate: time.Date(2026, 6, 20, 16, 59, 0, 0, time.UTC)},
		{Amount: 99000, Type: "expense", TransactionDate: time.Date(2026, 6, 20, 17, 0, 0, 0, time.UTC)},
	}

	filtered := filterTransactionsForLocalDay(rows, day)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 Jakarta transactions, got %d", len(filtered))
	}
	_, expense, _ := summarise(filtered)
	if expense != 67000 {
		t.Fatalf("expected Jakarta daily spending 67000, got %.0f", expense)
	}
}

func TestRequestReferenceTimeDefaultsToJakarta(t *testing.T) {
	reference := requestReferenceTime("2026-06-20", "")
	_, offset := reference.Zone()
	if reference.Year() != 2026 || reference.Month() != time.June || reference.Day() != 20 {
		t.Fatalf("unexpected reference date: %s", reference)
	}
	if offset != 7*60*60 {
		t.Fatalf("expected WIB UTC+7 offset, got %d", offset)
	}
}

func TestCategorizeClarificationSkipsCompleteTransaction(t *testing.T) {
	fields, question := categorizeClarification(dto.CategorizeResponse{Amount: 500000}, "Bayar VPS", "id")
	if len(fields) != 0 || question != "" {
		t.Fatalf("expected no clarification, got fields=%#v question=%q", fields, question)
	}
}
