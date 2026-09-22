package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	merchantext "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// ComboUseCase manages bundles.
type ComboUseCase struct {
	repo      ports.ComboRepository
	items     ports.ItemRepository
	merchants merchantext.Service
	ids       id.Generator
}

// NewComboUseCase wires the use case.
func NewComboUseCase(repo ports.ComboRepository, items ports.ItemRepository, merchants merchantext.Service, ids id.Generator) *ComboUseCase {
	return &ComboUseCase{repo: repo, items: items, merchants: merchants, ids: ids}
}

// List returns a shop's combos.
func (uc *ComboUseCase) List(ctx context.Context, merchantID string, activeOnly bool) ([]domain.Combo, error) {
	combos, err := uc.repo.Combos(ctx, merchantID, activeOnly)
	if err != nil {
		return nil, unavailable(err, "We could not load those combos just now. Please try again.")
	}
	return combos, nil
}

// Get returns one combo.
func (uc *ComboUseCase) Get(ctx context.Context, merchantID, comboID string) (domain.Combo, error) {
	combo, err := uc.repo.Combo(ctx, merchantID, comboID)
	if errors.Is(err, domain.ErrComboNotFound) {
		return domain.Combo{}, notFound(err, "combo_not_found",
			"We could not find that combo.", "combo_id", comboID)
	}
	if err != nil {
		return domain.Combo{}, unavailable(err, "We could not load that combo just now. Please try again.")
	}
	return combo, nil
}

// ComboRequest is a new or edited bundle.
type ComboRequest struct {
	Name        string
	Description string
	ImageURL    string
	PriceMinor  int64
	SortOrder   int
	Lines       []ComboLineRequest
}

// ComboLineRequest is one item in a bundle.
type ComboLineRequest struct {
	ItemID   string
	Quantity int
}

// Create adds a combo.
func (uc *ComboUseCase) Create(ctx context.Context, ownerUserID, merchantID string, req ComboRequest) (domain.Combo, error) {
	_, kind, err := uc.owned(ctx, ownerUserID, merchantID)
	if err != nil {
		return domain.Combo{}, err
	}

	combo, err := domain.NewCombo(uc.ids.New("cmb"), merchantID, kind, uc.toDraft(req))
	if err != nil {
		return domain.Combo{}, entryError(err)
	}
	if err := uc.membersExist(ctx, merchantID, combo); err != nil {
		return domain.Combo{}, err
	}
	if err := uc.repo.SaveCombo(ctx, combo); err != nil {
		return domain.Combo{}, unavailable(err, "We could not save that combo just now. Please try again.")
	}
	return combo, nil
}

// Update edits a combo, keeping whether it is shown and when.
func (uc *ComboUseCase) Update(ctx context.Context, ownerUserID, merchantID, comboID string, req ComboRequest) (domain.Combo, error) {
	existing, kind, err := uc.ownedCombo(ctx, ownerUserID, merchantID, comboID)
	if err != nil {
		return domain.Combo{}, err
	}

	// Rebuilt through NewCombo rather than patched, so an edit is validated by
	// exactly the rules a creation is.
	updated, err := domain.NewCombo(existing.ID, merchantID, kind, uc.toDraft(req))
	if err != nil {
		return domain.Combo{}, entryError(err)
	}
	updated.Active = existing.Active
	updated.Availability = existing.Availability

	if err := uc.membersExist(ctx, merchantID, updated); err != nil {
		return domain.Combo{}, err
	}
	if err := uc.repo.SaveCombo(ctx, updated); err != nil {
		return domain.Combo{}, unavailable(err, "We could not save that combo just now. Please try again.")
	}
	return updated, nil
}

// SetActive shows or hides a combo.
func (uc *ComboUseCase) SetActive(ctx context.Context, ownerUserID, merchantID, comboID string, active bool) (domain.Combo, error) {
	combo, _, err := uc.ownedCombo(ctx, ownerUserID, merchantID, comboID)
	if err != nil {
		return domain.Combo{}, err
	}
	updated := combo.WithActive(active)
	if err := uc.repo.SaveCombo(ctx, updated); err != nil {
		return domain.Combo{}, unavailable(err, "We could not update that combo just now. Please try again.")
	}
	return updated, nil
}

