package config

import (
	"context"
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Service is what every other module sees. These tests go through the public
// contract only, using the string keys a consumer would write, so a mismatch
// between the contract constants and the registry shows up here.

func newService(repo *fakeRepo) contract.ConfigContract {
	return application.NewService(application.NewResolveUseCase(repo))
}

func TestServiceResolvesThroughThePublicContract(t *testing.T) {
	scope, _ := domain.NewScope(domain.LevelArea, "DHK-DHM")
	value, _ := domain.Money(6000)
	repo := &fakeRepo{overrides: []domain.Override{
		{Key: domain.PricingDeliveryBase, Scope: scope, Value: value},
	}}

	settings, err := newService(repo).Settings(context.Background(), contract.Placement{
		AreaCode: "DHK-DHM", DistrictCode: "DHK", DivisionCode: "DHA",
	})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}

	base, err := settings.Int(contract.PricingDeliveryBase)
	if err != nil || base != 6000 {
		t.Errorf("delivery base = %d, %v; want the area override", base, err)
	}
	radius, err := settings.Int(contract.DiscoveryBaseRadius)
	if err != nil || radius != 5000 {
		t.Errorf("base radius = %d, %v; want the default in metres", radius, err)
	}
	ceiling, err := settings.Bool(contract.DiscoveryDivisionCeil)
	if err != nil || !ceiling {
		t.Errorf("division ceiling = %v, %v; want it on", ceiling, err)
	}
	multiplier, err := settings.Ratio(contract.PricingExpansionMult)
	if err != nil || multiplier != 1.5 {
		t.Errorf("expansion multiplier = %v, %v", multiplier, err)
	}
}

// TestContractKeysMatchTheRegistry is the check that makes the constants safe
// to use: a consumer naming contract.PricingDeliveryBase must reach the same
// variable the registry defines, or it silently gets a default forever.
func TestContractKeysMatchTheRegistry(t *testing.T) {
	exported := []string{
		contract.DiscoveryBaseRadius, contract.DiscoveryExpansionStep,
		contract.DiscoveryMaxExpansions, contract.DiscoveryMinMerchants,
		contract.DiscoveryAutoExpand, contract.DiscoveryDivisionCeil,
		contract.PricingDeliveryBase, contract.PricingDeliveryPerKm,
		contract.PricingExpansionMult, contract.PricingFreeDelivery,
		contract.DispatchShortDistance, contract.DispatchLongDistance,
		contract.DispatchPartnerRadius, contract.DispatchAssignTimeout,
		contract.DispatchMaxConcurrent, contract.OrderCancellationWindow,
		contract.OrderCODLimit,
	}
	if len(exported) != len(domain.AllKeys()) {
		t.Fatalf("contract exports %d keys, the registry has %d", len(exported), len(domain.AllKeys()))
	}

	known := make(map[domain.Key]bool, len(domain.AllKeys()))
	for _, k := range domain.AllKeys() {
		known[k] = true
	}
	for _, key := range exported {
		if !known[domain.Key(key)] {
			t.Errorf("contract exports %q, which the registry does not define", key)
		}
	}
}

func TestServiceSurfacesAResolutionFailure(t *testing.T) {
	repo := &fakeRepo{loadErr: errors.New("connection refused")}
	_, err := newService(repo).Settings(context.Background(), contract.Placement{})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

func TestSettingsRejectsTheWrongTypeAndUnknownKeys(t *testing.T) {
	settings, err := newService(&fakeRepo{}).Settings(context.Background(), contract.Placement{})
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if _, err := settings.Int(contract.DiscoveryAutoExpand); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Int on a bool = %v", err)
	}
	if _, err := settings.Bool(contract.PricingDeliveryBase); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Bool on money = %v", err)
	}
	if _, err := settings.Ratio(contract.PricingDeliveryBase); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Ratio on money = %v", err)
	}
	if _, err := settings.Int("not.a.key"); !errors.Is(err, domain.ErrUnknownKey) {
		t.Errorf("Int on an unknown key = %v", err)
	}
}
