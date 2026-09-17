package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/contract"
)

// Service implements contract.DispatchContract.
//
// A thin adapter over the offer use case. Order calls Offer when an order
// becomes ready and Withdraw when one is called off; tracking asks where the
// rider is.
type Service struct {
	repo   ports.Repository
	offers *OfferUseCase
}

// NewService wires the public service.
func NewService(repo ports.Repository, offers *OfferUseCase) *Service {
	return &Service{repo: repo, offers: offers}
}

// Offer puts an order on the board.
func (s *Service) Offer(ctx context.Context, req contract.OfferRequest) (contract.Job, error) {
	return s.offers.Offer(ctx, req)
}

// Withdraw takes a job off the board.
func (s *Service) Withdraw(ctx context.Context, orderID, reason string) error {
	return s.offers.Withdraw(ctx, orderID, reason)
}

// JobForOrder returns the delivery for an order, if there is one.
//
// The bool is false before an order is ready, which is most of its life — so a
// tracking screen asking about a freshly placed order gets "no rider yet"
// rather than a not-found it has to interpret.
func (s *Service) JobForOrder(ctx context.Context, orderID string) (contract.Job, bool, error) {
	job, found, err := s.repo.JobForOrder(ctx, orderID)
	if err != nil {
		return contract.Job{}, false, storageError(err)
	}
	if !found {
		return contract.Job{}, false, nil
	}
	return s.offers.toContract(ctx, job), true, nil
}

// PartnerOfUser resolves a signed-in account to its own partner id.
func (s *Service) PartnerOfUser(ctx context.Context, userID string) (string, bool, error) {
	partner, found, err := s.repo.PartnerOfUser(ctx, userID)
	if err != nil {
		return "", false, storageError(err)
	}
	if !found {
		return "", false, nil
	}
	return partner.ID, true, nil
}
