package cart

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	catcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
)

// The sentences. Every one of them is composed by the server so that every
// client says the same thing, and Bengali-first because the audience is (1.4).
// Pinning them here is what makes a change to the wording a deliberate act.

// blockedCart drives a cart into one blocker and returns the rendered view.
func blockedCart(t *testing.T, lang string, setUp func(*rig)) application.View {
	t.Helper()
	r := withBurger()
	add(t, r, "ITM-burger", 1)
	if _, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7, 90.4, lang); err != nil {
		t.Fatalf("SetAddress: %v", err)
	}
	setUp(r)
	view, _, err := r.carts.Current(context.Background(), "USR-1", lang)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	return view
}

func TestBlockerSentences(t *testing.T) {
	suspend := func(r *rig) { r.merchant.shop = merchantcontract.Merchant{ID: "MER-1"} }
	closeShop := func(r *rig) {
		r.merchant.shop = merchantcontract.Merchant{ID: "MER-1", IsListed: true}
	}
	otherDivision := func(r *rig) {
		r.discovery.reach = contract.Reach{Reason: contract.ReasonOutsideDivision}
	}
	tooFar := func(r *rig) {
		r.discovery.reach = contract.Reach{Reason: contract.ReasonBeyondMaxRadius}
	}
	soldOut := func(r *rig) {
		gone := orderableItem("ITM-burger", "Beef Burger", 25000)
		gone.Orderable = false
		gone.UnavailableReason = "out_of_stock"
		r.catalogue.items["ITM-burger"] = gone
	}

	cases := []struct {
		name             string
		setUp            func(*rig)
		blocker          domain.Blocker
		bengali, english string
	}{
		{"shop suspended", suspend, domain.BlockerShopUnavailable,
			"দোকানটি এখন অর্ডার নিচ্ছে না।", "This shop is not taking orders."},
		{"shop closed with no status", closeShop, domain.BlockerShopClosed,
			"দোকান এখন বন্ধ।", "The shop is closed."},
		{"another division", otherDivision, domain.BlockerOutsideDivision,
			"এই ঠিকানা অন্য বিভাগে। এই দোকান থেকে সেখানে ডেলিভারি করা যাবে না।",
			"That address is in another division, so this shop cannot deliver to it."},
		{"too far", tooFar, domain.BlockerBeyondRadius,
			"এই ঠিকানা দোকান থেকে অনেক দূরে।", "That address is too far from this shop."},
		{"a line is not orderable", soldOut, domain.BlockerLineProblems,
			"কার্টের কিছু জিনিস এখন পাওয়া যাচ্ছে না। সেগুলো সরিয়ে দিন।",
			"Some items in your cart are not available. Remove them to continue."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bn := blockedCart(t, "", tc.setUp)
			if bn.Blocker != string(tc.blocker) || bn.BlockerText != tc.bengali {
				t.Errorf("bn: blocker = %q, text = %q", bn.Blocker, bn.BlockerText)
			}
			en := blockedCart(t, "en", tc.setUp)
			if en.BlockerText != tc.english {
				t.Errorf("en: text = %q, want %q", en.BlockerText, tc.english)
			}
		})
	}
}

// The two blockers that need no address to reach.
func TestEmptyAndAddresslessSentences(t *testing.T) {
	r := withBurger()
	view, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "item", TargetID: "ITM-burger", Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if view.BlockerText != "ডেলিভারির ঠিকানা বেছে নিন।" {
		t.Errorf("bn no-address = %q", view.BlockerText)
	}

	en := withBurger()
	if got := add(t, en, "ITM-burger", 1); got.BlockerText != "Choose a delivery address." {
		t.Errorf("en no-address = %q", got.BlockerText)
	}

	lineID := view.Lines[0].ID
	emptied, err := r.carts.Remove(context.Background(), "USR-1", lineID, "")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if emptied.BlockerText != "আপনার কার্ট খালি।" {
		t.Errorf("bn empty = %q", emptied.BlockerText)
	}
	enEmptied, err := en.carts.Remove(context.Background(), "USR-1", en.repo.carts["USR-1"].Lines[0].ID, "en")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if enEmptied.BlockerText != "Your cart is empty." {
		t.Errorf("en empty = %q", enEmptied.BlockerText)
	}
}

// A cart that is ready has no sentence at all — there is nothing to explain.
func TestAReadyCartHasNoBlockerSentence(t *testing.T) {
	view := blockedCart(t, "en", func(*rig) {})
	if view.Blocker != "" || view.BlockerText != "" {
		t.Fatalf("a ready cart carried %q / %q", view.Blocker, view.BlockerText)
	}
	if view.Lines[0].Issue != "" || view.Lines[0].IssueText != "" {
		t.Errorf("an unchanged line carried %q / %q", view.Lines[0].Issue, view.Lines[0].IssueText)
	}
}

