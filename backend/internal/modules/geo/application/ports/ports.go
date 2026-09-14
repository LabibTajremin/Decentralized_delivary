// Package ports declares what the geo use cases need from the outside world.
//
// Every implementation lives in infrastructure/. No use case ever touches a
// database handle (05-architecture.md 2.3).
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
)

// MerchantLocation is a merchant as the geo module sees it: an identity and a
// point. Geo deliberately knows nothing else about a merchant — name, menu and
// opening hours belong to the merchant module.
type MerchantLocation struct {
	MerchantID string
	Location   domain.Coordinate
	Distance   domain.Distance
}

// GeoRepository reads administrative geometry and answers spatial questions.
//
// The interface is engine-neutral by design (ADR 0003): PostGIS is the
// supported implementation because ALG-01 needs a GiST index, but nothing in
// this signature says so.
type GeoRepository interface {
	// Divisions returns every division with its boundary.
	Divisions(ctx context.Context) ([]domain.Division, error)

	// DivisionContaining returns the division a point falls in, implementing
	// ALG-03. It returns domain.ErrUnknownDivision when the point is outside
	// every division.
	DivisionContaining(ctx context.Context, c domain.Coordinate) (domain.Division, error)

	// AreaContaining returns the finest administrative area for a point, which
	// is what config resolution keys off (Appendix B).
	AreaContaining(ctx context.Context, c domain.Coordinate) (domain.Area, error)

	// MerchantsWithinRadius implements ALG-01: every merchant within radius of
	// centre, nearest first, restricted to the given division so the D3 ceiling
	// is enforced in the query rather than filtered afterwards.
	MerchantsWithinRadius(ctx context.Context, centre domain.Coordinate, radius domain.Distance, division domain.DivisionCode, limit int) ([]MerchantLocation, error)

	// CountMerchantsWithinRadius returns only the count, which the expansion
	// decision (ALG-02) needs without paying to transfer the rows.
	CountMerchantsWithinRadius(ctx context.Context, centre domain.Coordinate, radius domain.Distance, division domain.DivisionCode) (int, error)

	// UpsertMerchantLocation records a merchant's point and whether it is
	// searchable, creating the row if it is new.
	UpsertMerchantLocation(ctx context.Context, m MerchantPoint) error

	// DeleteMerchantLocation removes a merchant from the spatial index.
	DeleteMerchantLocation(ctx context.Context, merchantID string) error
}

// MerchantPoint is a merchant location as it is written, placement included.
//
// Distinct from MerchantLocation, which is what a search reads back: a write
// carries the resolved division and area, and a read carries a distance. One
// struct doing both would have a field that is meaningless in half its uses.
type MerchantPoint struct {
	MerchantID string
	Location   domain.Coordinate
	Division   domain.DivisionCode
	// AreaCode may be empty: a point can fall inside a division but outside
	// every mapped area, and refusing the merchant for that would make
	// registration depend on how finely we have drawn the map (D1).
	AreaCode string
	Active   bool
}
