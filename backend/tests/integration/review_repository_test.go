package integration

import (
	"context"
	"testing"
	"time"

	reviewdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	reviewpg "github.com/rootlogic-lab/delivery/backend/internal/modules/review/infrastructure/persistence/postgres"
)

// withReview runs a test inside a transaction that is always rolled back.
// Review does not reference another module's schema — only the two tables
// it owns.
func withReview(t *testing.T, fn func(ctx context.Context, repo *reviewpg.Repository)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	fn(ctx, reviewpg.New(tx))
}

func reviewClock() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func sampleReview(t *testing.T, id, orderID, raterID string, subject reviewdomain.Subject, subjectID string, rating int) reviewdomain.Review {
	t.Helper()
	r, err := reviewdomain.NewReview(id, orderID, raterID, subject, subjectID, rating, "great", reviewClock())
	if err != nil {
		t.Fatalf("NewReview: %v", err)
	}
	return r
}

func TestAReviewRoundTrips(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		saved := sampleReview(t, "rev_1", "ord_1", "usr_1", reviewdomain.SubjectMerchant, "mer_1", 5)
		if err := repo.SaveReview(ctx, saved); err != nil {
			t.Fatalf("SaveReview: %v", err)
		}

		got, err := repo.ReviewsFor(ctx, reviewdomain.SubjectMerchant, "mer_1", 10)
		if err != nil {
			t.Fatalf("ReviewsFor: %v", err)
		}
		if len(got) != 1 || got[0].ID != "rev_1" || got[0].Rating != 5 {
			t.Fatalf("got = %+v", got)
		}
		if !got[0].CreatedAt.Equal(saved.CreatedAt) {
			t.Errorf("CreatedAt = %v, want %v", got[0].CreatedAt, saved.CreatedAt)
		}
	})
}

func TestExistsForRaterFindsAndScopesCorrectly(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		if err := repo.SaveReview(ctx, sampleReview(t, "rev_1", "ord_1", "usr_1", reviewdomain.SubjectMerchant, "mer_1", 5)); err != nil {
			t.Fatalf("SaveReview: %v", err)
		}

		exists, err := repo.ExistsForRater(ctx, "ord_1", "usr_1", reviewdomain.SubjectMerchant, "mer_1")
		if err != nil || !exists {
			t.Fatalf("exists = %v, err = %v, want true", exists, err)
		}

		// A different rater, a different order, or a different subject are
		// all distinct opinions, not the same one already recorded.
		notThisRater, err := repo.ExistsForRater(ctx, "ord_1", "usr_2", reviewdomain.SubjectMerchant, "mer_1")
		if err != nil || notThisRater {
			t.Fatalf("exists = %v for a different rater, want false", notThisRater)
		}
		notThisOrder, err := repo.ExistsForRater(ctx, "ord_2", "usr_1", reviewdomain.SubjectMerchant, "mer_1")
		if err != nil || notThisOrder {
			t.Fatalf("exists = %v for a different order, want false", notThisOrder)
		}
	})
}

func TestRatingForAveragesAcrossReviews(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		if err := repo.SaveReview(ctx, sampleReview(t, "rev_1", "ord_1", "usr_1", reviewdomain.SubjectItem, "itm_1", 5)); err != nil {
			t.Fatalf("SaveReview: %v", err)
		}
		if err := repo.SaveReview(ctx, sampleReview(t, "rev_2", "ord_2", "usr_2", reviewdomain.SubjectItem, "itm_1", 3)); err != nil {
			t.Fatalf("SaveReview: %v", err)
		}

		rating, err := repo.RatingFor(ctx, reviewdomain.SubjectItem, "itm_1")
		if err != nil {
			t.Fatalf("RatingFor: %v", err)
		}
		if rating.Count != 2 || rating.Average != 4 {
			t.Fatalf("rating = %+v, want average 4 across 2 reviews", rating)
		}
	})
}

func TestRatingForASubjectWithNoReviewsIsZero(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		rating, err := repo.RatingFor(ctx, reviewdomain.SubjectMerchant, "mer_missing")
		if err != nil {
			t.Fatalf("RatingFor: %v", err)
		}
		if rating.Count != 0 || rating.Average != 0 {
			t.Fatalf("rating = %+v, want zero value", rating)
		}
	})
}

// ReviewsFor orders newest first.
func TestReviewsForOrdersNewestFirst(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		base := reviewClock()
		for i, id := range []string{"rev_1", "rev_2", "rev_3"} {
			r, err := reviewdomain.NewReview(id, "ord_"+id, "usr_1", reviewdomain.SubjectMerchant, "mer_1", 4,
				"", base.Add(time.Duration(i)*time.Second))
			if err != nil {
				t.Fatalf("NewReview: %v", err)
			}
			if err := repo.SaveReview(ctx, r); err != nil {
				t.Fatalf("SaveReview: %v", err)
			}
		}

		got, err := repo.ReviewsFor(ctx, reviewdomain.SubjectMerchant, "mer_1", 2)
		if err != nil {
			t.Fatalf("ReviewsFor: %v", err)
		}
		if len(got) != 2 || got[0].ID != "rev_3" || got[1].ID != "rev_2" {
			t.Fatalf("got = %+v, want the two newest, newest first", got)
		}
	})
}

func sampleTicket(t *testing.T, id, orderID, raisedBy, subject string) reviewdomain.Ticket {
	t.Helper()
	tk, err := reviewdomain.NewTicket(id, orderID, raisedBy, subject, reviewClock())
	if err != nil {
		t.Fatalf("NewTicket: %v", err)
	}
	return tk
}

