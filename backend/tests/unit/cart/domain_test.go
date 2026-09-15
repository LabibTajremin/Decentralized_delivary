package cart

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

func line(id, target string, minor int64, quantity int, options ...domain.Option) domain.Line {
	return domain.Line{
		ID: id, Kind: domain.KindItem, TargetID: target, Name: target,
		UnitPrice: money.Taka(minor), Quantity: quantity, Options: options,
	}
}

func option(group, id string, minor int64) domain.Option {
	return domain.Option{GroupID: group, OptionID: id, Name: id, Price: money.Taka(minor)}
}

func newCart(t *testing.T) domain.Cart {
	t.Helper()
	c, err := domain.NewCart("CRT-1", "USR-1", "MER-1", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("NewCart: %v", err)
	}
	return c
}

func TestNewCartNeedsAnOwnerAndAShop(t *testing.T) {
	if _, err := domain.NewCart("CRT-1", "", "MER-1", time.Unix(0, 0)); !errors.Is(err, domain.ErrNoUser) {
		t.Errorf("a cart with no owner was accepted: %v", err)
	}
	if _, err := domain.NewCart("CRT-1", "USR-1", "", time.Unix(0, 0)); !errors.Is(err, domain.ErrNoMerchant) {
		t.Errorf("a cart with no shop was accepted: %v", err)
	}
}

// The rule the whole module is shaped around. An order is collected by one
// rider from one counter.
func TestACartBelongsToOneShop(t *testing.T) {
	c := newCart(t)
	if err := c.Add("MER-2", line("CLN-1", "ITM-1", 10000, 1)); !errors.Is(err, domain.ErrDifferentMerchant) {
		t.Fatalf("a line from another shop was accepted: %v", err)
	}
	if len(c.Lines) != 0 {
		t.Error("the refused line was added anyway")
	}
}

