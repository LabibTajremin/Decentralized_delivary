package catalogue

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

var errStore = errors.New("store unavailable")

// memoryRepo holds a catalogue the way the real repository does, including the
// per-shop scoping — so a test that passes here is testing the rule and not the
// fake.
type memoryRepo struct {
	mu         sync.Mutex
	categories map[string]domain.Category
	items      map[string]domain.Item
	combos     map[string]domain.Combo

	categoriesErr   error
	categoryErr     error
	saveCategoryErr error
	deleteCategErr  error
	itemsErr        error
	itemErr         error
	itemsByIDErr    error
	saveItemErr     error
	saveItemsErr    error
	deleteItemErr   error
	countErr        error
	combosErr       error
	comboErr        error
	saveComboErr    error
	deleteComboErr  error
}

func newRepo() *memoryRepo {
	return &memoryRepo{
		categories: map[string]domain.Category{},
		items:      map[string]domain.Item{},
		combos:     map[string]domain.Combo{},
	}
}

func (m *memoryRepo) Categories(_ context.Context, merchantID string) ([]domain.Category, error) {
	if m.categoriesErr != nil {
		return nil, m.categoriesErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []domain.Category
	for _, c := range m.categories {
		if c.MerchantID == merchantID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *memoryRepo) Category(_ context.Context, merchantID, categoryID string) (domain.Category, error) {
	if m.categoryErr != nil {
		return domain.Category{}, m.categoryErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.categories[categoryID]
	if !ok || c.MerchantID != merchantID {
		return domain.Category{}, domain.ErrCategoryNotFound
	}
	return c, nil
}

func (m *memoryRepo) SaveCategory(_ context.Context, c domain.Category) error {
	if m.saveCategoryErr != nil {
		return m.saveCategoryErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.categories[c.ID] = c
	return nil
}

func (m *memoryRepo) DeleteCategory(_ context.Context, merchantID, categoryID string) error {
	if m.deleteCategErr != nil {
		return m.deleteCategErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.categories[categoryID]; ok && c.MerchantID == merchantID {
		delete(m.categories, categoryID)
	}
	return nil
}

func (m *memoryRepo) Items(_ context.Context, f ports.ItemFilter) ([]domain.Item, error) {
	if m.itemsErr != nil {
		return nil, m.itemsErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []domain.Item
	for _, item := range m.items {
		switch {
		case item.MerchantID != f.MerchantID:
		case f.CategoryID != "" && item.CategoryID != f.CategoryID:
		case f.ActiveOnly && !item.Active:
		case f.Search != "" && !strings.Contains(strings.ToLower(item.Name), strings.ToLower(f.Search)):
		default:
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})

	if f.Offset >= len(out) {
		return nil, nil
	}
	out = out[f.Offset:]
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (m *memoryRepo) Item(_ context.Context, merchantID, itemID string) (domain.Item, error) {
	if m.itemErr != nil {
		return domain.Item{}, m.itemErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[itemID]
	if !ok || item.MerchantID != merchantID {
		return domain.Item{}, domain.ErrItemNotFound
	}
	return item, nil
}

func (m *memoryRepo) ItemsByID(_ context.Context, merchantID string, itemIDs []string) ([]domain.Item, error) {
	if m.itemsByIDErr != nil {
		return nil, m.itemsByIDErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []domain.Item
	for _, itemID := range itemIDs {
		if item, ok := m.items[itemID]; ok && item.MerchantID == merchantID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (m *memoryRepo) SaveItem(_ context.Context, i domain.Item) error {
	if m.saveItemErr != nil {
		return m.saveItemErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[i.ID] = i
	return nil
}

func (m *memoryRepo) SaveItems(_ context.Context, items []domain.Item) error {
	if m.saveItemsErr != nil {
		return m.saveItemsErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		m.items[item.ID] = item
	}
	return nil
}

func (m *memoryRepo) DeleteItem(_ context.Context, merchantID, itemID string) error {
	if m.deleteItemErr != nil {
		return m.deleteItemErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if item, ok := m.items[itemID]; ok && item.MerchantID == merchantID {
		delete(m.items, itemID)
	}
	return nil
}

func (m *memoryRepo) CountItemsInCategory(_ context.Context, merchantID, categoryID string) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	n := 0
	for _, item := range m.items {
		if item.MerchantID == merchantID && item.CategoryID == categoryID {
			n++
		}
	}
	return n, nil
}

func (m *memoryRepo) Combos(_ context.Context, merchantID string, activeOnly bool) ([]domain.Combo, error) {
	if m.combosErr != nil {
		return nil, m.combosErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []domain.Combo
	for _, combo := range m.combos {
		if combo.MerchantID != merchantID || (activeOnly && !combo.Active) {
			continue
		}
		out = append(out, combo)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *memoryRepo) Combo(_ context.Context, merchantID, comboID string) (domain.Combo, error) {
	if m.comboErr != nil {
		return domain.Combo{}, m.comboErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	combo, ok := m.combos[comboID]
	if !ok || combo.MerchantID != merchantID {
		return domain.Combo{}, domain.ErrComboNotFound
	}
	return combo, nil
}

func (m *memoryRepo) SaveCombo(_ context.Context, c domain.Combo) error {
	if m.saveComboErr != nil {
		return m.saveComboErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.combos[c.ID] = c
	return nil
}

func (m *memoryRepo) DeleteCombo(_ context.Context, merchantID, comboID string) error {
	if m.deleteComboErr != nil {
		return m.deleteComboErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if combo, ok := m.combos[comboID]; ok && combo.MerchantID == merchantID {
		delete(m.combos, comboID)
	}
	return nil
}

// fakeMerchants stands in for the merchant module.
type fakeMerchants struct {
	shops map[string]merchantcontract.Merchant
	err   error
}

func newMerchants() *fakeMerchants {
	return &fakeMerchants{shops: map[string]merchantcontract.Merchant{}}
}

// add registers a shop of a given type, owned by a given account.
func (f *fakeMerchants) add(merchantID, ownerUserID string, kind domain.MerchantType) {
	f.shops[merchantID] = merchantcontract.Merchant{
		ID: merchantID, OwnerUserID: ownerUserID,
		Type: merchantcontract.Type(kind), Name: "A shop",
	}
}

func (f *fakeMerchants) Merchant(_ context.Context, merchantID string) (merchantcontract.Merchant, error) {
	if f.err != nil {
		return merchantcontract.Merchant{}, f.err
	}
	shop, ok := f.shops[merchantID]
	if !ok {
		return merchantcontract.Merchant{}, errs.New(errs.KindNotFound, "merchant_not_found",
			"We could not find that shop.")
	}
	return shop, nil
}

// countingIDs hands out predictable ids, so a test can name what it made.
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
