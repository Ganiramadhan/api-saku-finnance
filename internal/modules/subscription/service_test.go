package subscription

import (
	"strings"
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
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

func TestPaymentMethodExpiryDurations(t *testing.T) {
	if got := expiryForPaymentType("qris"); got != 15*time.Minute {
		t.Fatalf("expected QRIS expiry 15 minutes, got %s", got)
	}
	if got := expiryForPaymentType("bank_transfer"); got != 24*time.Hour {
		t.Fatalf("expected VA expiry 24 hours, got %s", got)
	}
	if got := expiryForPaymentType(""); got != 24*time.Hour {
		t.Fatalf("expected unopened Snap expiry 24 hours, got %s", got)
	}
}

func TestPaymentPendingEmailContainsMethodExpiryAndOrder(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Date(2026, 6, 21, 16, 15, 0, 0, jakarta).UTC()
	body := paymentPendingEmailHTML(
		"Gani Ramadhan",
		"Pro",
		349000,
		"IDR",
		"qris",
		"SAKU-PRO-TEST123",
		&expiresAt,
	)
	for _, expected := range []string{
		"Pembayaran Menunggu Penyelesaian",
		"Gani Ramadhan",
		"QRIS",
		"21 Jun 2026 16:15 WIB",
		"SAKU-PRO-TEST123",
		"Rp 349.000",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected email body to contain %q", expected)
		}
	}
}

func TestFormatRupiahGroupsThousands(t *testing.T) {
	cases := map[float64]string{
		1000:    "Rp 1.000",
		349000:  "Rp 349.000",
		1000000: "Rp 1.000.000",
		0:       "Rp 0",
	}
	for amount, want := range cases {
		if got := formatRupiah(amount, "IDR"); got != want {
			t.Fatalf("formatRupiah(%v, IDR) = %q, want %q", amount, got, want)
		}
	}
	if got := formatRupiah(1000, "USD"); got != "1000 USD" {
		t.Fatalf("formatRupiah(1000, USD) = %q, want %q", got, "1000 USD")
	}
}

func TestPaymentSuccessEmailDoesNotDuplicatePeriodInPlanName(t *testing.T) {
	endsAt := time.Date(2027, 8, 18, 0, 0, 0, 0, time.UTC)
	body := paymentSuccessEmailHTML("Gani Ramadhan", "Pro Yearly", 1000, "IDR", "SAKU-PRO_YEARLY-TEST", &endsAt)

	if strings.Contains(body, "Pro Yearly yearly") || strings.Contains(body, "Pro Yearly Yearly") {
		t.Fatalf("expected plan name not to be duplicated with its period, got body containing: %q", body)
	}
	for _, expected := range []string{"Plan: Pro Yearly", "Rp 1.000", "SAKU-PRO_YEARLY-TEST", "18 Aug 2027"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected email body to contain %q, got: %s", expected, body)
		}
	}
}

func TestIsCurrentlyActiveSubscriptionRequiresFutureEndDate(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Second)
	future := now.Add(time.Second)

	if isCurrentlyActiveSubscription(&domain.Subscription{Status: domain.SubscriptionStatusActive, EndsAt: &past}, now) {
		t.Fatal("expected expired active row to be treated as inactive")
	}
	if !isCurrentlyActiveSubscription(&domain.Subscription{Status: domain.SubscriptionStatusActive, EndsAt: &future}, now) {
		t.Fatal("expected future-ended active row to be active")
	}
}

func TestExpireActiveSubscriptionIfNeededMarksOverdueSubscriptionExpired(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Second)
	nextBilling := past
	sub := &domain.Subscription{
		Status:        domain.SubscriptionStatusActive,
		EndsAt:        &past,
		NextBillingAt: &nextBilling,
	}

	if !expireActiveSubscriptionIfNeeded(sub, now) {
		t.Fatal("expected overdue subscription to be expired")
	}
	if sub.Status != domain.SubscriptionStatusExpired {
		t.Fatalf("expected expired status, got %q", sub.Status)
	}
	if sub.NextBillingAt != nil {
		t.Fatal("expected next billing to be cleared")
	}
}