// Adding the same thing twice bumps the quantity. The customer tapped "add"
// twice; they did not order two separate portions.
func TestAddMergesAnIdenticalSelection(t *testing.T) {
	c := newCart(t)
	large := option("GRP-size", "OPT-large", 5000)

	if err := c.Add("MER-1", line("CLN-1", "ITM-1", 20000, 1, large)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := c.Add("MER-1", line("CLN-2", "ITM-1", 20000, 2, large)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(c.Lines) != 1 || c.Lines[0].Quantity != 3 {
		t.Fatalf("lines = %+v, want one line of three", c.Lines)
	}
	// The original line keeps its id, so a client holding it is not left
	// pointing at nothing.
	if c.Lines[0].ID != "CLN-1" {
		t.Errorf("the merged line is %q, want the original CLN-1", c.Lines[0].ID)
	}
}

// A different choice is a different line, even on the same item.
func TestAddKeepsDifferentSelectionsApart(t *testing.T) {
	c := newCart(t)
	if err := c.Add("MER-1", line("CLN-1", "ITM-1", 20000, 1, option("GRP-size", "OPT-large", 5000))); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := c.Add("MER-1", line("CLN-2", "ITM-1", 20000, 1, option("GRP-size", "OPT-small", 0))); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(c.Lines) != 2 {
		t.Fatalf("lines = %d, want two", len(c.Lines))
	}

	// So is a different note: "no chilli" is part of what was ordered.
	withNote := line("CLN-3", "ITM-1", 20000, 1, option("GRP-size", "OPT-large", 5000))
	withNote.Note = "no chilli"
	if err := c.Add("MER-1", withNote); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(c.Lines) != 3 {
		t.Fatalf("a note did not make a separate line: %d", len(c.Lines))
	}
}

// The option set is compared as a set. The order the customer tapped them in is
// not part of what they ordered.
func TestSameSelectionIgnoresOptionOrder(t *testing.T) {
	a := line("CLN-1", "ITM-1", 100, 1, option("G1", "O1", 0), option("G2", "O2", 0))
	b := line("CLN-2", "ITM-1", 100, 1, option("G2", "O2", 0), option("G1", "O1", 0))
	if !a.SameSelection(b) {
		t.Error("the same two options in a different order read as a different selection")
	}

	different := line("CLN-3", "ITM-1", 100, 1, option("G1", "O1", 0), option("G2", "O3", 0))
	if a.SameSelection(different) {
		t.Error("two different selections read as the same")
	}
	fewer := line("CLN-4", "ITM-1", 100, 1, option("G1", "O1", 0))
	if a.SameSelection(fewer) {
		t.Error("a subset read as the same selection")
	}
	otherKind := line("CLN-5", "ITM-1", 100, 1, option("G1", "O1", 0), option("G2", "O2", 0))
	otherKind.Kind = domain.KindCombo
	if a.SameSelection(otherKind) {
		t.Error("an item and a combo with the same id read as the same selection")
	}
}

func TestAddRejectsImpossibleLines(t *testing.T) {
	c := newCart(t)
	cases := []struct {
		name string
		line domain.Line
		want error
	}{
		{"no target", domain.Line{Kind: domain.KindItem, Quantity: 1}, domain.ErrNoTarget},
		{"no kind", domain.Line{TargetID: "ITM-1", Quantity: 1}, domain.ErrNoTarget},
		{"zero quantity", line("CLN-1", "ITM-1", 100, 0), domain.ErrQuantityRange},
		{"too many", line("CLN-1", "ITM-1", 100, domain.MaxQuantity+1), domain.ErrQuantityRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := c.Add("MER-1", tc.line); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	long := line("CLN-1", "ITM-1", 100, 1)
	long.Note = string(make([]byte, domain.MaxNote+1))
	if err := c.Add("MER-1", long); !errors.Is(err, domain.ErrNoteTooLong) {
		t.Fatalf("a note past the limit was accepted: %v", err)
	}
}

// Merging must respect the per-line cap too, or two adds of twelve would make
// twenty-four.
func TestMergingRespectsTheQuantityCap(t *testing.T) {
	c := newCart(t)
	if err := c.Add("MER-1", line("CLN-1", "ITM-1", 100, 12)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := c.Add("MER-1", line("CLN-2", "ITM-1", 100, 12)); !errors.Is(err, domain.ErrQuantityRange) {
		t.Fatalf("merging past the cap was accepted: %v", err)
	}
	if c.Lines[0].Quantity != 12 {
		t.Errorf("the refused merge changed the quantity to %d", c.Lines[0].Quantity)
	}
}

func TestACartHoldsABoundedNumberOfLines(t *testing.T) {
	c := newCart(t)
	for i := 0; i < domain.MaxLines; i++ {
		n := strconv.Itoa(i)
		if err := c.Add("MER-1", line("CLN-"+n, "ITM-"+n, 100, 1)); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	over := line("CLN-over", "ITM-over", 100, 1)
	if err := c.Add("MER-1", over); !errors.Is(err, domain.ErrTooManyLines) {
		t.Fatalf("a cart past the line limit accepted more: %v", err)
	}
}

func TestSetQuantityAndRemove(t *testing.T) {
	c := newCart(t)
	if err := c.Add("MER-1", line("CLN-1", "ITM-1", 100, 1)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := c.SetQuantity("CLN-1", 5); err != nil {
		t.Fatalf("SetQuantity: %v", err)
	}
	if c.Lines[0].Quantity != 5 {
		t.Errorf("quantity = %d, want 5", c.Lines[0].Quantity)
	}

	// Zero removes it. Tapping minus down to nothing means "take it out".
	if err := c.SetQuantity("CLN-1", 0); err != nil {
		t.Fatalf("SetQuantity(0): %v", err)
	}
	if !c.IsEmpty() {
		t.Error("a quantity of zero left the line in the cart")
	}

	if err := c.SetQuantity("CLN-gone", 1); !errors.Is(err, domain.ErrNoSuchLine) {
		t.Errorf("err = %v, want ErrNoSuchLine", err)
	}
	if err := c.Remove("CLN-gone"); !errors.Is(err, domain.ErrNoSuchLine) {
		t.Errorf("err = %v, want ErrNoSuchLine", err)
	}
	if err := c.SetQuantity("CLN-1", -1); !errors.Is(err, domain.ErrQuantityRange) {
		t.Errorf("err = %v, want ErrQuantityRange", err)
	}
	if err := c.SetQuantity("CLN-1", domain.MaxQuantity+1); !errors.Is(err, domain.ErrQuantityRange) {
		t.Errorf("err = %v, want ErrQuantityRange", err)
	}
}

func TestClearAndCountAndSubtotal(t *testing.T) {
	c := newCart(t)
	if err := c.Add("MER-1", line("CLN-1", "ITM-1", 20000, 2, option("G1", "O1", 5000))); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := c.Add("MER-1", line("CLN-2", "ITM-2", 15000, 1)); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if c.Count() != 3 {
		t.Errorf("Count = %d, want 3", c.Count())
	}
	subtotal := c.Subtotal()
	// (200 + 50) × 2 + 150 = 650 taka.
	if subtotal.Minor() != 65000 {
		t.Errorf("subtotal = %d, want 65000", subtotal.Minor())
	}

	c.Clear()
	if !c.IsEmpty() || c.Count() != 0 {
		t.Error("Clear left something behind")
	}
	if empty := c.Subtotal(); !empty.IsZero() {
		t.Errorf("an empty cart totalled %v", empty)
	}
}

func TestPlaceAtAndHasAddress(t *testing.T) {
	c := newCart(t)
	if c.HasAddress() {
		t.Error("a new cart claimed to have an address")
	}
	c.PlaceAt("ADR-1", 23.7, 90.4)
	if !c.HasAddress() || c.AddressID != "ADR-1" || c.Lat != 23.7 || c.Lng != 90.4 {
		t.Errorf("PlaceAt = %+v", c)
	}
}

func TestLineTotals(t *testing.T) {
	l := line("CLN-1", "ITM-1", 20000, 3, option("G1", "O1", 5000), option("G2", "O2", 2500))
	unit := l.UnitTotal()
	if unit.Minor() != 27500 {
		t.Errorf("unit = %d, want 27500", unit.Minor())
	}
	total := l.Total()
	if total.Minor() != 82500 {
		t.Errorf("total = %d, want 82500", total.Minor())
	}
}
