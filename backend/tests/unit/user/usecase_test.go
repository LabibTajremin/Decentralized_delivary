package user

import (
	"context"
	"errors"
	"sync"
	"testing"

	geocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

var errStore = errors.New("store unavailable")

// memoryRepo holds profiles and addresses, applying the same "exactly one
// default" rule the real repository's transaction does — so a test that passes
// here is testing the rule and not the fake.
type memoryRepo struct {
	mu        sync.Mutex
	profiles  map[string]domain.Profile
	addresses map[string][]domain.Address

	profileReadErr  error
	profileWriteErr error
	listErr         error
	saveErr         error
	deleteErr       error
	defaultErr      error
}

func newRepo() *memoryRepo {
	return &memoryRepo{
		profiles:  map[string]domain.Profile{},
		addresses: map[string][]domain.Address{},
	}
}

func (m *memoryRepo) Profile(_ context.Context, userID string) (domain.Profile, error) {
	if m.profileReadErr != nil {
		return domain.Profile{}, m.profileReadErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, ok := m.profiles[userID]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	return profile, nil
}

func (m *memoryRepo) SaveProfile(_ context.Context, profile domain.Profile) error {
	if m.profileWriteErr != nil {
		return m.profileWriteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles[profile.UserID] = profile
	return nil
}

func (m *memoryRepo) Addresses(_ context.Context, userID string) ([]domain.Address, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.Address(nil), m.addresses[userID]...), nil
}

func (m *memoryRepo) Address(_ context.Context, userID, addressID string) (domain.Address, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.addresses[userID] {
		if a.ID == addressID {
			return a, nil
		}
	}
	return domain.Address{}, domain.ErrAddressNotFound
}

func (m *memoryRepo) DefaultAddress(_ context.Context, userID string) (domain.Address, error) {
	if m.defaultErr != nil {
		return domain.Address{}, m.defaultErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.addresses[userID] {
		if a.IsDefault {
			return a, nil
		}
	}
	return domain.Address{}, domain.ErrNoDefaultAddress
}

func (m *memoryRepo) SaveAddresses(_ context.Context, userID string, addresses []domain.Address) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addresses[userID] = append([]domain.Address(nil), addresses...)
	return nil
}

func (m *memoryRepo) DeleteAddress(_ context.Context, userID, addressID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.addresses[userID][:0]
	for _, a := range m.addresses[userID] {
		if a.ID == addressID {
			continue
		}
		kept = append(kept, a)
	}
	m.addresses[userID] = append([]domain.Address(nil), kept...)
	return nil
}

// stubGeo places every pin in Dhanmondi unless told otherwise.
type stubGeo struct {
	area geocontract.Area
	err  error
	seen []geocontract.Point
	mu   sync.Mutex
}

func newGeo() *stubGeo {
	return &stubGeo{area: geocontract.Area{
		AreaCode: "DHK-DHM", AreaName: "Dhanmondi",
		DistrictCode: "DHK", DivisionCode: "DHA", DivisionName: "Dhaka",
	}}
}

func (s *stubGeo) ResolveArea(_ context.Context, p geocontract.Point) (geocontract.Area, error) {
	s.mu.Lock()
	s.seen = append(s.seen, p)
	s.mu.Unlock()
	if s.err != nil {
		return geocontract.Area{}, s.err
	}
	return s.area, nil
}

type seqIDs struct {
	mu sync.Mutex
	n  int
}

func (s *seqIDs) New(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return prefix + "_" + string(rune('a'+(s.n-1)%26))
}

func ctx() context.Context { return context.Background() }

func newAddresses(repo *memoryRepo, geo *stubGeo) *application.AddressUseCase {
	return application.NewAddressUseCase(repo, geo, &seqIDs{})
}

func request() application.AddressRequest {
	return application.AddressRequest{
		Label:          "Home",
		RecipientName:  "Ayesha",
		RecipientPhone: "01712345678",
		Line1:          "House 12, Road 7",
		Line2:          "Dhanmondi",
		Lat:            23.7461,
		Lng:            90.3742,
	}
}

// ---------------------------------------------------------------- profile

// A user who has never opened the profile screen still has a profile. Returning
// 404 would make every client handle "signed in but no profile" as a special
// case, and some would handle it wrong.
func TestAMissingProfileIsNotAnError(t *testing.T) {
	uc := application.NewProfileUseCase(newRepo())
	profile, err := uc.Get(ctx(), "usr_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if profile.UserID != "usr_1" || profile.Language != domain.DefaultLanguage {
		t.Errorf("profile = %+v", profile)
	}
	if profile.DisplayName() == "" {
		t.Error("an empty profile has no display name")
	}
}

// TestAPartialUpdateLeavesOtherFieldsAlone is why the request fields are
// pointers: a client updating only the name must not erase the email.
func TestAPartialUpdateLeavesOtherFieldsAlone(t *testing.T) {
	repo := newRepo()
	uc := application.NewProfileUseCase(repo)

	name, email := "Ayesha Rahman", "ayesha@example.com"
	if _, err := uc.Update(ctx(), "usr_1", application.UpdateRequest{Name: &name, Email: &email}); err != nil {
		t.Fatalf("first update: %v", err)
	}

	newName := "Ayesha R"
	updated, err := uc.Update(ctx(), "usr_1", application.UpdateRequest{Name: &newName})
	if err != nil {
		t.Fatalf("second update: %v", err)
	}
	if updated.Email != "ayesha@example.com" {
		t.Errorf("email = %q; updating the name erased it", updated.Email)
	}
	if updated.Name != "Ayesha R" {
		t.Errorf("name = %q", updated.Name)
	}
}

// Clearing is a different request from leaving alone, and both must work.
func TestAFieldCanBeClearedExplicitly(t *testing.T) {
	repo := newRepo()
	uc := application.NewProfileUseCase(repo)

	email := "ayesha@example.com"
	if _, err := uc.Update(ctx(), "usr_1", application.UpdateRequest{Email: &email}); err != nil {
		t.Fatalf("update: %v", err)
	}
	empty := ""
	cleared, err := uc.Update(ctx(), "usr_1", application.UpdateRequest{Email: &empty})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared.Email != "" {
		t.Errorf("email = %q, want it cleared", cleared.Email)
	}
}

func TestProfileValidationSurfacesAsAUsefulError(t *testing.T) {
	uc := application.NewProfileUseCase(newRepo())

	bad := "not-an-email"
	if _, err := uc.Update(ctx(), "usr_1", application.UpdateRequest{Email: &bad}); errs.CodeOf(err) != "invalid_email" {
		t.Errorf("bad email = %v, want invalid_email", err)
	}

	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	if _, err := uc.Update(ctx(), "usr_1", application.UpdateRequest{Name: &long}); errs.CodeOf(err) != "name_too_long" {
		t.Errorf("long name = %v, want name_too_long", err)
	}

	french := "fr"
	if _, err := uc.Update(ctx(), "usr_1", application.UpdateRequest{Language: &french}); errs.CodeOf(err) != "unsupported_language" {
		t.Errorf("bad language = %v, want unsupported_language", err)
	}
}

func TestProfileSurfacesStoreFailures(t *testing.T) {
	readFail := newRepo()
	readFail.profileReadErr = errStore
	if _, err := application.NewProfileUseCase(readFail).Get(ctx(), "usr_1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("read failure = %v, want unavailable", err)
	}

	writeFail := newRepo()
	writeFail.profileWriteErr = errStore
	name := "Ayesha"
	if _, err := application.NewProfileUseCase(writeFail).Update(ctx(), "usr_1",
		application.UpdateRequest{Name: &name}); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("write failure = %v, want unavailable", err)
	}
}

// ---------------------------------------------------------------- addresses

// TestAnAddressIsPlacedBeforeItIsSaved is the phase's acceptance criterion.
// Config resolution, pricing and dispatch all key off the area, so an unplaced
// address is an order that reaches checkout and cannot be priced.
func TestAnAddressIsPlacedBeforeItIsSaved(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	address, err := newAddresses(repo, geo).Add(ctx(), "usr_1", request())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !address.Placement.IsPlaced() {
		t.Fatalf("the saved address is unplaced: %+v", address.Placement)
	}
	if address.Placement.AreaCode != "DHK-DHM" || address.Placement.DivisionCode != "DHA" {
		t.Errorf("placement = %+v", address.Placement)
	}
	if len(geo.seen) != 1 || geo.seen[0].Lat != 23.7461 {
		t.Errorf("geo saw %+v; the pin must be what is resolved", geo.seen)
	}
}

// An address outside the service area is refused, carrying geo's own wording so
// there is one message rather than two.
func TestAnUnplaceableAddressIsRefused(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	geo.err = errs.New(errs.KindNotFound, "outside_service_area",
		"We do not deliver to this location yet.")

	_, err := newAddresses(repo, geo).Add(ctx(), "usr_1", request())
	if errs.CodeOf(err) != "outside_service_area" {
		t.Fatalf("error = %v, want the geo refusal passed through", err)
	}

	stored, _ := repo.Addresses(ctx(), "usr_1")
	if len(stored) != 0 {
		t.Error("an unplaceable address was saved anyway")
	}
}

// A customer with one address and no default would be asked to choose between
// one option at checkout.
func TestTheFirstAddressBecomesTheDefault(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	address, err := newAddresses(repo, geo).Add(ctx(), "usr_1", request())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !address.IsDefault {
		t.Error("the first address is not the default")
	}
}

func TestALaterAddressDoesNotStealTheDefault(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	first, err := uc.Add(ctx(), "usr_1", request())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := uc.Add(ctx(), "usr_1", request())
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.IsDefault {
		t.Error("a later address took the default without being asked")
	}

	stored, _ := repo.Addresses(ctx(), "usr_1")
	if len(stored) != 2 {
		t.Fatalf("stored %d addresses", len(stored))
	}
	for _, a := range stored {
		if a.ID == first.ID && !a.IsDefault {
			t.Error("the first address lost the default")
		}
	}
}

func TestAnAddressCanBeMadeDefaultOnCreation(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	first, _ := uc.Add(ctx(), "usr_1", request())
	req := request()
	req.MakeDefault = true
	second, err := uc.Add(ctx(), "usr_1", req)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !second.IsDefault {
		t.Error("make_default was ignored")
	}

	stored, _ := repo.Addresses(ctx(), "usr_1")
	for _, a := range stored {
		if a.ID == first.ID && a.IsDefault {
			t.Error("two addresses are now the default")
		}
	}
}

// TestEditingAnAddressRePlacesIt: a customer correcting a street name has
// usually nudged the pin too, and an address whose stored area no longer
// matches its location prices the wrong zone — silently.
func TestEditingAnAddressRePlacesIt(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	address, err := uc.Add(ctx(), "usr_1", request())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	geo.area = geocontract.Area{
		AreaCode: "DHK-GUL", AreaName: "Gulshan",
		DistrictCode: "DHK", DivisionCode: "DHA", DivisionName: "Dhaka",
	}
	moved := request()
	moved.Lat, moved.Lng = 23.7925, 90.4152

	updated, err := uc.Update(ctx(), "usr_1", address.ID, moved)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Placement.AreaCode != "DHK-GUL" {
		t.Errorf("placement = %+v, want it re-resolved", updated.Placement)
	}
	if !updated.IsDefault {
		t.Error("editing an address took away its default")
	}
}

func TestEditingAnUnknownAddressIsNotFound(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	_, err := newAddresses(repo, geo).Update(ctx(), "usr_1", "adr_nope", request())
	if errs.KindOf(err) != errs.KindNotFound || errs.CodeOf(err) != "address_not_found" {
		t.Errorf("error = %v, want address_not_found", err)
	}
}

// Deleting the default promotes another, so a user with addresses always has one.
func TestDeletingTheDefaultPromotesAnother(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	first, _ := uc.Add(ctx(), "usr_1", request())
	if _, err := uc.Add(ctx(), "usr_1", request()); err != nil {
		t.Fatalf("second: %v", err)
	}

	if err := uc.Delete(ctx(), "usr_1", first.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	stored, _ := repo.Addresses(ctx(), "usr_1")
	if len(stored) != 1 {
		t.Fatalf("stored %d addresses", len(stored))
	}
	if !stored[0].IsDefault {
		t.Error("no address is the default after deleting the old one")
	}
}

func TestDeletingTheLastAddressLeavesNothing(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	address, _ := uc.Add(ctx(), "usr_1", request())
	if err := uc.Delete(ctx(), "usr_1", address.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	stored, _ := repo.Addresses(ctx(), "usr_1")
	if len(stored) != 0 {
		t.Errorf("stored = %+v", stored)
	}
}

func TestDeletingAnUnknownAddressIsNotFound(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	err := newAddresses(repo, geo).Delete(ctx(), "usr_1", "adr_nope")
	if errs.CodeOf(err) != "address_not_found" {
		t.Errorf("error = %v", err)
	}
}

func TestSettingADefaultMovesItExactlyOnce(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	first, _ := uc.Add(ctx(), "usr_1", request())
	second, _ := uc.Add(ctx(), "usr_1", request())

	promoted, err := uc.SetDefault(ctx(), "usr_1", second.ID)
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if !promoted.IsDefault {
		t.Error("the promoted address is not the default")
	}

	stored, _ := repo.Addresses(ctx(), "usr_1")
	defaults := 0
	for _, a := range stored {
		if a.IsDefault {
			defaults++
			if a.ID != second.ID {
				t.Errorf("the default is %s, want %s", a.ID, second.ID)
			}
		}
		if a.ID == first.ID && a.IsDefault {
			t.Error("the old default was not cleared")
		}
	}
	if defaults != 1 {
		t.Errorf("%d defaults, want exactly 1", defaults)
	}
}

func TestSettingAnUnknownAddressAsDefaultIsNotFound(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	_, err := newAddresses(repo, geo).SetDefault(ctx(), "usr_1", "adr_nope")
	if errs.CodeOf(err) != "address_not_found" {
		t.Errorf("error = %v", err)
	}
}

func TestTheDefaultAddressIsReadable(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	added, _ := uc.Add(ctx(), "usr_1", request())
	got, err := uc.Default(ctx(), "usr_1")
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if got.ID != added.ID {
		t.Errorf("default = %s, want %s", got.ID, added.ID)
	}
}

// A user with no address gets a 404 with a message telling them what to do,
// because this is what checkout hits for a new customer.
func TestNoDefaultAddressIsAUsefulNotFound(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	_, err := newAddresses(repo, geo).Default(ctx(), "usr_1")
	if errs.KindOf(err) != errs.KindNotFound || errs.CodeOf(err) != "no_default_address" {
		t.Fatalf("error = %v", err)
	}
	if errs.MessageOf(err) == "" {
		t.Error("the message is empty; a new customer needs telling what to do")
	}
}

// Refusing rather than silently dropping the oldest: an address is something
// the customer typed, and quietly losing one is worse than saying the book is
// full.
func TestTheAddressBookHasALimit(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	for i := 0; i < 20; i++ {
		if _, err := uc.Add(ctx(), "usr_1", request()); err != nil {
			t.Fatalf("address %d: %v", i, err)
		}
	}
	_, err := uc.Add(ctx(), "usr_1", request())
	if errs.KindOf(err) != errs.KindConflict || errs.CodeOf(err) != "address_book_full" {
		t.Errorf("error = %v, want address_book_full", err)
	}

	stored, _ := repo.Addresses(ctx(), "usr_1")
	if len(stored) != 20 {
		t.Errorf("stored %d addresses, want the limit held", len(stored))
	}
}

func TestInvalidAddressDetailsAreRefusedWithUsefulCodes(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)

	cases := map[string]func(*application.AddressRequest){
		"invalid_pin":           func(r *application.AddressRequest) { r.Lat, r.Lng = 0, 0 },
		"address_line_required": func(r *application.AddressRequest) { r.Line1 = "  " },
		"recipient_required":    func(r *application.AddressRequest) { r.RecipientPhone = "" },
	}
	for wantCode, mutate := range cases {
		req := request()
		mutate(&req)
		if _, err := uc.Add(ctx(), "usr_1", req); errs.CodeOf(err) != wantCode {
			t.Errorf("want %s, got %v", wantCode, err)
		}
	}

	long := request()
	for i := 0; i < 300; i++ {
		long.Line1 += "x"
	}
	if _, err := uc.Add(ctx(), "usr_1", long); errs.CodeOf(err) != "address_too_long" {
		t.Errorf("long line = %v, want address_too_long", err)
	}
}

func TestAddressesSurfaceStoreFailures(t *testing.T) {
	geo := newGeo()

	listFail := newRepo()
	listFail.listErr = errStore
	uc := newAddresses(listFail, geo)
	if _, err := uc.List(ctx(), "usr_1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("list = %v", err)
	}
	if _, err := uc.Add(ctx(), "usr_1", request()); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("add = %v", err)
	}
	if _, err := uc.Update(ctx(), "usr_1", "adr_a", request()); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("update = %v", err)
	}
	if err := uc.Delete(ctx(), "usr_1", "adr_a"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("delete = %v", err)
	}
	if _, err := uc.SetDefault(ctx(), "usr_1", "adr_a"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("set default = %v", err)
	}

	defaultFail := newRepo()
	defaultFail.defaultErr = errStore
	if _, err := newAddresses(defaultFail, geo).Default(ctx(), "usr_1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("default = %v", err)
	}

	saveFail := newRepo()
	saveFail.saveErr = errStore
	if _, err := newAddresses(saveFail, geo).Add(ctx(), "usr_1", request()); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("save = %v", err)
	}
}

