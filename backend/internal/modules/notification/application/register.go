package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
)

// RegisterDeviceUseCase records a device so push has somewhere to go.
type RegisterDeviceUseCase struct {
	repo ports.Repository
}

// NewRegisterDeviceUseCase wires the use case.
func NewRegisterDeviceUseCase(repo ports.Repository) *RegisterDeviceUseCase {
	return &RegisterDeviceUseCase{repo: repo}
}

// Register saves a device token, replacing whatever this user last
// registered on the same platform — a phone that reinstalled the app has a
// new token, and the old one would only ever fail to deliver from here on.
func (uc *RegisterDeviceUseCase) Register(ctx context.Context, userID, platform, token string) error {
	t, err := domain.NewDeviceToken(userID, platform, token)
	if err != nil {
		return notificationError(err)
	}
	if err := uc.repo.SaveDeviceToken(ctx, t); err != nil {
		return storageError(err)
	}
	return nil
}
