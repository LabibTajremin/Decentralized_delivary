package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
)

// Service implements contract.UserContract.
type Service struct {
	profiles  *ProfileUseCase
	addresses *AddressUseCase
}

// NewService wires the public service.
func NewService(profiles *ProfileUseCase, addresses *AddressUseCase) *Service {
	return &Service{profiles: profiles, addresses: addresses}
}

// Profile returns a user's profile.
func (s *Service) Profile(ctx context.Context, userID string) (contract.Profile, error) {
	profile, err := s.profiles.Get(ctx, userID)
	if err != nil {
		return contract.Profile{}, err
	}
	return toContractProfile(profile), nil
}

// DefaultAddress returns where to deliver by default.
func (s *Service) DefaultAddress(ctx context.Context, userID string) (contract.Address, error) {
	address, err := s.addresses.Default(ctx, userID)
	if err != nil {
		return contract.Address{}, err
	}
	return ToContractAddress(address), nil
}

// Address returns one of the user's addresses.
func (s *Service) Address(ctx context.Context, userID, addressID string) (contract.Address, error) {
	addresses, err := s.addresses.List(ctx, userID)
	if err != nil {
		return contract.Address{}, err
	}
	for _, a := range addresses {
		if a.ID == addressID {
			return ToContractAddress(a), nil
		}
	}
	return contract.Address{}, notFound(addressID)
}

func toContractProfile(p domain.Profile) contract.Profile {
	return contract.Profile{
		UserID:      p.UserID,
		Name:        p.Name,
		DisplayName: p.DisplayName(),
		Email:       p.Email,
		Language:    p.Language.String(),
	}
}

// ToContractAddress converts an address to its public form.
//
// Exported because transport renders the same shape: one conversion means the
// address a consuming module sees and the address the app shows are assembled
// by the same code, so they cannot drift apart.
func ToContractAddress(a domain.Address) contract.Address {
	return contract.Address{
		ID:             a.ID,
		Label:          a.DisplayLabel(),
		RecipientName:  a.RecipientName,
		RecipientPhone: a.RecipientPhone,
		Line1:          a.Line1,
		Line2:          a.Line2,
		Instructions:   a.Instructions,
		SingleLine:     a.SingleLine(),
		Lat:            a.Pin.Lat,
		Lng:            a.Pin.Lng,
		AreaCode:       a.Placement.AreaCode,
		AreaName:       a.Placement.AreaName,
		DistrictCode:   a.Placement.DistrictCode,
		DivisionCode:   a.Placement.DivisionCode,
	}
}