func TestDeleteSurfacesAWriteFailure(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)
	address, _ := uc.Add(ctx(), "usr_1", request())

	repo.deleteErr = errStore
	if err := uc.Delete(ctx(), "usr_1", address.ID); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("delete = %v", err)
	}
}

// The delete succeeds but the follow-up write that promotes a new default
// fails. The caller must be told: otherwise they believe the address is gone
// and the book is left with no default.
func TestDeleteSurfacesAFailureToPromoteANewDefault(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)
	address, _ := uc.Add(ctx(), "usr_1", request())
	if _, err := uc.Add(ctx(), "usr_1", request()); err != nil {
		t.Fatalf("second: %v", err)
	}

	repo.saveErr = errStore
	if err := uc.Delete(ctx(), "usr_1", address.ID); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("delete = %v", err)
	}
}

func TestUpdateSurfacesAWriteFailure(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)
	address, _ := uc.Add(ctx(), "usr_1", request())

	repo.saveErr = errStore
	if _, err := uc.Update(ctx(), "usr_1", address.ID, request()); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("update = %v", err)
	}
}

func TestSetDefaultSurfacesAWriteFailure(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)
	address, _ := uc.Add(ctx(), "usr_1", request())

	repo.saveErr = errStore
	if _, err := uc.SetDefault(ctx(), "usr_1", address.ID); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("set default = %v", err)
	}
}

func TestEditingSurfacesAGeoFailure(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)
	address, _ := uc.Add(ctx(), "usr_1", request())

	geo.err = errs.New(errs.KindUnavailable, "geo_lookup_failed", "Please try again.")
	if _, err := uc.Update(ctx(), "usr_1", address.ID, request()); errs.CodeOf(err) != "geo_lookup_failed" {
		t.Errorf("error = %v", err)
	}
}

func TestEditingWithInvalidDetailsIsRefused(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)
	address, err := uc.Add(ctx(), "usr_1", request())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	broken := request()
	broken.Line1 = "   "
	if _, err := uc.Update(ctx(), "usr_1", address.ID, broken); errs.CodeOf(err) != "address_line_required" {
		t.Errorf("error = %v, want address_line_required", err)
	}
}

func TestUpdatingAProfileSurfacesAReadFailure(t *testing.T) {
	repo := newRepo()
	repo.profileReadErr = errStore
	name := "Ayesha"
	_, err := application.NewProfileUseCase(repo).Update(ctx(), "usr_1",
		application.UpdateRequest{Name: &name})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}
