package landingchat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
)

func TestAsk_EmptyMessage_ReturnsInvalidInput(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.Ask(context.Background(), dto.LandingChatRequest{Message: "   "})

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestSystemPrompt_StaysScopedToSaku(t *testing.T) {
	if !strings.Contains(systemPrompt, "STRICT SCOPE") {
		t.Fatal("systemPrompt lost its scope-limiting instructions")
	}
	if !strings.Contains(systemPrompt, "NO access to any real account") {
		t.Fatal("systemPrompt lost the no-account-data instruction")
	}
}

func TestStripMarkdown(t *testing.T) {
	in := "Kalau OTP verifikasi belum masuk setelah sign up, coba langkah ini dulu:\n\n" +
		"1. **Cek folder spam/junk** di email kamu — kadang OTP nyasar ke sana\n" +
		"2. Tunggu 1-2 menit, kadang emailnya agak delay\n" +
		"3. Kalau masih belum ada, pencet tombol **\"Resend code\"** (tersedia lagi setelah 60 detik)\n\n" +
		"Kalau sudah dicoba beberapa kali resend tapi tetap nggak masuk, hubungi **Customer Service**."

	got := stripMarkdown(in)

	if strings.Contains(got, "*") {
		t.Fatalf("stripMarkdown left asterisks behind: %q", got)
	}
	for _, want := range []string{
		"1. Cek folder spam/junk di email kamu",
		"2. Tunggu 1-2 menit",
		`3. Kalau masih belum ada, pencet tombol "Resend code"`,
		"hubungi Customer Service.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stripMarkdown output missing %q, got: %q", want, got)
		}
	}
}

func TestStripMarkdown_HeadersLinksAndCode(t *testing.T) {
	in := "# Title\nSee [SAKU](https://saku.app) or run `saku --help`.\n```go\nfmt.Println(\"hi\")\n```\n* bullet one\n* bullet two"

	got := stripMarkdown(in)

	if strings.ContainsAny(got, "#`") || strings.Contains(got, "](") {
		t.Fatalf("stripMarkdown left markdown syntax behind: %q", got)
	}
	if !strings.Contains(got, "See SAKU or run saku --help.") {
		t.Fatalf("stripMarkdown mangled link/code text: %q", got)
	}
	if !strings.Contains(got, "- bullet one") || !strings.Contains(got, "- bullet two") {
		t.Fatalf("stripMarkdown did not normalize bullet markers: %q", got)
	}
}
