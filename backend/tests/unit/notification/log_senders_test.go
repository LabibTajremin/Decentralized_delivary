package notification

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	pushlog "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/infrastructure/push/log"
	smslog "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/infrastructure/sms/log"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestPushLogSenderRefusesProduction(t *testing.T) {
	if _, err := pushlog.New(discardLogger(), true); !errors.Is(err, pushlog.ErrNotForProduction) {
		t.Errorf("err = %v", err)
	}
}

func TestPushLogSenderSendsOutsideProduction(t *testing.T) {
	s, err := pushlog.New(discardLogger(), false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Send(context.Background(), "tok_1", "Title", "Body"); err != nil {
		t.Errorf("Send: %v", err)
	}
}

func TestSMSLogSenderRefusesProduction(t *testing.T) {
	if _, err := smslog.New(discardLogger(), true); !errors.Is(err, smslog.ErrNotForProduction) {
		t.Errorf("err = %v", err)
	}
}

func TestSMSLogSenderSendsOutsideProduction(t *testing.T) {
	s, err := smslog.New(discardLogger(), false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Send(context.Background(), "+8801700000000", "Body"); err != nil {
		t.Errorf("Send: %v", err)
	}
}
