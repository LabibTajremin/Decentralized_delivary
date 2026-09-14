// Package application holds the merchant use cases.
package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	geoext "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/external/geo"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// RegistrationUseCase is a shop owner setting their shop up.
type RegistrationUseCase struct {
	repo  ports.Repository
	geo   geoext.Service
	clock clock.Clock
	ids   id.Generator
}

// NewRegistrationUseCase wires the use case.
func NewRegistrationUseCase(repo ports.Repository, geo geoext.Service, c clock.Clock, ids id.Generator) *RegistrationUseCase {
	return &RegistrationUseCase{repo: repo, geo: geo, clock: c, ids: ids}
}

// DetailsRequest is what an owner submits about their shop.
type DetailsRequest struct {
	Name    string
	Type    string
	Phone   string
	Email   string
	LogoURL string
	Line1   string
	Line2   string
	Lat     float64
	Lng     float64
}

// Register creates a shop in draft.
//
// This is D1 in code: the only geographic question asked is whether the pin is
// inside Bangladesh at all. There is no check against how many shops the
// division already has, how far the nearest customer is, or whether we
// currently deliver there. A shop in a village registers on the same terms as
// one in Dhanmondi; whether anyone can see it is settled later, by approval and
// by the radius search.
func (uc *RegistrationUseCase) Register(ctx context.Context, ownerUserID string, req DetailsRequest) (domain.Merchant, error) {
	if _, err := uc.repo.ByOwner(ctx, ownerUserID); err == nil {
		return domain.Merchant{}, errs.Wrap(domain.ErrAlreadyRegistered, errs.KindConflict,
			"already_registered", "This account already has a shop.")
	} else if !errors.Is(err, domain.ErrMerchantNotFound) {
		return domain.Merchant{}, unavailable(err, "We could not set your shop up just now. Please try again.")
	}

	details, err := toDetails(req)
	if err != nil {
		return domain.Merchant{}, err
	}

	merchant, err := domain.NewMerchant(uc.ids.New("mch"), ownerUserID, details, uc.clock.Now())
	if err != nil {
		return domain.Merchant{}, detailsError(err)
	}

	placed, err := uc.place(ctx, merchant)
	if err != nil {
		return domain.Merchant{}, err
	}

	if err := uc.repo.Create(ctx, placed); err != nil {
		if errors.Is(err, domain.ErrAlreadyRegistered) {
			// Two registrations racing: the unique constraint decided, and the
			// loser gets the same answer as if it had arrived second.
			return domain.Merchant{}, errs.Wrap(err, errs.KindConflict,
				"already_registered", "This account already has a shop.")
		}
		return domain.Merchant{}, unavailable(err, "We could not set your shop up just now. Please try again.")
	}

	// The location is published to geo only after the record exists. The other
	// order would leave a point in the search index with no shop behind it if
	// the write failed — a result discovery would return and nothing could
	// resolve.
	if err := uc.publish(ctx, placed); err != nil {
		return domain.Merchant{}, err
	}
	return placed, nil
}

// Update edits the shop's details.
//
// Refused while the shop is under review: an admin deciding on a set of details
// should be deciding on the set they were shown, and an owner who edits
// mid-review either invalidates the decision or gets approved for a shop nobody
// checked.
func (uc *RegistrationUseCase) Update(ctx context.Context, ownerUserID string, req DetailsRequest) (domain.Merchant, error) {
	merchant, err := uc.owned(ctx, ownerUserID)
	if err != nil {
		return domain.Merchant{}, err
	}
	if merchant.Status == domain.StatusPendingReview {
		return domain.Merchant{}, errs.New(errs.KindConflict, "under_review",
			"Your shop is being reviewed. You can make changes once we have finished.")
	}

	details, err := toDetails(req)
	if err != nil {
		return domain.Merchant{}, err
	}
	updated, err := merchant.WithDetails(details)
	if err != nil {
		return domain.Merchant{}, detailsError(err)
	}

	placed, err := uc.place(ctx, updated)
	if err != nil {
		return domain.Merchant{}, err
	}
	if err := uc.repo.Save(ctx, placed); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not save those changes just now. Please try again.")
	}
	if err := uc.publish(ctx, placed); err != nil {
		return domain.Merchant{}, err
	}
	return placed, nil
}

// DocumentRequest is one paper an owner is supplying.
type DocumentRequest struct {
	Kind    string
	Number  string
	FileURL string
}