// SetAvailability puts a combo on a schedule, or takes it off one.
func (uc *ComboUseCase) SetAvailability(ctx context.Context, ownerUserID, merchantID, comboID string, req AvailabilityRequest) (domain.Combo, error) {
	combo, _, err := uc.ownedCombo(ctx, ownerUserID, merchantID, comboID)
	if err != nil {
		return domain.Combo{}, err
	}

	availability := domain.AlwaysAvailable()
	if !req.Always {
		if len(req.Days) == 0 {
			return domain.Combo{}, errs.New(errs.KindInvalid, "availability_required",
				"Please choose when this is available, or mark it always available.")
		}
		decoded, decodeErr := domain.DecodeAvailability(req.Days)
		if decodeErr != nil {
			return domain.Combo{}, errs.Wrap(decodeErr, errs.KindInvalid, "invalid_availability",
				"Please write availability times like 07:00-11:00.")
		}
		availability = decoded
	}

	updated := combo.WithAvailability(availability)
	if err := uc.repo.SaveCombo(ctx, updated); err != nil {
		return domain.Combo{}, unavailable(err, "We could not update that combo just now. Please try again.")
	}
	return updated, nil
}

// Delete removes a combo. The items in it are untouched: a bundle is an offer
// over things that exist independently of it.
func (uc *ComboUseCase) Delete(ctx context.Context, ownerUserID, merchantID, comboID string) error {
	if _, _, err := uc.ownedCombo(ctx, ownerUserID, merchantID, comboID); err != nil {
		return err
	}
	if err := uc.repo.DeleteCombo(ctx, merchantID, comboID); err != nil {
		return unavailable(err, "We could not remove that combo just now. Please try again.")
	}
	return nil
}

// membersExist checks every line points at an item of this shop.
//
// Checked here rather than in the domain because it needs the repository. The
// alternative — trusting the ids — is a combo that resolves to nothing at
// checkout, which the customer discovers and the owner does not.
func (uc *ComboUseCase) membersExist(ctx context.Context, merchantID string, combo domain.Combo) error {
	wanted := combo.ItemIDs()
	found, err := uc.items.ItemsByID(ctx, merchantID, wanted)
	if err != nil {
		return unavailable(err, "We could not check those items just now. Please try again.")
	}

	present := make(map[string]bool, len(found))
	for _, item := range found {
		present[item.ID] = true
	}
	for _, itemID := range wanted {
		if !present[itemID] {
			return notFound(domain.ErrItemNotFound,
				"item_not_found", "One of those items is not on this menu.", "item_id", itemID)
		}
	}
	return nil
}

// toDraft turns a request into a draft.
func (uc *ComboUseCase) toDraft(req ComboRequest) domain.ComboDraft {
	lines := make([]domain.ComboLine, 0, len(req.Lines))
	for _, line := range req.Lines {
		lines = append(lines, domain.ComboLine{ItemID: line.ItemID, Quantity: line.Quantity})
	}
	return domain.ComboDraft{
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		Price:       money.Taka(req.PriceMinor),
		Lines:       lines,
		SortOrder:   req.SortOrder,
	}
}

// owned resolves the shop and checks the caller owns it.
func (uc *ComboUseCase) owned(ctx context.Context, ownerUserID, merchantID string) (merchantext.Merchant, domain.MerchantType, error) {
	found, kind, err := shop(ctx, uc.merchants, merchantID)
	if err != nil {
		return merchantext.Merchant{}, "", err
	}
	if err := ownedBy(found, ownerUserID); err != nil {
		return merchantext.Merchant{}, "", err
	}
	return found, kind, nil
}

// ownedCombo resolves the shop, checks ownership, and loads the combo.
func (uc *ComboUseCase) ownedCombo(ctx context.Context, ownerUserID, merchantID, comboID string) (domain.Combo, domain.MerchantType, error) {
	_, kind, err := uc.owned(ctx, ownerUserID, merchantID)
	if err != nil {
		return domain.Combo{}, "", err
	}
	combo, err := uc.Get(ctx, merchantID, comboID)
	if err != nil {
		return domain.Combo{}, "", err
	}
	return combo, kind, nil
}
