package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/pricing"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// SearchUseCase answers "what can I order from, here" — D1's local visibility,
// D2's expansion and D3's ceiling in one call.
type SearchUseCase struct {
	geo      geo.Service
	merchant merchant.Service
	config   cfg.Service
	pricing  pricing.Service
}

// NewSearchUseCase wires the use case.
func NewSearchUseCase(g geo.Service, m merchant.Service, c cfg.Service, p pricing.Service) *SearchUseCase {
	return &SearchUseCase{geo: g, merchant: m, config: c, pricing: p}
}

// Query is one customer's search.
type Query struct {
	// Point is where the customer wants delivery, not where their phone is.
	// They are usually the same and occasionally very much not — somebody
	// ordering medicine for a parent in another upazila is a real case, and D3
	// decides it against the delivery point.
	Point geo.Point
	// Level is the expansion level the customer is asking for. Clamped, never
	// refused: the client is holding a number this server gave it, and
	// configuration may have narrowed the ladder since.
	Level int
	// Type filters by kind of shop — "restaurant", "grocery", "pharmacy" — or
	// is empty for all three.
	Type string
	// Text filters by shop name.
	Text string
	// Lang selects the language of the composed display strings. Anything but
	// "en" is Bengali, because the audience is (1.4).
	Lang string
	// Limit and Offset page the result.
	Limit  int
	Offset int
}

// MerchantCard is one shop as the customer's list renders it.
//
// Every field is final. There is nothing here for a client to compute, combine
// or decide — the distance is formatted, the fee is formatted, the open status
// is a sentence, and whether the shop is shown at all was decided here (2.9).
type MerchantCard struct {
	ID         string
	Name       string
	Type       string
	LogoURL    string
	Phone      string
	AreaName   string
	Lat        float64
	Lng        float64
	DistanceM  float64
	Distance   string
	IsOpenNow  bool
	OpenStatus string
	Delivery   pricing.Fee
}

// Result is a whole search: where we decided the customer is, what we found,
// and what widening would do.
type Result struct {
	Placement geo.Area
	Expansion domain.Expansion
	Merchants []MerchantCard
	// Total is how many merchants matched before paging, so the client can say
	// "১২টি দোকান" without another request.
	Total int
	// RadiusText and NextRadiusText are the searched and offered radii as the
	// customer reads them.
	RadiusText     string
	NextRadiusText string
	// Notice is the one line to show above the list: nothing found, expansion
	// offered, or the division ceiling reached. Composed here so every client
	// says the same thing (2.9).
	Notice string
}

// Execute runs ALG-01 and ALG-02.
//
// The shape is: resolve where the customer is, read the ladder configured for
// that place, settle on a level, run one radius query, then ask merchant which
// of the results may be seen. The division bound is applied by geo inside the
// query predicate, not filtered here — D3 is the one invariant that must not
// depend on a caller remembering to apply it.
func (uc *SearchUseCase) Execute(ctx context.Context, q Query) (Result, error) {
	if q.Type != "" && !validType(q.Type) {
		return Result{}, errs.New(errs.KindInvalid, "invalid_merchant_type",
			"That is not a kind of shop we deliver from.")
	}

	placement, err := uc.geo.ResolveDivision(ctx, q.Point)
	if err != nil {
		return Result{}, err
	}

	settings, err := uc.config.Settings(ctx, configPlacement(placement))
	if err != nil {
		return Result{}, configError(err)
	}
	policy, err := policyFrom(settings)
	if err != nil {
		return Result{}, err
	}

	level, err := uc.settle(ctx, policy, q)
	if err != nil {
		return Result{}, err
	}
	radius := policy.Radius(level)

	nearby, err := uc.geo.MerchantsWithinRadius(ctx, q.Point, radius, searchFanOut)
	if err != nil {
		return Result{}, err
	}

	cards, err := uc.cards(ctx, placement, nearby, level, radius, q)
	if err != nil {
		return Result{}, err
	}

	total := len(cards)
	expansion := domain.DescribeExpansion(policy, level, radius, total)

	return Result{
		Placement:      placement,
		Expansion:      expansion,
		Merchants:      page(cards, q.Limit, q.Offset),
		Total:          total,
		RadiusText:     domain.FormatRadius(expansion.RadiusM, q.Lang),
		NextRadiusText: nextRadiusText(expansion, q.Lang),
		Notice:         notice(expansion, q.Lang),
	}, nil
}

// settle decides which level to search at.
//
// With auto-expand off — the default — the answer is simply what the customer
// asked for: expansion costs money, so it is their decision to make (D2). With
// it on, the loop walks the ladder until enough shops are in range or the
// division ceiling stops it, which is ALG-02 exactly: O(s·log n) for s steps,
// each step a counting query rather than a fetch, because nobody reads the rows
// of a search that is about to be widened.
func (uc *SearchUseCase) settle(ctx context.Context, policy domain.Policy, q Query) (int, error) {
	level := policy.ClampLevel(q.Level)
	if !policy.AutoExpand() {
		return level, nil
	}

	for level < policy.MaxLevel() {
		found, err := uc.geo.CountMerchantsWithinRadius(ctx, q.Point, policy.Radius(level))
		if err != nil {
			return 0, err
		}
		if policy.Enough(found) {
			return level, nil
		}
		level++
	}
	return level, nil
}

