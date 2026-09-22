package merchant

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"sync"

	geocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
)

var errStore = errors.New("store unavailable")

// memoryRepo holds merchants the way the real repository does, one shop per
// owner included — so a test that passes here is testing the rule and not the
// fake.
type memoryRepo struct {
	mu        sync.Mutex
	merchants map[string]domain.Merchant
	events    []ports.StatusChange

	readErr    error
	byOwnerErr error
	createErr  error
	saveErr    error
	deleteErr  error
	listErr    error
	eventErr   error
	historyErr error
}

func newRepo() *memoryRepo {
	return &memoryRepo{merchants: map[string]domain.Merchant{}}
}

func (m *memoryRepo) Create(_ context.Context, merchant domain.Merchant) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.merchants {
		if existing.OwnerUserID == merchant.OwnerUserID {
			return domain.ErrAlreadyRegistered
		}
	}
	m.merchants[merchant.ID] = merchant
	return nil
}

func (m *memoryRepo) Merchant(_ context.Context, merchantID string) (domain.Merchant, error) {
	if m.readErr != nil {
		return domain.Merchant{}, m.readErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	merchant, ok := m.merchants[merchantID]
	if !ok {
		return domain.Merchant{}, domain.ErrMerchantNotFound
	}
	return merchant, nil
}

func (m *memoryRepo) ByOwner(_ context.Context, ownerUserID string) (domain.Merchant, error) {
	if m.byOwnerErr != nil {
		return domain.Merchant{}, m.byOwnerErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, merchant := range m.merchants {
		if merchant.OwnerUserID == ownerUserID {
			return merchant, nil
		}
	}
	return domain.Merchant{}, domain.ErrMerchantNotFound
}

func (m *memoryRepo) Save(_ context.Context, merchant domain.Merchant) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.merchants[merchant.ID] = merchant
	return nil
}

func (m *memoryRepo) Delete(_ context.Context, merchantID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.merchants, merchantID)
	return nil
}

func (m *memoryRepo) List(_ context.Context, f ports.Filter) ([]domain.Merchant, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var matched []domain.Merchant
	for _, merchant := range m.merchants {
		if f.Status != "" && merchant.Status != f.Status {
			continue
		}
		if f.Type != "" && merchant.Type != f.Type {
			continue
		}
		if f.DivisionCode != "" && merchant.Placement.DivisionCode != f.DivisionCode {
			continue
		}
		matched = append(matched, merchant)
	}
	// Newest first, as the real query orders.
	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID > matched[j].ID
	})

	if f.Offset >= len(matched) {
		return nil, nil
	}
	matched = matched[f.Offset:]
	if f.Limit > 0 && len(matched) > f.Limit {
		matched = matched[:f.Limit]
	}
	return matched, nil
}

func (m *memoryRepo) RecordStatusChange(_ context.Context, e ports.StatusChange) error {
	if m.eventErr != nil {
		return m.eventErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

func (m *memoryRepo) StatusHistory(_ context.Context, merchantID string, limit int) ([]ports.StatusChange, error) {
	if m.historyErr != nil {
		return nil, m.historyErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []ports.StatusChange
	for i := len(m.events) - 1; i >= 0; i-- {
		if m.events[i].MerchantID != merchantID {
			continue
		}
		out = append(out, m.events[i])
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// fakeGeo stands in for the geo module, recording what the merchant module
// published so a test can assert the search index was kept in step.
type fakeGeo struct {
	mu     sync.Mutex
	placed map[string]geocontract.MerchantPlacement
	// removed records the ids withdrawn from the index.
	removed []string

	area       geocontract.Area
	resolveErr error
	placeErr   error
	removeErr  error
}

func newGeo() *fakeGeo {
	return &fakeGeo{
		placed: map[string]geocontract.MerchantPlacement{},
		area: geocontract.Area{
			AreaCode: "BD-C-DHA-01", AreaName: "Dhanmondi",
			DistrictCode: "BD-C-DHA", DivisionCode: "BD-C", DivisionName: "Dhaka",
		},
	}
}

func (g *fakeGeo) ResolveDivision(context.Context, geocontract.Point) (geocontract.Area, error) {
	if g.resolveErr != nil {
		return geocontract.Area{}, g.resolveErr
	}
	return g.area, nil
}

func (g *fakeGeo) PlaceMerchant(_ context.Context, p geocontract.MerchantPlacement) error {
	if g.placeErr != nil {
		return g.placeErr
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.placed[p.MerchantID] = p
	return nil
}

func (g *fakeGeo) RemoveMerchant(_ context.Context, merchantID string) error {
	if g.removeErr != nil {
		return g.removeErr
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.placed, merchantID)
	g.removed = append(g.removed, merchantID)
	return nil
}

// isActive reports whether a merchant is currently searchable.
func (g *fakeGeo) isActive(merchantID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.placed[merchantID].Active
}

// countingIDs hands out predictable ids, so a test can name the shop it made.
type countingIDs struct {
	mu sync.Mutex
	n  int
}

func (c *countingIDs) New(prefix string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return prefix + "_" + strconv.Itoa(c.n)
}
