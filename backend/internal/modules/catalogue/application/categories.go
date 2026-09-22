package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	merchantext "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// maxCategoriesPerShop caps a menu's sections.
//
// Fifty is far more than a shop needs and low enough that the whole menu still
// renders on a low-end phone. Refusing rather than silently dropping: a category
// is something the owner typed.
const maxCategoriesPerShop = 50

// CategoryUseCase manages a shop's menu sections.
type CategoryUseCase struct {
	repo      ports.CategoryRepository
	items     ports.ItemRepository
	merchants merchantext.Service
	ids       id.Generator
}

// NewCategoryUseCase wires the use case.
func NewCategoryUseCase(repo ports.CategoryRepository, items ports.ItemRepository, merchants merchantext.Service, ids id.Generator) *CategoryUseCase {
	return &CategoryUseCase{repo: repo, items: items, merchants: merchants, ids: ids}
}

// List returns a shop's categories.
func (uc *CategoryUseCase) List(ctx context.Context, merchantID string) ([]domain.Category, error) {
	categories, err := uc.repo.Categories(ctx, merchantID)
	if err != nil {
		return nil, unavailable(err, "We could not load the menu just now. Please try again.")
	}
	return categories, nil
}

// CategoryRequest is a new or edited section.
type CategoryRequest struct {
	Name      string
	SortOrder int
}

// Create adds a section to a shop's menu.
func (uc *CategoryUseCase) Create(ctx context.Context, ownerUserID, merchantID string, req CategoryRequest) (domain.Category, error) {
	if _, _, err := uc.owned(ctx, ownerUserID, merchantID); err != nil {
		return domain.Category{}, err
	}

	existing, err := uc.repo.Categories(ctx, merchantID)
	if err != nil {
		return domain.Category{}, unavailable(err, "We could not save that section just now. Please try again.")
	}
	if len(existing) >= maxCategoriesPerShop {
		return domain.Category{}, errs.New(errs.KindConflict, "too_many_categories",
			"Your menu has as many sections as we can keep. Please remove one first.")
	}

	sortOrder := req.SortOrder
	if sortOrder == 0 {
		// Appended to the end by default, which is where an owner adding a
		// section expects it — rather than at the top, silently reordering a
		// menu they were happy with.
		sortOrder = len(existing) + 1
	}

	category, err := domain.NewCategory(uc.ids.New("cat"), merchantID, req.Name, sortOrder)
	if err != nil {
		return domain.Category{}, entryError(err)
	}
	if err := uc.repo.SaveCategory(ctx, category); err != nil {
		return domain.Category{}, unavailable(err, "We could not save that section just now. Please try again.")
	}
	return category, nil
}

// Update renames or reorders a section.
func (uc *CategoryUseCase) Update(ctx context.Context, ownerUserID, merchantID, categoryID string, req CategoryRequest) (domain.Category, error) {
	category, err := uc.load(ctx, ownerUserID, merchantID, categoryID)
	if err != nil {
		return domain.Category{}, err
	}

	updated, err := category.WithName(req.Name)
	if err != nil {
		return domain.Category{}, entryError(err)
	}
	if req.SortOrder > 0 {
		updated = updated.WithSortOrder(req.SortOrder)
	}

	if err := uc.repo.SaveCategory(ctx, updated); err != nil {
		return domain.Category{}, unavailable(err, "We could not save that section just now. Please try again.")
	}
	return updated, nil
}

// SetActive shows or hides a section.
func (uc *CategoryUseCase) SetActive(ctx context.Context, ownerUserID, merchantID, categoryID string, active bool) (domain.Category, error) {
	category, err := uc.load(ctx, ownerUserID, merchantID, categoryID)
	if err != nil {
		return domain.Category{}, err
	}

	updated := category.WithActive(active)
	if err := uc.repo.SaveCategory(ctx, updated); err != nil {
		return domain.Category{}, unavailable(err, "We could not update that section just now. Please try again.")
	}
	return updated, nil
}

// Delete removes an empty section.
//
// Refused while items still point at it. Cascading would silently delete a
// shop's food; reassigning the items somewhere would be a decision the owner did
// not make. Telling them to empty it first is the only option that does not lose
// something they typed.
func (uc *CategoryUseCase) Delete(ctx context.Context, ownerUserID, merchantID, categoryID string) error {
	if _, err := uc.load(ctx, ownerUserID, merchantID, categoryID); err != nil {
		return err
	}

	count, err := uc.items.CountItemsInCategory(ctx, merchantID, categoryID)
	if err != nil {
		return unavailable(err, "We could not remove that section just now. Please try again.")
	}
	if count > 0 {
		return errs.New(errs.KindConflict, "category_not_empty",
			"Please move or remove the items in that section first.")
	}

	if err := uc.repo.DeleteCategory(ctx, merchantID, categoryID); err != nil {
		return unavailable(err, "We could not remove that section just now. Please try again.")
	}
	return nil
}

// load fetches a category the caller owns.
func (uc *CategoryUseCase) load(ctx context.Context, ownerUserID, merchantID, categoryID string) (domain.Category, error) {
	if _, _, err := uc.owned(ctx, ownerUserID, merchantID); err != nil {
		return domain.Category{}, err
	}

	category, err := uc.repo.Category(ctx, merchantID, categoryID)
	if errors.Is(err, domain.ErrCategoryNotFound) {
		return domain.Category{}, notFound(err, "category_not_found",
			"We could not find that section.", "category_id", categoryID)
	}
	if err != nil {
		return domain.Category{}, unavailable(err, "We could not load that section just now. Please try again.")
	}
	return category, nil
}

// owned resolves the shop and checks the caller owns it.
func (uc *CategoryUseCase) owned(ctx context.Context, ownerUserID, merchantID string) (merchantext.Merchant, domain.MerchantType, error) {
	found, kind, err := shop(ctx, uc.merchants, merchantID)
	if err != nil {
		return merchantext.Merchant{}, "", err
	}
	if err := ownedBy(found, ownerUserID); err != nil {
		return merchantext.Merchant{}, "", err
	}
	return found, kind, nil
}

// MerchantTypeOf reports what kind of shop this is, for the caller who owns it.
//
// The merchant app asks so it knows which fields to show — a pharmacy form has
// a strength box and no add-ons. Serving it rather than letting the app decide
// keeps one copy of the rule (2.9).
func (uc *CategoryUseCase) MerchantTypeOf(ctx context.Context, ownerUserID, merchantID string) (domain.MerchantType, error) {
	_, kind, err := uc.owned(ctx, ownerUserID, merchantID)
	if err != nil {
		return "", err
	}
	return kind, nil
}