// cards turns a radius result into the list the customer sees: which of these
// shops may be shown, filtered, ranked and priced.
func (uc *SearchUseCase) cards(
	ctx context.Context,
	placement geo.Area,
	nearby []geo.NearbyMerchant,
	level int,
	radius float64,
	q Query,
) ([]MerchantCard, error) {
	if len(nearby) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(nearby))
	distances := make(map[string]float64, len(nearby))
	for _, n := range nearby {
		ids = append(ids, n.MerchantID)
		distances[n.MerchantID] = n.DistanceM
	}

	listed, err := uc.merchant.Listed(ctx, ids)
	if err != nil {
		return nil, err
	}

	// One tariff for the whole page. Resolved here rather than per card so a
	// search cannot quote two shops against two different configurations.
	tariff, err := uc.pricing.Tariff(ctx, placementOf(placement))
	if err != nil {
		return nil, err
	}

	shops := make(map[string]merchant.Merchant, len(listed))
	candidates := make([]domain.Candidate, 0, len(listed))
	for _, m := range listed {
		if q.Type != "" && string(m.Type) != q.Type {
			continue
		}
		relevance, matched := domain.Relevance(m.Name, q.Text)
		if !matched {
			continue
		}
		shops[m.ID] = m
		candidates = append(candidates, domain.Candidate{
			ID:        m.ID,
			Relevance: relevance,
			DistanceM: distances[m.ID],
			Open:      m.IsOpenNow,
		})
	}

	domain.Rank(candidates, radius)

	out := make([]MerchantCard, 0, len(candidates))
	for _, c := range candidates {
		m := shops[c.ID]
		quote, quoteErr := tariff.DeliveryFee(pricing.QuoteRequest{
			DistanceM:      c.DistanceM,
			ExpansionLevel: level,
			Lang:           q.Lang,
		})
		if quoteErr != nil {
			return nil, quoteErr
		}
		out = append(out, MerchantCard{
			ID:         m.ID,
			Name:       m.Name,
			Type:       string(m.Type),
			LogoURL:    m.LogoURL,
			Phone:      m.Phone,
			AreaName:   m.AreaName,
			Lat:        m.Lat,
			Lng:        m.Lng,
			DistanceM:  c.DistanceM,
			Distance:   domain.FormatDistance(c.DistanceM, q.Lang),
			IsOpenNow:  m.IsOpenNow,
			OpenStatus: m.OpenStatus,
			Delivery:   quote,
		})
	}
	return out, nil
}

// validType reports whether a filter names a kind of shop we have.
func validType(t string) bool {
	switch merchant.Type(t) {
	case merchant.TypeRestaurant, merchant.TypeGrocery, merchant.TypePharmacy:
		return true
	default:
		return false
	}
}

// page applies limit and offset. Done here rather than in the query because
// the radius result is filtered and re-ranked after it comes back, so a
// database OFFSET would page the wrong list.
func page(cards []MerchantCard, limit, offset int) []MerchantCard {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(cards) {
		return []MerchantCard{}
	}
	end := len(cards)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return cards[offset:end]
}

// nextRadiusText renders the radius a customer would get by expanding, or
// nothing when there is no expansion left.
func nextRadiusText(e domain.Expansion, lang string) string {
	if !e.CanExpand {
		return ""
	}
	return domain.FormatRadius(e.NextRadiusM, lang)
}

// notice is the line above the list.
//
// Composed by the server, like every other display string (2.9). The four cases
// are genuinely different messages and not one template with a variable: "there
// is nothing here and nothing we can do about it" has to read as a final answer,
// or the customer keeps tapping a button that will never help.
func notice(e domain.Expansion, lang string) string {
	bengali := lang != "en"
	radius := domain.FormatRadius(e.RadiusM, lang)
	next := domain.FormatRadius(e.NextRadiusM, lang)

	switch {
	case e.Found == 0 && e.AtCeiling:
		if bengali {
			return "আপনার বিভাগের মধ্যে " + radius + " পর্যন্ত খুঁজে কোনো দোকান পাওয়া যায়নি। এটাই সর্বোচ্চ দূরত্ব।"
		}
		return "No shops found within " + radius + ", which is as far as we can search in your division."
	case e.Found == 0:
		if bengali {
			return radius + " এর মধ্যে কোনো দোকান নেই। " + next + " পর্যন্ত খুঁজে দেখুন — ডেলিভারি চার্জ বেশি হবে।"
		}
		return "No shops within " + radius + ". Try searching up to " + next + " — delivery will cost more."
	case e.Offered:
		if bengali {
			return "কাছাকাছি কম দোকান আছে। " + next + " পর্যন্ত খুঁজলে আরও পাবেন — ডেলিভারি চার্জ বেশি হবে।"
		}
		return "Only a few shops nearby. Searching up to " + next + " will find more — delivery will cost more."
	case e.AtCeiling:
		if bengali {
			return radius + " এর মধ্যে — এটাই আপনার বিভাগের সর্বোচ্চ দূরত্ব।"
		}
		return "Within " + radius + " — as far as we can search in your division."
	default:
		if bengali {
			return radius + " এর মধ্যে"
		}
		return "Within " + radius
	}
}
