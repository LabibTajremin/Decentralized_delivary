package user

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Service is what order and dispatch see. They need a delivery address and
// somebody to hand the parcel to, and nothing more.

func newService(repo *memoryRepo, geo *stubGeo) contract.UserContract {
	return application.NewService(
		application.NewProfileUseCase(repo),
		newAddresses(repo, geo),
	)
}

func TestTheContractServesAProfile(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	profiles := application.NewProfileUseCase(repo)
	name := "Ayesha Rahman"
	if _, err := profiles.Update(ctx(), "usr_1", application.UpdateRequest{Name: &name}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := newService(repo, geo).Profile(ctx(), "usr_1")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if got.Name != "Ayesha Rahman" || got.DisplayName != "Ayesha Rahman" {
		t.Errorf("profile = %+v", got)
	}
	if got.Language != "bn" {
		t.Errorf("language = %q", got.Language)
	}
}

// DisplayName is never empty, so two consuming modules cannot invent two
// different greetings for the same nameless customer.
func TestTheContractAlwaysSuppliesADisplayName(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	got, err := newService(repo, geo).Profile(ctx(), "usr_nameless")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if got.DisplayName == "" {
		t.Error("a nameless profile has no display name")
	}
}

// TestTheContractCarriesThePreformattedAddress: every surface that shows an
// address shows the same string, because a client joining the parts itself will
// eventually join them differently (2.9).
func TestTheContractCarriesThePreformattedAddress(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	uc := newAddresses(repo, geo)
	added, err := uc.Add(ctx(), "usr_1", request())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	service := newService(repo, geo)
	got, err := service.DefaultAddress(ctx(), "usr_1")
	if err != nil {
		t.Fatalf("DefaultAddress: %v", err)
	}
	if got.ID != added.ID {
		t.Errorf("address = %s, want %s", got.ID, added.ID)
	}
	if got.SingleLine == "" {
		t.Error("no single line; a receipt and the app would compose their own")
	}
	if got.RecipientPhone != "01712345678" {
		t.Errorf("recipient phone = %q; the rider needs it", got.RecipientPhone)
	}
	// The placement travels with it, so a consumer can price without a second
	// call into geo.
	if got.AreaCode != "DHK-DHM" || got.DivisionCode != "DHA" {
		t.Errorf("placement = %+v", got)
	}
}

func TestTheContractServesOneAddressByID(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	added, err := newAddresses(repo, geo).Add(ctx(), "usr_1", request())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := newService(repo, geo).Address(ctx(), "usr_1", added.ID)
	if err != nil {
		t.Fatalf("Address: %v", err)
	}
	if got.ID != added.ID {
		t.Errorf("address = %+v", got)
	}
}

// The user id is part of the lookup, not a check afterwards: an accessor that
// can return another user's address is one careless call from leaking a home
// address and a phone number.
func TestTheContractWillNotServeAnotherUsersAddress(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	added, err := newAddresses(repo, geo).Add(ctx(), "usr_owner", request())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	_, err = newService(repo, geo).Address(ctx(), "usr_attacker", added.ID)
	if errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("error = %v, want not found", err)
	}
}

func TestTheContractSurfacesFailures(t *testing.T) {
	geo := newGeo()

	profileFail := newRepo()
	profileFail.profileReadErr = errStore
	if _, err := newService(profileFail, geo).Profile(ctx(), "usr_1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("profile = %v", err)
	}

	addressFail := newRepo()
	addressFail.defaultErr = errStore
	if _, err := newService(addressFail, geo).DefaultAddress(ctx(), "usr_1"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("default address = %v", err)
	}

	listFail := newRepo()
	listFail.listErr = errStore
	if _, err := newService(listFail, geo).Address(ctx(), "usr_1", "adr_a"); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("address = %v", err)
	}
}

// A user with no address gets a not-found the checkout screen can act on.
func TestTheContractReportsNoDefaultAddress(t *testing.T) {
	repo, geo := newRepo(), newGeo()
	_, err := newService(repo, geo).DefaultAddress(ctx(), "usr_1")
	if errs.CodeOf(err) != "no_default_address" {
		t.Errorf("error = %v", err)
	}
}

// The transport guard means an empty user id cannot arrive in practice. It is
// still wrapped rather than surfacing as an opaque 500 with no code to act on.
func TestAnEmptyUserIdIsAWrappedRefusal(t *testing.T) {
	repo, geo := newRepo(), newGeo()

	if _, err := application.NewProfileUseCase(repo).Get(context.Background(), ""); errs.CodeOf(err) != "invalid_profile" {
		t.Errorf("profile = %v, want a wrapped refusal", err)
	}
	if _, err := newAddresses(repo, geo).Add(context.Background(), "", request()); errs.CodeOf(err) != "invalid_address" {
		t.Errorf("address = %v, want a wrapped refusal", err)
	}
}
