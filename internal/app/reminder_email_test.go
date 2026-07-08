package app

import (
	"strings"
	"testing"
	"time"
)

func TestBillReminderEmailUsesSharedHTMLTemplate(t *testing.T) {
	body := billReminderEmailHTML(
		"Gani Ramadhan",
		"VPS Ganipedia",
		500000,
		"IDR",
		time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC),
	)
	for _, want := range []string{
		"<!doctype html>",
		"Upcoming Bill Reminder",
		"Billing Reminder",
		"Hi Gani Ramadhan,",
		"VPS Ganipedia",
		"Rp 500.000",
		"27 Jun 2026",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected reminder email to contain %q", want)
		}
	}
}

func TestSubscriptionEndedEmailUsesSharedHTMLTemplate(t *testing.T) {
	body := subscriptionReminderEmailHTML(
		"Gani Ramadhan",
		"SAKU Pro",
		time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
		0,
		true,
	)
	for _, want := range []string{"<!doctype html>", "Subscription Ended", "Renewal Needed", "SAKU Pro", "Ended"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected subscription email to contain %q", want)
		}
	}
}
