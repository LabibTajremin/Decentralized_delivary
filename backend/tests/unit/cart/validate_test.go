package cart

import (
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The revalidation rules. Everything here is about *saying* what changed rather
// than fixing it — a cart that silently repriced itself would show one number
// on the list and charge another at checkout.

func TestCheckLineUnchanged(t *testing.T) {
	l := line("CLN-1", "ITM-1", 20000, 2, option("G1", "O1", 5000))
	got := domain.CheckLine(l, domain.Current{Found: true, Orderable: true, UnitPrice: money.Taka(25000)})
	if got.Issue != domain.IssueNone || !got.Orderable || got.UnitPrice.Minor() != 25000 {
		t.Fatalf("an unchanged line was reported as changed: %+v", got)
	}
}

// The price moved. The line stays orderable — the customer may well still want
// it — but at the new price, shown.
func TestCheckLinePriceChanged(t *testing.T) {
	l := line("CLN-1", "ITM-1", 20000, 1)
	got := domain.CheckLine(l, domain.Current{Found: true, Orderable: true, UnitPrice: money.Taka(23000)})
	if got.Issue != domain.IssuePriceChanged || !got.Orderable {
		t.Fatalf("a price change was not reported: %+v", got)
	}
	if got.UnitPrice.Minor() != 23000 {
		t.Errorf("the reported price is %d, want the new 23000", got.UnitPrice.Minor())
	}
}

func TestCheckLineRemoved(t *testing.T) {
	got := domain.CheckLine(line("CLN-1", "ITM-1", 20000, 1), domain.Current{})
	if got.Issue != domain.IssueRemoved || got.Orderable {
		t.Fatalf("a deleted item was not reported as removed: %+v", got)
	}
}

// Catalogue's reasons, translated into this module's vocabulary. The default
// case matters: the day catalogue adds a fourth reason, a cart should still
// compile and say something safe.
func TestCheckLineUnavailableReasons(t *testing.T) {
	cases := map[string]domain.Issue{
		"out_of_stock":      domain.IssueOutOfStock,
		"not_available_now": domain.IssueNotAvailableNow,
		"unavailable":       domain.IssueUnavailable,
		"":                  domain.IssueUnavailable,
		"something_new":     domain.IssueUnavailable,
	}
	for reason, want := range cases {
		got := domain.CheckLine(line("CLN-1", "ITM-1", 20000, 1),
			domain.Current{Found: true, Reason: reason, UnitPrice: money.Taka(20000)})
		if got.Issue != want {
			t.Errorf("reason %q gave issue %q, want %q", reason, got.Issue, want)
		}
		if got.Orderable {
			t.Errorf("reason %q left the line orderable", reason)
		}
	}
}

func cartWith(t *testing.T, lines ...domain.Line) domain.Cart {
	t.Helper()
	c, err := domain.NewCart("CRT-1", "USR-1", "MER-1", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("NewCart: %v", err)
	}
	for _, l := range lines {
		if err := c.Add("MER-1", l); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	return c
}

func okLine() domain.CheckedLine {
	return domain.CheckedLine{
		Line: line("CLN-1", "ITM-1", 20000, 2), UnitPrice: money.Taka(20000), Orderable: true,
	}
}

// The order of the blockers is the order a customer cares about. There is no
// point reporting an out-of-stock line when the shop is in another division and
// none of it can be delivered.
func TestDecideReportsTheBlockerThatMattersMost(t *testing.T) {
	placed := cartWith(t, line("CLN-1", "ITM-1", 20000, 2))
	placed.PlaceAt("ADR-1", 23.7, 90.4)

	open := domain.Shop{Listed: true, Open: true}
	reachable := domain.Reach{Reachable: true}
	broken := domain.CheckedLine{Line: line("CLN-1", "ITM-1", 20000, 1), Issue: domain.IssueOutOfStock}

	cases := []struct {
		name  string
		cart  domain.Cart
		shop  domain.Shop
		reach domain.Reach
		lines []domain.CheckedLine
		want  domain.Blocker
	}{
		{"empty", cartWith(t), open, reachable, nil, domain.BlockerEmpty},
		{"shop suspended", placed, domain.Shop{}, reachable, []domain.CheckedLine{okLine()}, domain.BlockerShopUnavailable},
		{"no address", cartWith(t, line("CLN-1", "ITM-1", 20000, 2)), open, reachable, []domain.CheckedLine{okLine()}, domain.BlockerNoAddress},
		{"another division", placed, open, domain.Reach{Reason: "outside_division"}, []domain.CheckedLine{okLine()}, domain.BlockerOutsideDivision},
		{"too far", placed, open, domain.Reach{Reason: "beyond_max_radius"}, []domain.CheckedLine{okLine()}, domain.BlockerBeyondRadius},
		{"shop closed", placed, domain.Shop{Listed: true}, reachable, []domain.CheckedLine{okLine()}, domain.BlockerShopClosed},
		{"a line is not orderable", placed, open, reachable, []domain.CheckedLine{okLine(), broken}, domain.BlockerLineProblems},
		{"ready", placed, open, reachable, []domain.CheckedLine{okLine()}, domain.BlockerNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.Decide(tc.cart, tc.shop, tc.reach, tc.lines)
			if got.Blocker != tc.want {
				t.Errorf("blocker = %q, want %q", got.Blocker, tc.want)
			}
			if got.Orderable != (tc.want == domain.BlockerNone) {
				t.Errorf("Orderable = %v with blocker %q", got.Orderable, got.Blocker)
			}
		})
	}
}

// D3 beats everything except an empty cart and a suspended shop. A shop that
// is closed for the evening but in another division should report the division,
// because that one is permanent.
func TestDivisionOutranksClosedForTheEvening(t *testing.T) {
	placed := cartWith(t, line("CLN-1", "ITM-1", 20000, 1))
	placed.PlaceAt("ADR-1", 23.7, 90.4)

	got := domain.Decide(placed,
		domain.Shop{Listed: true, Open: false},
		domain.Reach{Reason: "outside_division"},
		[]domain.CheckedLine{okLine()})
	if got.Blocker != domain.BlockerOutsideDivision {
		t.Fatalf("blocker = %q, want the permanent one", got.Blocker)
	}
}

// The total is what the customer is about to pay: current prices, and nothing
// for the lines that cannot be ordered.
func TestPricedUsesCurrentPricesAndSkipsWhatCannotBeOrdered(t *testing.T) {
	lines := []domain.CheckedLine{
		{Line: line("CLN-1", "ITM-1", 20000, 2), UnitPrice: money.Taka(23000), Orderable: true},
		{Line: line("CLN-2", "ITM-2", 10000, 3), UnitPrice: money.Taka(10000), Issue: domain.IssueOutOfStock},
	}
	total := domain.Priced(lines)
	// 230 × 2, and nothing for the out-of-stock line.
	if total.Minor() != 46000 {
		t.Errorf("total = %d, want 46000", total.Minor())
	}

	if empty := domain.Priced(nil); !empty.IsZero() {
		t.Errorf("an empty check totalled %v", empty)
	}
}
