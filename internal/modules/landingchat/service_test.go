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