func TestATicketRoundTrips(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		saved := sampleTicket(t, "tkt_1", "ord_1", "usr_1", "item damaged")
		if err := repo.SaveTicket(ctx, saved); err != nil {
			t.Fatalf("SaveTicket: %v", err)
		}

		got, found, err := repo.Ticket(ctx, "tkt_1")
		if err != nil || !found {
			t.Fatalf("found = %v, err = %v", found, err)
		}
		if got.OrderID != "ord_1" || got.Status != reviewdomain.TicketOpen {
			t.Fatalf("got = %+v", got)
		}
	})
}

func TestTicketNotFoundInStorage(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		_, found, err := repo.Ticket(ctx, "tkt_missing")
		if err != nil {
			t.Fatalf("Ticket: %v", err)
		}
		if found {
			t.Fatal("found a ticket that was never saved")
		}
	})
}

// A resolution update overwrites the same row rather than inserting a
// second one — the audit trail is the change itself, not a history table.
func TestSavingAResolutionUpdatesTheSameTicket(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		tk := sampleTicket(t, "tkt_1", "ord_1", "usr_1", "never arrived")
		if err := repo.SaveTicket(ctx, tk); err != nil {
			t.Fatalf("SaveTicket: %v", err)
		}
		if err := tk.Resolve("adm_1", reviewdomain.ResolutionRefunded, "policy", reviewClock()); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if err := repo.SaveTicket(ctx, tk); err != nil {
			t.Fatalf("SaveTicket (resolved): %v", err)
		}

		got, found, err := repo.Ticket(ctx, "tkt_1")
		if err != nil || !found {
			t.Fatalf("found = %v, err = %v", found, err)
		}
		if got.Status != reviewdomain.TicketResolved || got.Resolution != reviewdomain.ResolutionRefunded ||
			got.AgentID != "adm_1" || got.ResolvedAt.IsZero() {
			t.Fatalf("got = %+v", got)
		}

		open, err := repo.OpenTickets(ctx, 10)
		if err != nil {
			t.Fatalf("OpenTickets: %v", err)
		}
		for _, o := range open {
			if o.ID == "tkt_1" {
				t.Error("a resolved ticket still appears in the open queue")
			}
		}
	})
}

func TestTicketsRaisedByScopesToOneCustomer(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		if err := repo.SaveTicket(ctx, sampleTicket(t, "tkt_1", "ord_1", "usr_1", "damaged")); err != nil {
			t.Fatalf("SaveTicket: %v", err)
		}
		if err := repo.SaveTicket(ctx, sampleTicket(t, "tkt_2", "ord_2", "usr_2", "damaged")); err != nil {
			t.Fatalf("SaveTicket: %v", err)
		}

		got, err := repo.TicketsRaisedBy(ctx, "usr_1")
		if err != nil {
			t.Fatalf("TicketsRaisedBy: %v", err)
		}
		if len(got) != 1 || got[0].ID != "tkt_1" {
			t.Fatalf("got = %+v", got)
		}
	})
}

func TestOpenTicketsListsOldestFirst(t *testing.T) {
	withReview(t, func(ctx context.Context, repo *reviewpg.Repository) {
		base := reviewClock()
		for i, id := range []string{"tkt_1", "tkt_2"} {
			tk, err := reviewdomain.NewTicket(id, "ord_"+id, "usr_1", "damaged", base.Add(time.Duration(i)*time.Second))
			if err != nil {
				t.Fatalf("NewTicket: %v", err)
			}
			if err := repo.SaveTicket(ctx, tk); err != nil {
				t.Fatalf("SaveTicket: %v", err)
			}
		}

		got, err := repo.OpenTickets(ctx, 10)
		if err != nil {
			t.Fatalf("OpenTickets: %v", err)
		}
		if len(got) != 2 || got[0].ID != "tkt_1" || got[1].ID != "tkt_2" {
			t.Fatalf("got = %+v, want oldest first", got)
		}
	})
}

func TestReviewRepositorySurfacesAClosedConnection(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	if err := conn.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	repo := reviewpg.New(conn)

	if err := repo.SaveReview(ctx, sampleReview(t, "rev_1", "ord_1", "usr_1", reviewdomain.SubjectMerchant, "mer_1", 5)); err == nil {
		t.Error("SaveReview on a closed connection must fail")
	}
	if _, err := repo.ExistsForRater(ctx, "ord_1", "usr_1", reviewdomain.SubjectMerchant, "mer_1"); err == nil {
		t.Error("ExistsForRater on a closed connection must fail")
	}
	if _, err := repo.RatingFor(ctx, reviewdomain.SubjectMerchant, "mer_1"); err == nil {
		t.Error("RatingFor on a closed connection must fail")
	}
	if _, err := repo.ReviewsFor(ctx, reviewdomain.SubjectMerchant, "mer_1", 10); err == nil {
		t.Error("ReviewsFor on a closed connection must fail")
	}
	if err := repo.SaveTicket(ctx, sampleTicket(t, "tkt_1", "ord_1", "usr_1", "damaged")); err == nil {
		t.Error("SaveTicket on a closed connection must fail")
	}
	if _, _, err := repo.Ticket(ctx, "tkt_1"); err == nil {
		t.Error("Ticket on a closed connection must fail")
	}
	if _, err := repo.TicketsRaisedBy(ctx, "usr_1"); err == nil {
		t.Error("TicketsRaisedBy on a closed connection must fail")
	}
	if _, err := repo.OpenTickets(ctx, 10); err == nil {
		t.Error("OpenTickets on a closed connection must fail")
	}
}

func TestNewFromPoolIsWiredForReview(t *testing.T) {
	if repo := reviewpg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}
