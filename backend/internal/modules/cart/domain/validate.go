package domain

import "github.com/rootlogic-lab/delivery/backend/internal/shared/money"

// Revalidation. A cart is a promise the customer made to themselves an hour
// ago; the shop has been editing its menu since, and the customer may have
// changed the delivery address. Everything in this file exists to say what
// changed, in words, rather than to quietly fix it.
//
// Quietly fixing it is the tempting option and the wrong one. A cart that
// silently repriced itself would show one number on the list and charge another
// at checkout, and a cart that silently dropped an out-of-stock line would have
// the customer arrive at a total they cannot account for.

// Issue is what happened to one line since it was added.
type Issue string

// The things that can happen to a line.
const (
	// IssueNone is a line that is still exactly what the customer chose.
	IssueNone Issue = ""
	// IssuePriceChanged is the shop charging something else now. The line stays
	// orderable — the customer may well still want it — but at the new price,
	// shown.
	IssuePriceChanged Issue = "price_changed"
	// IssueOutOfStock is a tracked item with none left.
	IssueOutOfStock Issue = "out_of_stock"
	// IssueNotAvailableNow is a breakfast item at nine at night.
	IssueNotAvailableNow Issue = "not_available_now"
	// IssueUnavailable is the owner having taken it down.
	IssueUnavailable Issue = "unavailable"
	// IssueRemoved is the item no longer existing at all.
	IssueRemoved Issue = "removed"
)

// Current is what the shop says about one line right now.
type Current struct {
	// Found is whether the item or combo still exists.
	Found bool
	// Orderable is whether a customer may add it right now.
	Orderable bool
	// Reason is catalogue's word for why not: "out_of_stock",
	// "not_available_now", "unavailable", or empty.
	Reason string
	// UnitPrice is what one costs now, options included, computed by the caller
	// against the option groups as they stand.
	UnitPrice money.Money
}

// CheckedLine is one line with the verdict on it.
type CheckedLine struct {
	Line Line
	// Issue is what changed, or IssueNone.
	Issue Issue
	// UnitPrice is the price now — equal to the snapshot unless the price
	// changed, and zero when the line no longer exists.
	UnitPrice money.Money
	// Orderable is whether this line can go into an order as it stands.
	Orderable bool
}

// CheckLine compares one line against what the shop says now.
func CheckLine(line Line, current Current) CheckedLine {
	if !current.Found {
		return CheckedLine{Line: line, Issue: IssueRemoved}
	}
	if !current.Orderable {
		return CheckedLine{Line: line, Issue: issueFor(current.Reason), UnitPrice: current.UnitPrice}
	}

	if line.UnitTotal().Compare(current.UnitPrice) != 0 {
		return CheckedLine{Line: line, Issue: IssuePriceChanged, UnitPrice: current.UnitPrice, Orderable: true}
	}
	return CheckedLine{Line: line, UnitPrice: current.UnitPrice, Orderable: true}
}

// issueFor maps catalogue's reason onto this module's vocabulary.
//
// A translation rather than a shared enum: catalogue's reasons are about an
// item on a shelf and these are about a line in a cart, and the day catalogue
// adds a fourth reason the cart should still compile and say something safe.
func issueFor(reason string) Issue {
	switch reason {
	case "out_of_stock":
		return IssueOutOfStock
	case "not_available_now":
		return IssueNotAvailableNow
	default:
		return IssueUnavailable
	}
}

// Blocker is why a whole cart cannot be ordered.
type Blocker string

// The reasons a cart is not orderable.
const (
	// BlockerNone is a cart ready to check out.
	BlockerNone Blocker = ""
	// BlockerEmpty is nothing in it.
	BlockerEmpty Blocker = "empty"
	// BlockerNoAddress is no delivery point chosen, so nothing can be said
	// about reachability or cost.
	BlockerNoAddress Blocker = "no_address"
	// BlockerShopUnavailable is the shop no longer listed — suspended, or
	// closed for a holiday.
	BlockerShopUnavailable Blocker = "shop_unavailable"
	// BlockerShopClosed is the shop listed but shut for the evening. Different
	// from unavailable because it ends, and the app can say when.
	BlockerShopClosed Blocker = "shop_closed"
	// BlockerOutsideDivision is D3: the address is in another division, so this
	// shop can never deliver to it. Permanent, and the cart has to be abandoned
	// or the address changed back.
	BlockerOutsideDivision Blocker = "outside_division"
	// BlockerBeyondRadius is the address inside the division but past the
	// widest radius the ladder allows.
	BlockerBeyondRadius Blocker = "beyond_radius"
	// BlockerLineProblems is the cart itself being fine but something in it not
	// being orderable.
	BlockerLineProblems Blocker = "line_problems"
)

// Shop is what the merchant module says about the shop right now.
type Shop struct {
	Listed bool
	Open   bool
}

// Reach is what discovery says about the delivery address.
type Reach struct {
	Reachable bool
	// Reason is discovery's word: "outside_division", "beyond_max_radius", or
	// empty.
	Reason string
}

// Check is the whole verdict on a cart.
type Check struct {
	Lines   []CheckedLine
	Blocker Blocker
	// Orderable is whether checkout may proceed. It is the single answer the
	// client branches on: a client that worked out orderability from the line
	// issues would be deciding eligibility, which 2.9 forbids.
	Orderable bool
}

// Decide turns the parts into one verdict.
//
// The order is the order a customer would care about. There is no point telling
// someone that one of their eight lines is out of stock when the shop is in
// another division and none of it can be delivered.
func Decide(cart Cart, shop Shop, reach Reach, lines []CheckedLine) Check {
	check := Check{Lines: lines}

	switch {
	case cart.IsEmpty():
		check.Blocker = BlockerEmpty
	case !shop.Listed:
		check.Blocker = BlockerShopUnavailable
	case !cart.HasAddress():
		check.Blocker = BlockerNoAddress
	case !reach.Reachable:
		check.Blocker = blockerFor(reach.Reason)
	case !shop.Open:
		check.Blocker = BlockerShopClosed
	case !allOrderable(lines):
		check.Blocker = BlockerLineProblems
	default:
		check.Orderable = true
	}
	return check
}

// blockerFor maps discovery's reason onto this module's vocabulary, for the
// same reason issueFor does.
func blockerFor(reason string) Blocker {
	if reason == "outside_division" {
		return BlockerOutsideDivision
	}
	return BlockerBeyondRadius
}

func allOrderable(lines []CheckedLine) bool {
	for _, l := range lines {
		if !l.Orderable {
			return false
		}
	}
	return true
}

// Priced is the subtotal computed from the *current* prices rather than the
// snapshots, which is what the customer is actually about to pay.
//
// Lines that are not orderable contribute nothing: a total that included an
// out-of-stock item would be a number nobody will ever be charged.
func Priced(lines []CheckedLine) money.Money {
	var minor int64
	for _, l := range lines {
		if !l.Orderable {
			continue
		}
		minor += l.UnitPrice.Times(l.Line.Quantity).Minor()
	}
	return money.Taka(minor)
}