// AddDocument attaches or replaces a document.
func (uc *RegistrationUseCase) AddDocument(ctx context.Context, ownerUserID string, req DocumentRequest) (domain.Merchant, error) {
	merchant, err := uc.owned(ctx, ownerUserID)
	if err != nil {
		return domain.Merchant{}, err
	}
	if merchant.Status == domain.StatusPendingReview {
		return domain.Merchant{}, errs.New(errs.KindConflict, "under_review",
			"Your shop is being reviewed. You can make changes once we have finished.")
	}

	kind, err := domain.ParseDocumentKind(req.Kind)
	if err != nil {
		return domain.Merchant{}, errs.Wrap(err, errs.KindInvalid, "unknown_document_kind",
			"We do not collect that kind of document.")
	}
	if !merchant.RequiresDocument(kind) {
		// Refused rather than stored and ignored: an owner who uploads a drug
		// licence to a grocery has misunderstood something, and accepting it
		// silently leaves them waiting for an approval that was never blocked
		// on it.
		return domain.Merchant{}, errs.Wrap(domain.ErrDocumentNotRequired, errs.KindInvalid,
			"document_not_required", "That document is not needed for this kind of shop.")
	}

	document, err := domain.NewDocument(kind, req.Number, req.FileURL, uc.clock.Now())
	if err != nil {
		return domain.Merchant{}, documentError(err)
	}

	updated := merchant.WithDocument(document)
	if err := uc.repo.Save(ctx, updated); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not save that document just now. Please try again.")
	}
	return updated, nil
}

// SubmitForReview hands the shop to an admin.
func (uc *RegistrationUseCase) SubmitForReview(ctx context.Context, ownerUserID string) (domain.Merchant, error) {
	merchant, err := uc.owned(ctx, ownerUserID)
	if err != nil {
		return domain.Merchant{}, err
	}
	if err := merchant.ReadyForReview(); err != nil {
		return domain.Merchant{}, readinessError(err, merchant.MissingDocuments())
	}

	submitted, err := merchant.WithStatus(domain.StatusPendingReview, "")
	if err != nil {
		return domain.Merchant{}, transitionError(err, merchant.Status)
	}
	if err := uc.repo.Save(ctx, submitted); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not submit your shop just now. Please try again.")
	}
	if err := uc.repo.RecordStatusChange(ctx, ports.StatusChange{
		ID:          uc.ids.New("mse"),
		MerchantID:  submitted.ID,
		From:        merchant.Status,
		To:          submitted.Status,
		ActorUserID: ownerUserID,
		At:          uc.clock.Now(),
	}); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not submit your shop just now. Please try again.")
	}
	return submitted, nil
}

// Mine returns the shop an account owns.
func (uc *RegistrationUseCase) Mine(ctx context.Context, ownerUserID string) (domain.Merchant, error) {
	return uc.owned(ctx, ownerUserID)
}

// owned loads the caller's shop.
func (uc *RegistrationUseCase) owned(ctx context.Context, ownerUserID string) (domain.Merchant, error) {
	merchant, err := uc.repo.ByOwner(ctx, ownerUserID)
	if errors.Is(err, domain.ErrMerchantNotFound) {
		return domain.Merchant{}, errs.Wrap(err, errs.KindNotFound, "no_shop",
			"You have not registered a shop yet.")
	}
	if err != nil {
		return domain.Merchant{}, unavailable(err, "We could not load your shop just now. Please try again.")
	}
	return merchant, nil
}

// place resolves the shop's administrative area from its pin.
func (uc *RegistrationUseCase) place(ctx context.Context, m domain.Merchant) (domain.Merchant, error) {
	area, err := uc.geo.ResolveDivision(ctx, geoext.Point{Lat: m.Pin.Lat, Lng: m.Pin.Lng})
	if err != nil {
		// Geo already distinguishes "outside the service area" from "we could
		// not check", and both messages are written for a person. Passing the
		// error through keeps one wording rather than inventing a second.
		return domain.Merchant{}, err
	}
	return m.WithPlacement(domain.Placement{
		AreaCode:     area.AreaCode,
		AreaName:     area.AreaName,
		DistrictCode: area.DistrictCode,
		DivisionCode: area.DivisionCode,
	}), nil
}

// publish tells geo where the shop is and whether it may be found.
//
// Active tracks listing, not existence: a shop in draft, under review, rejected
// or suspended keeps its point in the index with the flag off, so approving it
// is a flag change rather than a re-registration, and so a suspension takes
// effect on the next search rather than after a rebuild.
func (uc *RegistrationUseCase) publish(ctx context.Context, m domain.Merchant) error {
	return uc.geo.PlaceMerchant(ctx, geoext.Placement{
		MerchantID: m.ID,
		Lat:        m.Pin.Lat,
		Lng:        m.Pin.Lng,
		Active:     m.Status == domain.StatusApproved,
	})
}

