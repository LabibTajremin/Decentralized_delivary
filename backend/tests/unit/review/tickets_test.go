package review

import (
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func newRaiseTicket(repo *fakeTicketRepo, ord *fakeOrder) *application.RaiseTicketUseCase {
	return application.NewRaiseTicketUseCase(repo, ord, fakeClock{}, &fakeIDs{})
}

func TestRaiseTicketOpensATicket(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeTicketRepo{}

	tk, err := newRaiseTicket(repo, ord).Execute(ctx(), "ord_1", "usr_1", "item damaged")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if tk.Status != domain.TicketOpen || tk.Subject != "item damaged" {
		t.Errorf("ticket = %+v", tk)
	}
	if len(repo.tickets) != 1 {
		t.Errorf("stored tickets = %d, want 1", len(repo.tickets))
	}
}

func TestRaiseTicketRejectsSomeoneElsesOrder(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeTicketRepo{}

	_, err := newRaiseTicket(repo, ord).Execute(ctx(), "ord_1", "usr_2", "never arrived")
	if errs.CodeOf(err) != "not_your_order" {
		t.Errorf("err = %v", err)
	}
}

func TestRaiseTicketSurfacesANotFoundOrder(t *testing.T) {
	ord := newFakeOrder()
	repo := &fakeTicketRepo{}
	_, err := newRaiseTicket(repo, ord).Execute(ctx(), "ord_missing", "usr_1", "never arrived")
	if errs.CodeOf(err) != "not_found" {
		t.Errorf("err = %v", err)
	}
}

func TestRaiseTicketSurfacesAnOrderStorageFailure(t *testing.T) {
	ord := newFakeOrder()
	ord.err = errBoom
	repo := &fakeTicketRepo{}
	_, err := newRaiseTicket(repo, ord).Execute(ctx(), "ord_1", "usr_1", "never arrived")
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}

func TestRaiseTicketRejectsAnEmptySubject(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeTicketRepo{}
	_, err := newRaiseTicket(repo, ord).Execute(ctx(), "ord_1", "usr_1", "")
	if errs.CodeOf(err) != "invalid_request" {
		t.Errorf("err = %v", err)
	}
}

func TestRaiseTicketSurfacesASaveFailure(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeTicketRepo{saveErr: errBoom}
	_, err := newRaiseTicket(repo, ord).Execute(ctx(), "ord_1", "usr_1", "damaged")
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}

func openTicket(t *testing.T, repo *fakeTicketRepo, id, orderID string) domain.Ticket {
	t.Helper()
	tk, err := domain.NewTicket(id, orderID, "usr_1", "damaged", at)
	if err != nil {
		t.Fatalf("NewTicket: %v", err)
	}
	repo.tickets = append(repo.tickets, tk)
	return tk
}

func TestResolveTicketRefundsThroughPayment(t *testing.T) {
	repo := &fakeTicketRepo{}
	openTicket(t, repo, "tkt_1", "ord_1")
	pay := &fakePayment{}

	tk, err := application.NewResolveTicketUseCase(repo, pay, fakeClock{}).
		Execute(ctx(), "tkt_1", "adm_1", domain.ResolutionRefunded, "policy")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if tk.Status != domain.TicketResolved || tk.Resolution != domain.ResolutionRefunded {
		t.Errorf("ticket = %+v", tk)
	}
	if len(pay.refunds) != 1 || pay.refunds[0].orderID != "ord_1" {
		t.Errorf("refunds = %+v", pay.refunds)
	}
}

func TestResolveTicketRejectingNeedsNoRefund(t *testing.T) {
	repo := &fakeTicketRepo{}
	openTicket(t, repo, "tkt_1", "ord_1")
	pay := &fakePayment{}

	tk, err := application.NewResolveTicketUseCase(repo, pay, fakeClock{}).
		Execute(ctx(), "tkt_1", "adm_1", domain.ResolutionRejected, "not eligible")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if tk.Resolution != domain.ResolutionRejected {
		t.Errorf("ticket = %+v", tk)
	}
	if len(pay.refunds) != 0 {
		t.Errorf("refunds = %+v, want none", pay.refunds)
	}
}

func TestResolveTicketSurfacesANotFoundTicket(t *testing.T) {
	repo := &fakeTicketRepo{}
	pay := &fakePayment{}
	_, err := application.NewResolveTicketUseCase(repo, pay, fakeClock{}).
		Execute(ctx(), "tkt_missing", "adm_1", domain.ResolutionRejected, "")
	if errs.CodeOf(err) != "not_found" {
		t.Errorf("err = %v", err)
	}
}

func TestResolveTicketSurfacesAGetFailure(t *testing.T) {
	repo := &fakeTicketRepo{getErr: errBoom}
	pay := &fakePayment{}
	_, err := application.NewResolveTicketUseCase(repo, pay, fakeClock{}).
		Execute(ctx(), "tkt_1", "adm_1", domain.ResolutionRejected, "")
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}

func TestResolveTicketRejectsAnAlreadyResolvedTicket(t *testing.T) {
	repo := &fakeTicketRepo{}
	openTicket(t, repo, "tkt_1", "ord_1")
	pay := &fakePayment{}
	uc := application.NewResolveTicketUseCase(repo, pay, fakeClock{})
	if _, err := uc.Execute(ctx(), "tkt_1", "adm_1", domain.ResolutionRejected, ""); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	if _, err := uc.Execute(ctx(), "tkt_1", "adm_1", domain.ResolutionRefunded, ""); errs.CodeOf(err) != "ticket_already_resolved" {
		t.Errorf("err = %v", err)
	}
}

func TestResolveTicketRejectsAnInvalidResolution(t *testing.T) {
	repo := &fakeTicketRepo{}
	openTicket(t, repo, "tkt_1", "ord_1")
	pay := &fakePayment{}
	_, err := application.NewResolveTicketUseCase(repo, pay, fakeClock{}).
		Execute(ctx(), "tkt_1", "adm_1", domain.Resolution("maybe"), "")
	if errs.CodeOf(err) != "invalid_resolution" {
		t.Errorf("err = %v", err)
	}
}

func TestResolveTicketSurfacesARefundFailure(t *testing.T) {
	repo := &fakeTicketRepo{}
	openTicket(t, repo, "tkt_1", "ord_1")
	pay := &fakePayment{err: errBoom}
	_, err := application.NewResolveTicketUseCase(repo, pay, fakeClock{}).
		Execute(ctx(), "tkt_1", "adm_1", domain.ResolutionRefunded, "")
	if errs.CodeOf(err) != "refund_failed" {
		t.Errorf("err = %v", err)
	}
}

func TestResolveTicketSurfacesASaveFailure(t *testing.T) {
	repo := &fakeTicketRepo{saveErr: errBoom}
	openTicket(t, repo, "tkt_1", "ord_1")
	pay := &fakePayment{}
	_, err := application.NewResolveTicketUseCase(repo, pay, fakeClock{}).
		Execute(ctx(), "tkt_1", "adm_1", domain.ResolutionRejected, "")
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}

func TestMyTicketsForListsACustomersOwnTickets(t *testing.T) {
	repo := &fakeTicketRepo{}
	openTicket(t, repo, "tkt_1", "ord_1")
	tickets, err := application.NewMyTicketsUseCase(repo).For(ctx(), "usr_1")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if len(tickets) != 1 {
		t.Errorf("tickets = %d, want 1", len(tickets))
	}
}

func TestMyTicketsForSurfacesAStorageFailure(t *testing.T) {
	repo := &fakeTicketRepo{listErr: errBoom}
	_, err := application.NewMyTicketsUseCase(repo).For(ctx(), "usr_1")
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}

func TestOpenTicketsExecuteListsOnlyOpenTickets(t *testing.T) {
	repo := &fakeTicketRepo{}
	openTicket(t, repo, "tkt_1", "ord_1")
	resolved, _ := domain.NewTicket("tkt_2", "ord_2", "usr_1", "damaged", at)
	_ = resolved.Resolve("adm_1", domain.ResolutionRejected, "", at)
	repo.tickets = append(repo.tickets, resolved)

	tickets, err := application.NewOpenTicketsUseCase(repo).Execute(ctx(), 0)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(tickets) != 1 || tickets[0].ID != "tkt_1" {
		t.Errorf("tickets = %+v", tickets)
	}
}

func TestOpenTicketsExecuteClampsAnOversizedLimit(t *testing.T) {
	repo := &fakeTicketRepo{}
	openTicket(t, repo, "tkt_1", "ord_1")
	tickets, err := application.NewOpenTicketsUseCase(repo).Execute(ctx(), 500)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(tickets) != 1 {
		t.Errorf("tickets = %d, want 1", len(tickets))
	}
}

func TestOpenTicketsExecuteSurfacesAStorageFailure(t *testing.T) {
	repo := &fakeTicketRepo{openErr: errBoom}
	_, err := application.NewOpenTicketsUseCase(repo).Execute(ctx(), 0)
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}
