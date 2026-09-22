package application

import "context"

// Service implements contract.NotificationContract.
type Service struct {
	notify *NotifyUseCase
}

// NewService wires the public service.
func NewService(notify *NotifyUseCase) *Service { return &Service{notify: notify} }

// Notify delivers a message, push first, falling back to SMS.
func (s *Service) Notify(ctx context.Context, userID, title, body string) error {
	return s.notify.Notify(ctx, userID, title, body)
}
