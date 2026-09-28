package cloud

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestMockCaptchaGatewayOnlyIssuesPermitAfterVerification(t *testing.T) {
	gateway, err := NewMockCaptchaGateway("captcha_ticket_01", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("NewMockCaptchaGateway: %v", err)
	}

	permit, err := gateway.Verify(context.Background(), false)
	if !errors.Is(err, ErrCaptchaVerificationRequired) {
		t.Fatalf("Verify(false) error = %v, want ErrCaptchaVerificationRequired", err)
	}
	if _, err := permit.Ticket(); !errors.Is(err, ErrCaptchaVerificationRequired) {
		t.Fatalf("unissued permit Ticket error = %v, want ErrCaptchaVerificationRequired", err)
	}

	permit, err = gateway.Verify(context.Background(), true)
	if err != nil {
		t.Fatalf("Verify(true): %v", err)
	}
	if ticket, err := permit.Ticket(); err != nil || ticket != "captcha_ticket_01" {
		t.Fatalf("issued permit Ticket = %q, %v; want captcha_ticket_01, nil", ticket, err)
	}
}

func TestMockCaptchaGatewayRejectsExpiredVerification(t *testing.T) {
	gateway, err := NewMockCaptchaGateway("captcha_ticket_01", time.Now().Add(-time.Second))
	if err != nil {
		t.Fatalf("NewMockCaptchaGateway: %v", err)
	}
	if _, err := gateway.Verify(context.Background(), true); !errors.Is(err, ErrCaptchaPermitExpired) {
		t.Fatalf("Verify expired captcha error = %v, want ErrCaptchaPermitExpired", err)
	}
}

func TestDevelopmentCaptchaGatewayIssuesUniqueBoundedTickets(t *testing.T) {
	now := time.Now().UTC()
	random := append(bytes.Repeat([]byte{0x11}, 16), bytes.Repeat([]byte{0x22}, 16)...)
	gateway := NewDevelopmentCaptchaGateway(bytes.NewReader(random), func() time.Time { return now })
	first, err := gateway.Verify(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gateway.Verify(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	firstTicket, err := first.Ticket()
	if err != nil {
		t.Fatal(err)
	}
	secondTicket, err := second.Ticket()
	if err != nil {
		t.Fatal(err)
	}
	if firstTicket == secondTicket || !strings.HasPrefix(firstTicket, "dev-captcha-") || len(firstTicket) > 128 || strings.TrimSpace(firstTicket) != firstTicket {
		t.Fatalf("invalid development tickets %q and %q", firstTicket, secondTicket)
	}
}

func TestDevelopmentCaptchaGatewayRequiresVerificationAndRandomness(t *testing.T) {
	gateway := NewDevelopmentCaptchaGateway(bytes.NewReader(bytes.Repeat([]byte{0x33}, 16)), time.Now)
	if _, err := gateway.Verify(context.Background(), false); !errors.Is(err, ErrCaptchaVerificationRequired) {
		t.Fatalf("Verify(false) error = %v", err)
	}
	gateway = NewDevelopmentCaptchaGateway(failingCaptchaRandom{}, time.Now)
	if _, err := gateway.Verify(context.Background(), true); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("random error = %v", err)
	}
}

type failingCaptchaRandom struct{}

func (failingCaptchaRandom) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
