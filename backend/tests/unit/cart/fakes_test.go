// Package cart holds the unit tests for the cart module.
package cart

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/discovery"
	catcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/contract"
	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	discocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

var errBoom = errors.New("boom")

// fakeRepo is an in-memory cart store, one cart per user, as the unique index
// in the schema enforces.
type fakeRepo struct {
	carts map[string]domain.Cart

	readErr   error
	saveErr   error
	deleteErr error

	saves   int
	deletes int
}

func newRepo() *fakeRepo { return &fakeRepo{carts: map[string]domain.Cart{}} }

func (r *fakeRepo) OfUser(_ context.Context, userID string) (domain.Cart, bool, error) {
	if r.readErr != nil {
		return domain.Cart{}, false, r.readErr
	}
	c, ok := r.carts[userID]
	return c, ok, nil
}

func (r *fakeRepo) Save(_ context.Context, c domain.Cart) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saves++
	r.carts[c.UserID] = c
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, cartID string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletes++
	for userID, c := range r.carts {
		if c.ID == cartID {
			delete(r.carts, userID)
		}
	}
	return nil
}

// fakeCatalogue stands in for the catalogue module.
type fakeCatalogue struct {
	items  map[string]catcontract.Item
	combos map[string]catcontract.Combo

	itemErr  error
	itemsErr error
	comboErr error

	// batches counts the calls to Items, which is how the tests check that
	// revalidating eight lines does not make eight round trips.
	batches int
}

func newCatalogue() *fakeCatalogue {
	return &fakeCatalogue{
		items:  map[string]catcontract.Item{},
		combos: map[string]catcontract.Combo{},
	}
}

func (c *fakeCatalogue) Item(_ context.Context, _, itemID string) (catcontract.Item, error) {
	if c.itemErr != nil {
		return catcontract.Item{}, c.itemErr
	}
	item, ok := c.items[itemID]
	if !ok {
		return catcontract.Item{}, errs.New(errs.KindNotFound, "item_not_found", "No such item.")
	}
	return item, nil
}

func (c *fakeCatalogue) Items(_ context.Context, _ string, itemIDs []string) ([]catcontract.Item, error) {
	c.batches++
	if c.itemsErr != nil {
		return nil, c.itemsErr
	}
	out := make([]catcontract.Item, 0, len(itemIDs))
	for _, id := range itemIDs {
		if item, ok := c.items[id]; ok {
			out = append(out, item)
		}
	}
	return out, nil
}

func (c *fakeCatalogue) Combo(_ context.Context, _, comboID string) (catcontract.Combo, error) {
	if c.comboErr != nil {
		return catcontract.Combo{}, c.comboErr
	}
	combo, ok := c.combos[comboID]
	if !ok {
		return catcontract.Combo{}, errs.New(errs.KindNotFound, "combo_not_found", "No such bundle.")
	}
	return combo, nil
}

// fakeMerchant stands in for the merchant module.
type fakeMerchant struct {
	shop merchantcontract.Merchant
	err  error
}

func (m *fakeMerchant) Merchant(_ context.Context, _ string) (merchantcontract.Merchant, error) {
	return m.shop, m.err
}

// fakeDiscovery stands in for the discovery module.
type fakeDiscovery struct {
	reach discocontract.Reach
	err   error
	// asked records the points discovery was asked about, so a test can prove
	// the cart re-asks after an address change rather than caching the answer.
	asked []discovery.Point
}

func (d *fakeDiscovery) Reach(_ context.Context, from discovery.Point, merchantID string) (discocontract.Reach, error) {
	d.asked = append(d.asked, from)
	if d.err != nil {
		return discocontract.Reach{}, d.err
	}
	reach := d.reach
	reach.MerchantID = merchantID
	return reach, nil
}

// fakeIDs hands out predictable ids so assertions can name a line.
type fakeIDs struct{ n int }

func (g *fakeIDs) New(prefix string) string {
	g.n++
	return prefix + "-" + string(rune('0'+g.n))
}

// money builds a contract amount the way catalogue emits one.
func taka(minor int64) catcontract.Money {
	return catcontract.Money{Minor: minor, Currency: "BDT", Display: "৳"}
}

// openShop is a listed, open restaurant.
func openShop() merchantcontract.Merchant {
	return merchantcontract.Merchant{
		ID: "MER-1", Name: "Star Kabab", Type: merchantcontract.TypeRestaurant,
		LogoURL: "/logo.png", IsListed: true, IsOpenNow: true,
		OpenStatus: "খোলা আছে", DivisionCode: "DHA",
	}
}

// orderableItem is a plain item with no options.
func orderableItem(id, name string, minor int64) catcontract.Item {
	return catcontract.Item{
		ID: id, MerchantID: "MER-1", Name: name,
		Price: taka(minor), Orderable: true,
	}
}

// pizza is an item with a required size group and an optional extras group,
// used wherever a test needs options rather than a bare price.
func pizza() catcontract.Item {
	return catcontract.Item{
		ID: "ITM-pizza", MerchantID: "MER-1", Name: "Pizza",
		Price: taka(50000), Orderable: true,
		VariantGroups: []catcontract.OptionGroup{{
			ID: "GRP-size", Name: "Size", Required: true, MinChoices: 1, MaxChoices: 1,
			Options: []catcontract.Option{
				{ID: "OPT-small", Name: "Small", Price: taka(0), Available: true},
				{ID: "OPT-large", Name: "Large", Price: taka(15000), Available: true},
			},
		}},
	}
}

// codeOf is errs.CodeOf, named here so the sentence tests read as one idea.
func codeOf(err error) string { return errs.CodeOf(err) }

// fakeSettings and fakeConfig stand in for the config module, which the cart
// reaches only through pricing.
type fakeSettings struct {
	ints   map[string]int64
	ratios map[string]float64
	failOn string
}

func (s fakeSettings) Int(key string) (int64, error) {
	if key == s.failOn {
		return 0, errBoom
	}
	v, ok := s.ints[key]
	if !ok {
		return 0, errBoom
	}
	return v, nil
}

func (s fakeSettings) Bool(string) (bool, error) { return false, errBoom }

func (s fakeSettings) Ratio(key string) (float64, error) {
	if key == s.failOn {
		return 0, errBoom
	}
	v, ok := s.ratios[key]
	if !ok {
		return 0, errBoom
	}
	return v, nil
}

type fakeConfig struct {
	settings cfgcontract.Settings
	err      error
	seen     []cfgcontract.Placement
}

func (f *fakeConfig) Settings(_ context.Context, p cfgcontract.Placement) (cfgcontract.Settings, error) {
	f.seen = append(f.seen, p)
	if f.err != nil {
		return nil, f.err
	}
	return f.settings, nil
}

// appendixB is the default tariff: ৳40 base, ৳10 a kilometre, 1.5× when the
// radius was widened, free delivery above ৳500.
func appendixB() fakeSettings {
	return fakeSettings{
		ints: map[string]int64{
			cfgcontract.PricingDeliveryBase:  4000,
			cfgcontract.PricingDeliveryPerKm: 1000,
			cfgcontract.PricingFreeDelivery:  50000,
		},
		ratios: map[string]float64{cfgcontract.PricingExpansionMult: 1.5},
	}
}
