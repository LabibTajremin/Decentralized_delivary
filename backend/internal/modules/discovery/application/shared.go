// Package application holds discovery's use cases: what a customer sees from
// where they are standing, and how that widens when there is nothing there.
//
// Discovery owns no storage. Every fact it uses comes from geo, merchant,
// catalogue or config through external/ (05-architecture.md 2.5), which is
// what makes it a policy module rather than another table.
package application

import (
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// searchFanOut caps how many merchants one radius query pulls back before
// filtering. Generous enough that a type filter still fills a page, bounded so
// a customer in central Dhaka does not make the database materialise every
// shop in the division (2.9: no list long enough to need client pagination).
const searchFanOut = 120

// policyFrom builds the expansion ladder from a resolved settings snapshot.
//
// One snapshot, read once: a radius taken from one read and a fee from another
// is the inconsistency that only shows up under load.
func policyFrom(settings cfg.Settings) (domain.Policy, error) {
	baseRadius, err := settings.Int(cfg.BaseRadius)
	if err != nil {
		return domain.Policy{}, configError(err)
	}
	step, err := settings.Int(cfg.ExpansionStep)
	if err != nil {
		return domain.Policy{}, configError(err)
	}
	maxExpansions, err := settings.Int(cfg.MaxExpansions)
	if err != nil {
		return domain.Policy{}, configError(err)
	}
	minMerchants, err := settings.Int(cfg.MinMerchants)
	if err != nil {
		return domain.Policy{}, configError(err)
	}
	autoExpand, err := settings.Bool(cfg.AutoExpand)
	if err != nil {
		return domain.Policy{}, configError(err)
	}
	ceiling, err := settings.Bool(cfg.DivisionCeil)
	if err != nil {
		return domain.Policy{}, configError(err)
	}

	policy, err := domain.NewPolicy(
		float64(baseRadius), float64(step),
		int(maxExpansions), int(minMerchants),
		autoExpand, ceiling,
	)
	if err != nil {
		return domain.Policy{}, policyError(err)
	}
	return policy, nil
}

// configError turns a missing or malformed setting into a failure that names
// itself. A search that silently fell back to a hard-coded 5 km would be a
// misconfigured division nobody noticed until the orders stopped.
func configError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "discovery_config_unavailable",
		"We could not work out what is near you just now. Please try again.")
}

// policyError reports a configuration that is present but impossible.
func policyError(err error) error {
	if errors.Is(err, domain.ErrCeilingDisabled) {
		// Appendix B: this one can never be disabled. Refusing the search is
		// the correct outcome — a search with the ceiling off would show a
		// customer in Sylhet a restaurant in Khulna, which is the one thing D3
		// exists to prevent.
		return errs.Wrap(err, errs.KindInternal, "division_ceiling_disabled",
			"Search is unavailable in this area. Please try again later.")
	}
	return errs.Wrap(err, errs.KindInternal, "discovery_config_invalid",
		"Search is unavailable in this area. Please try again later.")
}

// placementOf restates a resolved area as the primitive config and the fee
// quoter both take.
func placementOf(a geo.Area) ports.Placement {
	return ports.Placement{
		AreaCode:     a.AreaCode,
		DistrictCode: a.DistrictCode,
		DivisionCode: a.DivisionCode,
	}
}

// configPlacement is the same restatement for config's own contract.
func configPlacement(a geo.Area) cfg.Placement {
	return cfg.Placement{
		AreaCode:     a.AreaCode,
		DistrictCode: a.DistrictCode,
		DivisionCode: a.DivisionCode,
	}
}
