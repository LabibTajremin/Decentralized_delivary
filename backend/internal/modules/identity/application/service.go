package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Service implements contract.IdentityContract.
type Service struct {
	signer   ports.TokenSigner
	sessions ports.SessionStore
}

// NewService wires the public service.
func NewService(signer ports.TokenSigner, sessions ports.SessionStore) *Service {
	return &Service{signer: signer, sessions: sessions}
}

// PrincipalFromToken verifies an access token.
func (s *Service) PrincipalFromToken(_ context.Context, accessToken string) (contract.Principal, error) {
	claims, err := s.signer.Verify(accessToken)
	if err != nil {
		return contract.Principal{}, errs.Wrap(err, errs.KindUnauthorized, "invalid_token",
			"Please sign in again.")
	}
	return contract.Principal{
		UserID:    claims.UserID,
		Role:      contract.Role(claims.Role),
		SessionID: claims.SessionID,
	}, nil
}

// RevokeUserSessions signs a user out on every device.
func (s *Service) RevokeUserSessions(ctx context.Context, userID string) error {
	if err := s.sessions.RevokeUserSessions(ctx, userID); err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
			"We could not sign that user out. Please try again.")
	}
	return nil
}
