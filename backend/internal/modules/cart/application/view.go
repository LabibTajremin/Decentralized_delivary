package application

import (
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/pricing"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Everything a cart screen renders, already decided. There is nothing here for
// the client to total, compare or interpret (2.9) — including the sentences
// explaining why a line or a whole cart cannot be ordered, which are composed
// here so every client says the same thing and says it in Bengali (1.4).

// Money is an amount in both forms.
type Money struct {
	Minor    int64
	Currency string
	Display  string
}

func toMoney(m money.Money) Money {
	return Money{Minor: m.Minor(), Currency: string(m.Currency()), Display: m.Display()}
}

// Option is one choice on a line.
type Option struct {
	GroupID  string
	OptionID string
	Name     string
	Price    Money
}

// LineView is one line as the cart screen shows it.
type LineView struct {
	ID        string
	Kind      string
	TargetID  string
	Name      string
	Options   []Option
	Quantity  int
	Note      string
	UnitPrice Money
	LineTotal Money
	// Issue is what changed since it was added, as a code the client may
	// branch a badge on, and IssueText is the sentence to show.
	Issue     string
	IssueText string
	Orderable bool
}

// View is a whole cart, ready to render.
type View struct {
	ID         string
	MerchantID string
	// The shop's own details, so a cart screen needs no second request.
	MerchantName    string
	MerchantLogoURL string
	MerchantStatus  string
	AddressID       string
	// Lat and Lng are the delivery point the cart is held against. Carried on
	// the view because order freezes them onto the order, and because
	// reachability was decided against these coordinates rather than against
	// whatever the address book says by the time checkout runs.
	Lat   float64
	Lng   float64
	Lines []LineView
	// Count is how many individual things are in it — the badge on the icon.
	Count int
	// Subtotal is the goods at their current prices. Delivery and the grand
	// total are pricing's (P10).
	Subtotal Money
	// Orderable is the single answer checkout branches on, and Blocker with
	// BlockerText says why not.
	Orderable   bool
	Blocker     string
	BlockerText string
	// Pricing is the receipt — delivery, any surcharge, the total. Absent when
	// there is nothing true to quote: no address yet, or an address this shop
	// cannot deliver to. A fee for a journey that cannot happen is a number the
	// customer would reasonably take for a promise.
	Pricing *pricing.Quote
}

// viewOf assembles the rendered cart.
func viewOf(cart domain.Cart, shop merchant.Merchant, check domain.Check, lang string) View {
	lines := make([]LineView, 0, len(check.Lines))
	for _, checked := range check.Lines {
		lines = append(lines, lineViewOf(checked, lang))
	}

	return View{
		ID:              cart.ID,
		MerchantID:      cart.MerchantID,
		MerchantName:    shop.Name,
		MerchantLogoURL: shop.LogoURL,
		MerchantStatus:  shop.OpenStatus,
		AddressID:       cart.AddressID,
		Lat:             cart.Lat,
		Lng:             cart.Lng,
		Lines:           lines,
		Count:           cart.Count(),
		Subtotal:        toMoney(domain.Priced(check.Lines)),
		Orderable:       check.Orderable,
		Blocker:         string(check.Blocker),
		BlockerText:     blockerText(check.Blocker, shop, lang),
	}
}

func lineViewOf(checked domain.CheckedLine, lang string) LineView {
	options := make([]Option, 0, len(checked.Line.Options))
	for _, o := range checked.Line.Options {
		options = append(options, Option{
			GroupID: o.GroupID, OptionID: o.OptionID, Name: o.Name, Price: toMoney(o.Price),
		})
	}
	return LineView{
		ID:        checked.Line.ID,
		Kind:      string(checked.Line.Kind),
		TargetID:  checked.Line.TargetID,
		Name:      checked.Line.Name,
		Options:   options,
		Quantity:  checked.Line.Quantity,
		Note:      checked.Line.Note,
		UnitPrice: toMoney(checked.UnitPrice),
		LineTotal: toMoney(checked.UnitPrice.Times(checked.Line.Quantity)),
		Issue:     string(checked.Issue),
		IssueText: issueText(checked.Issue, lang),
		Orderable: checked.Orderable,
	}
}

// issueText is the line's badge, in words.
func issueText(issue domain.Issue, lang string) string {
	bengali := lang != "en"
	switch issue {
	case domain.IssuePriceChanged:
		if bengali {
			return "দাম পরিবর্তন হয়েছে"
		}
		return "The price has changed"
	case domain.IssueOutOfStock:
		if bengali {
			return "স্টকে নেই"
		}
		return "Out of stock"
	case domain.IssueNotAvailableNow:
		if bengali {
			return "এখন পাওয়া যাচ্ছে না"
		}
		return "Not available at this time"
	case domain.IssueUnavailable:
		if bengali {
			return "এখন আর পাওয়া যাচ্ছে না"
		}
		return "No longer available"
	case domain.IssueRemoved:
		if bengali {
			return "দোকান এটি সরিয়ে ফেলেছে"
		}
		return "The shop has removed this"
	default:
		return ""
	}
}

// blockerText is why checkout is not available, in words.
//
// The shop's own open status is folded in for the closed case, because "closed"
// on its own leaves the customer with nothing to do and "খুলবে ০৯:০০" tells them
// to come back.
func blockerText(blocker domain.Blocker, shop merchant.Merchant, lang string) string {
	bengali := lang != "en"
	switch blocker {
	case domain.BlockerEmpty:
		if bengali {
			return "আপনার কার্ট খালি।"
		}
		return "Your cart is empty."
	case domain.BlockerNoAddress:
		if bengali {
			return "ডেলিভারির ঠিকানা বেছে নিন।"
		}
		return "Choose a delivery address."
	case domain.BlockerShopUnavailable:
		if bengali {
			return "দোকানটি এখন অর্ডার নিচ্ছে না।"
		}
		return "This shop is not taking orders."
	case domain.BlockerShopClosed:
		if shop.OpenStatus != "" {
			return shop.OpenStatus
		}
		if bengali {
			return "দোকান এখন বন্ধ।"
		}
		return "The shop is closed."
	case domain.BlockerOutsideDivision:
		if bengali {
			return "এই ঠিকানা অন্য বিভাগে। এই দোকান থেকে সেখানে ডেলিভারি করা যাবে না।"
		}
		return "That address is in another division, so this shop cannot deliver to it."
	case domain.BlockerBeyondRadius:
		if bengali {
			return "এই ঠিকানা দোকান থেকে অনেক দূরে।"
		}
		return "That address is too far from this shop."
	case domain.BlockerLineProblems:
		if bengali {
			return "কার্টের কিছু জিনিস এখন পাওয়া যাচ্ছে না। সেগুলো সরিয়ে দিন।"
		}
		return "Some items in your cart are not available. Remove them to continue."
	default:
		return ""
	}
}
