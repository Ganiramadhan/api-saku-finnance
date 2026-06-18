package subscription

import (
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/dto"
)

func TestParseMidtransTimeJakarta(t *testing.T) {
	got := parseMidtransTime("2026-06-17 11:49:29")
	if got == nil {
		t.Fatal("expected parsed time")
	}
	want := time.Date(2026, 6, 17, 4, 49, 29, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMidtransExpiryTimePrefersProviderValue(t *testing.T) {
	createdAt := time.Date(2026, 6, 17, 3, 0, 0, 0, time.UTC)
	got := midtransExpiryTime(dto.MidtransWebhook{
		PaymentType: "bank_transfer",
		ExpiryTime:  "2026-06-17 11:49:29",
	}, createdAt)
	if got == nil {
		t.Fatal("expected expiry time")
	}
	want := time.Date(2026, 6, 17, 4, 49, 29, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMidtransExpiryTimeWithoutPaymentMethodIsUnknown(t *testing.T) {
	got := midtransExpiryTime(dto.MidtransWebhook{}, time.Now())
	if got != nil {
		t.Fatalf("expected nil expiry, got %s", got)
	}
}