// Withdraw abandons a registration that was never approved.
//
// Only from draft or rejected. An approved shop is withdrawn by suspending it,
// which is an admin's decision and leaves a record: orders, reviews and payouts
// point at a merchant id, and letting an owner delete one because a customer
// complained would take the evidence with it.
func (uc *RegistrationUseCase) Withdraw(ctx context.Context, ownerUserID string) error {
	merchant, err := uc.owned(ctx, ownerUserID)
	if err != nil {
		return err
	}
	if merchant.Status != domain.StatusDraft && merchant.Status != domain.StatusRejected {
		return errs.New(errs.KindConflict, "cannot_withdraw",
			"A shop that has been approved cannot be removed here. Please contact support.")
	}

	// Geo first. The reverse order could leave a point in the search index
	// pointing at a merchant that no longer exists, which discovery would
	// return and nothing could resolve; this order can at worst leave a
	// withdrawn shop the owner can delete again.
	if err := uc.geo.RemoveMerchant(ctx, merchant.ID); err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, merchant.ID); err != nil {
		return unavailable(err, "We could not remove your shop just now. Please try again.")
	}
	return nil
}

// toDetails validates a request into domain details.
func toDetails(req DetailsRequest) (domain.Details, error) {
	pin, err := domain.NewPin(req.Lat, req.Lng)
	if err != nil {
		return domain.Details{}, errs.Wrap(err, errs.KindInvalid, "invalid_pin",
			"Please drop the pin on your shop.")
	}
	kind, err := domain.ParseType(req.Type)
	if err != nil {
		return domain.Details{}, errs.Wrap(err, errs.KindInvalid, "unknown_merchant_type",
			"Please choose restaurant, grocery or pharmacy.")
	}
	return domain.Details{
		Name:    req.Name,
		Type:    kind,
		Phone:   req.Phone,
		Email:   req.Email,
		LogoURL: req.LogoURL,
		Line1:   req.Line1,
		Line2:   req.Line2,
		Pin:     pin,
	}, nil
}

// unavailable wraps a dependency failure.
func unavailable(err error, message string) error {
	return errs.Wrap(err, errs.KindUnavailable, "merchant_unavailable", message)
}

// detailsError turns a domain refusal into something an owner can act on.
//
// Type and pin failures are not listed: toDetails parses both before the domain
// sees them, so those branches cannot fire from this path and listing them
// would be code no test could reach.
func detailsError(err error) error {
	switch {
	case errors.Is(err, domain.ErrEmptyName):
		return errs.Wrap(err, errs.KindInvalid, "shop_name_required", "Please enter your shop's name.")
	case errors.Is(err, domain.ErrNameTooLong), errors.Is(err, domain.ErrTooLong):
		return errs.Wrap(err, errs.KindInvalid, "shop_details_too_long", "Some of those details are too long.")
	case errors.Is(err, domain.ErrInvalidPhone):
		return errs.Wrap(err, errs.KindInvalid, "invalid_phone",
			"Please enter your shop's mobile number, like 01712345678.")
	case errors.Is(err, domain.ErrInvalidEmail):
		return errs.Wrap(err, errs.KindInvalid, "invalid_email", "That email address does not look valid.")
	case errors.Is(err, domain.ErrInvalidLogoURL):
		return errs.Wrap(err, errs.KindInvalid, "invalid_logo_url", "That logo address is not valid.")
	case errors.Is(err, domain.ErrEmptyAddressLine):
		return errs.Wrap(err, errs.KindInvalid, "address_line_required", "Please enter your shop's street address.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_shop_details", "Some of those details are not valid.")
	}
}

// documentError turns a document refusal into something an owner can act on.
func documentError(err error) error {
	switch {
	case errors.Is(err, domain.ErrEmptyDocumentNumber):
		return errs.Wrap(err, errs.KindInvalid, "document_number_required",
			"Please enter the number printed on the document.")
	case errors.Is(err, domain.ErrEmptyDocumentFile):
		return errs.Wrap(err, errs.KindInvalid, "document_file_required",
			"Please attach a photo of the document.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_document", "That document is not valid.")
	}
}

// readinessError explains what is still missing before a review.
func readinessError(err error, missing []domain.DocumentKind) error {
	if errors.Is(err, domain.ErrUnplaced) {
		return errs.Wrap(err, errs.KindConflict, "shop_not_placed",
			"We could not place your shop on the map. Please set the pin again.")
	}
	wrapped := errs.Wrap(err, errs.KindConflict, "documents_incomplete",
		"Please upload the remaining documents before submitting.")
	for _, kind := range missing {
		wrapped = wrapped.With("missing_"+kind.String(), "required")
	}
	return wrapped
}

// transitionError explains a refused status change.
func transitionError(err error, from domain.Status) error {
	return errs.Wrap(err, errs.KindConflict, "invalid_status_change",
		"Your shop cannot move to that state from where it is now.").With("status", from.String())
}