func TestIssueSentences(t *testing.T) {
	cases := []struct {
		name             string
		mutate           func(*rig)
		issue            domain.Issue
		bengali, english string
	}{
		{"price changed", func(r *rig) {
			r.catalogue.items["ITM-burger"] = orderableItem("ITM-burger", "Beef Burger", 30000)
		}, domain.IssuePriceChanged, "দাম পরিবর্তন হয়েছে", "The price has changed"},

		{"out of stock", func(r *rig) {
			item := orderableItem("ITM-burger", "Beef Burger", 25000)
			item.Orderable, item.UnavailableReason = false, "out_of_stock"
			r.catalogue.items["ITM-burger"] = item
		}, domain.IssueOutOfStock, "স্টকে নেই", "Out of stock"},

		{"not available now", func(r *rig) {
			item := orderableItem("ITM-burger", "Beef Burger", 25000)
			item.Orderable, item.UnavailableReason = false, "not_available_now"
			r.catalogue.items["ITM-burger"] = item
		}, domain.IssueNotAvailableNow, "এখন পাওয়া যাচ্ছে না", "Not available at this time"},

		{"taken down", func(r *rig) {
			item := orderableItem("ITM-burger", "Beef Burger", 25000)
			item.Orderable, item.UnavailableReason = false, "unavailable"
			r.catalogue.items["ITM-burger"] = item
		}, domain.IssueUnavailable, "এখন আর পাওয়া যাচ্ছে না", "No longer available"},

		{"deleted", func(r *rig) {
			delete(r.catalogue.items, "ITM-burger")
		}, domain.IssueRemoved, "দোকান এটি সরিয়ে ফেলেছে", "The shop has removed this"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for lang, want := range map[string]string{"": tc.bengali, "en": tc.english} {
				r := withBurger()
				add(t, r, "ITM-burger", 1)
				tc.mutate(r)
				got, _, err := r.carts.Current(context.Background(), "USR-1", lang)
				if err != nil {
					t.Fatalf("Current: %v", err)
				}
				if got.Lines[0].Issue != string(tc.issue) {
					t.Fatalf("issue = %q, want %q", got.Lines[0].Issue, tc.issue)
				}
				if got.Lines[0].IssueText != want {
					t.Errorf("lang %q: text = %q, want %q", lang, got.Lines[0].IssueText, want)
				}
			}
		})
	}
}

// Every domain rule reaches the customer as something they can act on.
func TestEveryRuleGetsASentence(t *testing.T) {
	ctx := context.Background()

	t.Run("a cart that is already full", func(t *testing.T) {
		r := newRig()
		for i := 0; i < domain.MaxLines; i++ {
			id := "ITM-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
			r.catalogue.items[id] = orderableItem(id, id, 1000)
			add(t, r, id, 1)
		}
		r.catalogue.items["ITM-one-more"] = orderableItem("ITM-one-more", "One more", 1000)
		_, err := r.carts.Add(ctx, "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "item", TargetID: "ITM-one-more", Quantity: 1,
		})
		if got := codeOf(err); got != "cart_full" {
			t.Fatalf("err = %v, want cart_full", err)
		}
	})

	t.Run("a note longer than the field", func(t *testing.T) {
		r := withBurger()
		_, err := r.carts.Add(ctx, "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "item", TargetID: "ITM-burger", Quantity: 1,
			Note: string(make([]byte, domain.MaxNote+1)),
		})
		if got := codeOf(err); got != "note_too_long" {
			t.Fatalf("err = %v, want note_too_long", err)
		}
	})

	t.Run("a line that is no longer there", func(t *testing.T) {
		r := withBurger()
		add(t, r, "ITM-burger", 1)
		_, err := r.carts.Remove(ctx, "USR-1", "CLN-gone", "en")
		if got := codeOf(err); got != "line_not_found" {
			t.Fatalf("err = %v, want line_not_found", err)
		}
	})

	t.Run("a quantity past the cap", func(t *testing.T) {
		r := withBurger()
		view := add(t, r, "ITM-burger", 1)
		_, err := r.carts.SetQuantity(ctx, "USR-1", view.Lines[0].ID, domain.MaxQuantity+1, "en")
		if got := codeOf(err); got != "invalid_quantity" {
			t.Fatalf("err = %v, want invalid_quantity", err)
		}
	})
}

// An item whose options the customer never chose, added to a cart that is then
// revalidated after the shop removed the whole group. The group going is not
// the same as an option going: nothing was chosen, so nothing is missing.
func TestRemovingAGroupNobodyUsedLeavesTheLineAlone(t *testing.T) {
	r := newRig()
	r.catalogue.items["ITM-tea"] = catcontract.Item{
		ID: "ITM-tea", MerchantID: "MER-1", Name: "Tea", Price: taka(2000), Orderable: true,
		AddOnGroups: []catcontract.OptionGroup{{
			ID: "GRP-sugar", Name: "Sugar",
			Options: []catcontract.Option{{ID: "OPT-extra", Name: "Extra", Price: taka(500), Available: true}},
		}},
	}
	add(t, r, "ITM-tea", 1)
	r.catalogue.items["ITM-tea"] = orderableItem("ITM-tea", "Tea", 2000)

	got, _, err := r.carts.Current(context.Background(), "USR-1", "en")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got.Lines[0].Issue != "" || !got.Lines[0].Orderable {
		t.Fatalf("line = %+v", got.Lines[0])
	}
}
