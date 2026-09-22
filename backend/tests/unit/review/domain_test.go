package review

import (
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
)

func TestSubjectValid(t *testing.T) {
	for _, s := range []domain.Subject{domain.SubjectMerchant, domain.SubjectPartner, domain.SubjectItem} {
		if !s.Valid() {
			t.Errorf("%s should be valid", s)
		}
	}
	if domain.Subject("courier").Valid() {
		t.Error("an unknown subject reported valid")
	}
}

func TestNewReviewRejectsEachMissingField(t *testing.T) {
	cases := []struct {
		name      string
		orderID   string
		raterID   string
		subject   domain.Subject
		subjectID string
		rating    int
		wantErr   error
	}{
		{"no order", "", "usr", domain.SubjectMerchant, "mer", 5, domain.ErrNoOrder},
		{"no rater", "ord", "", domain.SubjectMerchant, "mer", 5, domain.ErrNoRater},
		{"invalid subject", "ord", "usr", domain.Subject("courier"), "mer", 5, domain.ErrInvalidSubject},
		{"no subject id", "ord", "usr", domain.SubjectMerchant, "", 5, domain.ErrNoSubjectID},
		{"rating too low", "ord", "usr", domain.SubjectMerchant, "mer", 0, domain.ErrInvalidRating},
		{"rating too high", "ord", "usr", domain.SubjectMerchant, "mer", 6, domain.ErrInvalidRating},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := domain.NewReview("rev_1", c.orderID, c.raterID, c.subject, c.subjectID, c.rating, "", at)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("err = %v, want %v", err, c.wantErr)
			}
		})
	}
}

func TestNewReviewBuildsAValidReview(t *testing.T) {
	r, err := domain.NewReview("rev_1", "ord_1", "usr_1", domain.SubjectMerchant, "mer_1", 4, "good", at)
	if err != nil {
		t.Fatalf("NewReview: %v", err)
	}
	if r.ID != "rev_1" || r.OrderID != "ord_1" || r.RaterID != "usr_1" ||
		r.Subject != domain.SubjectMerchant || r.SubjectID != "mer_1" ||
		r.Rating != 4 || r.Comment != "good" || !r.CreatedAt.Equal(at) {
		t.Errorf("review = %+v", r)
	}
}

func TestNewTicketRejectsEachMissingField(t *testing.T) {
	cases := []struct {
		name     string
		orderID  string
		raisedBy string
		subject  string
		wantErr  error
	}{
		{"no order", "", "usr", "damaged", domain.ErrNoTicketOrder},
		{"no raiser", "ord", "", "damaged", domain.ErrNoRaisedBy},
		{"no subject", "ord", "usr", "", domain.ErrNoSubject},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := domain.NewTicket("tkt_1", c.orderID, c.raisedBy, c.subject, at)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("err = %v, want %v", err, c.wantErr)
			}
		})
	}
}

func TestNewTicketBuildsAnOpenTicket(t *testing.T) {
	tk, err := domain.NewTicket("tkt_1", "ord_1", "usr_1", "wrong item", at)
	if err != nil {
		t.Fatalf("NewTicket: %v", err)
	}
	if tk.Status != domain.TicketOpen || tk.OrderID != "ord_1" || tk.RaisedBy != "usr_1" ||
		tk.Subject != "wrong item" || !tk.CreatedAt.Equal(at) {
		t.Errorf("ticket = %+v", tk)
	}
}

func TestResolveRecordsTheAgentsDecision(t *testing.T) {
	tk, _ := domain.NewTicket("tkt_1", "ord_1", "usr_1", "wrong item", at)
	resolvedAt := at.Add(time.Hour)
	if err := tk.Resolve("adm_1", domain.ResolutionRefunded, "refunded per policy", resolvedAt); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if tk.Status != domain.TicketResolved || tk.Resolution != domain.ResolutionRefunded ||
		tk.Note != "refunded per policy" || tk.AgentID != "adm_1" || !tk.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("ticket = %+v", tk)
	}
}

func TestResolveRejectsAnAlreadyResolvedTicket(t *testing.T) {
	tk, _ := domain.NewTicket("tkt_1", "ord_1", "usr_1", "wrong item", at)
	if err := tk.Resolve("adm_1", domain.ResolutionRejected, "", at); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := tk.Resolve("adm_2", domain.ResolutionRefunded, "", at); !errors.Is(err, domain.ErrTicketClosed) {
		t.Errorf("err = %v, want ErrTicketClosed", err)
	}
}

func TestResolveRejectsNoAgent(t *testing.T) {
	tk, _ := domain.NewTicket("tkt_1", "ord_1", "usr_1", "wrong item", at)
	if err := tk.Resolve("", domain.ResolutionRejected, "", at); !errors.Is(err, domain.ErrNoAgent) {
		t.Errorf("err = %v, want ErrNoAgent", err)
	}
}

func TestResolveRejectsAnUnknownResolution(t *testing.T) {
	tk, _ := domain.NewTicket("tkt_1", "ord_1", "usr_1", "wrong item", at)
	if err := tk.Resolve("adm_1", domain.Resolution("maybe"), "", at); !errors.Is(err, domain.ErrNoResolution) {
		t.Errorf("err = %v, want ErrNoResolution", err)
	}
}
