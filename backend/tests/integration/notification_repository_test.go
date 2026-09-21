package integration

import (
	"context"
	"testing"
	"time"

	notificationdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
	notificationpg "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/infrastructure/persistence/postgres"
)

// withNotification runs a test inside a transaction that is always rolled
// back. Notification does not reference another module's schema — only the
// two tables it owns.
func withNotification(t *testing.T, fn func(ctx context.Context, repo *notificationpg.Repository)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	fn(ctx, notificationpg.New(tx))
}

func notificationClock() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func sampleNotification(t *testing.T, id, userID, title, body string, channel notificationdomain.Channel, status notificationdomain.Status) notificationdomain.Notification {
	t.Helper()
	n, err := notificationdomain.NewNotification(id, userID, title, body, channel, status, notificationClock())
	if err != nil {
		t.Fatalf("NewNotification: %v", err)
	}
	return n
}

func TestANotificationRoundTrips(t *testing.T) {
	withNotification(t, func(ctx context.Context, repo *notificationpg.Repository) {
		saved := sampleNotification(t, "ntf_1", "usr_1", "Title", "Body",
			notificationdomain.ChannelPush, notificationdomain.StatusSent)
		if err := repo.SaveNotification(ctx, saved); err != nil {
			t.Fatalf("SaveNotification: %v", err)
		}

		got, err := repo.NotificationsForUser(ctx, "usr_1", 10)
		if err != nil {
			t.Fatalf("NotificationsForUser: %v", err)
		}
		if len(got) != 1 || got[0].ID != "ntf_1" || got[0].Channel != notificationdomain.ChannelPush {
			t.Fatalf("got = %+v", got)
		}
		if !got[0].CreatedAt.Equal(saved.CreatedAt) {
			t.Errorf("CreatedAt = %v, want %v", got[0].CreatedAt, saved.CreatedAt)
		}
	})
}

// Newest first, scoped to the caller, capped at the limit.
func TestNotificationsForUserOrdersAndScopesAndCaps(t *testing.T) {
	withNotification(t, func(ctx context.Context, repo *notificationpg.Repository) {
		base := notificationClock()
		for i, id := range []string{"ntf_1", "ntf_2", "ntf_3"} {
			n, err := notificationdomain.NewNotification(id, "usr_1", "Title", "Body",
				notificationdomain.ChannelPush, notificationdomain.StatusSent, base.Add(time.Duration(i)*time.Second))
			if err != nil {
				t.Fatalf("NewNotification: %v", err)
			}
			if err := repo.SaveNotification(ctx, n); err != nil {
				t.Fatalf("SaveNotification: %v", err)
			}
		}
		if err := repo.SaveNotification(ctx, sampleNotification(t, "ntf_other", "usr_2", "Title", "Body",
			notificationdomain.ChannelSMS, notificationdomain.StatusSent)); err != nil {
			t.Fatalf("SaveNotification (other user): %v", err)
		}

		got, err := repo.NotificationsForUser(ctx, "usr_1", 2)
		if err != nil {
			t.Fatalf("NotificationsForUser: %v", err)
		}
		if len(got) != 2 || got[0].ID != "ntf_3" || got[1].ID != "ntf_2" {
			t.Fatalf("got = %+v, want the two newest for usr_1, newest first", got)
		}
	})
}

func TestNotificationsForUserWithNoneIsEmpty(t *testing.T) {
	withNotification(t, func(ctx context.Context, repo *notificationpg.Repository) {
		got, err := repo.NotificationsForUser(ctx, "usr_missing", 10)
		if err != nil {
			t.Fatalf("NotificationsForUser: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got = %+v", got)
		}
	})
}

func TestADeviceTokenRoundTrips(t *testing.T) {
	withNotification(t, func(ctx context.Context, repo *notificationpg.Repository) {
		tok, err := notificationdomain.NewDeviceToken("usr_1", "ios", "tok_1")
		if err != nil {
			t.Fatalf("NewDeviceToken: %v", err)
		}
		if err := repo.SaveDeviceToken(ctx, tok); err != nil {
			t.Fatalf("SaveDeviceToken: %v", err)
		}

		got, err := repo.DeviceTokensForUser(ctx, "usr_1")
		if err != nil {
			t.Fatalf("DeviceTokensForUser: %v", err)
		}
		if len(got) != 1 || got[0].Token != "tok_1" || got[0].Platform != notificationdomain.PlatformIOS {
			t.Fatalf("got = %+v", got)
		}
	})
}

// A second registration on the same platform replaces the first, rather
// than adding a second row nothing will ever clean up.
func TestSaveDeviceTokenUpsertsPerPlatform(t *testing.T) {
	withNotification(t, func(ctx context.Context, repo *notificationpg.Repository) {
		first, err := notificationdomain.NewDeviceToken("usr_1", "ios", "tok_old")
		if err != nil {
			t.Fatalf("NewDeviceToken: %v", err)
		}
		if err := repo.SaveDeviceToken(ctx, first); err != nil {
			t.Fatalf("SaveDeviceToken: %v", err)
		}
		second, err := notificationdomain.NewDeviceToken("usr_1", "ios", "tok_new")
		if err != nil {
			t.Fatalf("NewDeviceToken: %v", err)
		}
		if err := repo.SaveDeviceToken(ctx, second); err != nil {
			t.Fatalf("SaveDeviceToken: %v", err)
		}

		got, err := repo.DeviceTokensForUser(ctx, "usr_1")
		if err != nil {
			t.Fatalf("DeviceTokensForUser: %v", err)
		}
		if len(got) != 1 || got[0].Token != "tok_new" {
			t.Fatalf("got = %+v, want exactly one row with the newer token", got)
		}
	})
}

// Two platforms for the same user are two rows.
func TestSaveDeviceTokenKeepsBothPlatforms(t *testing.T) {
	withNotification(t, func(ctx context.Context, repo *notificationpg.Repository) {
		ios, err := notificationdomain.NewDeviceToken("usr_1", "ios", "tok_ios")
		if err != nil {
			t.Fatalf("NewDeviceToken: %v", err)
		}
		android, err := notificationdomain.NewDeviceToken("usr_1", "android", "tok_android")
		if err != nil {
			t.Fatalf("NewDeviceToken: %v", err)
		}
		if err := repo.SaveDeviceToken(ctx, ios); err != nil {
			t.Fatalf("SaveDeviceToken(ios): %v", err)
		}
		if err := repo.SaveDeviceToken(ctx, android); err != nil {
			t.Fatalf("SaveDeviceToken(android): %v", err)
		}

		got, err := repo.DeviceTokensForUser(ctx, "usr_1")
		if err != nil {
			t.Fatalf("DeviceTokensForUser: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got = %+v, want both platforms", got)
		}
	})
}

func TestDeviceTokensForUserWithNoneIsEmpty(t *testing.T) {
	withNotification(t, func(ctx context.Context, repo *notificationpg.Repository) {
		got, err := repo.DeviceTokensForUser(ctx, "usr_missing")
		if err != nil {
			t.Fatalf("DeviceTokensForUser: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got = %+v", got)
		}
	})
}

func TestNotificationRepositorySurfacesAClosedConnection(t *testing.T) {
	ctx := context.Background()
	conn := connect(t)
	if err := conn.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	repo := notificationpg.New(conn)

	if err := repo.SaveNotification(ctx, sampleNotification(t, "ntf_1", "usr_1", "T", "B",
		notificationdomain.ChannelPush, notificationdomain.StatusSent)); err == nil {
		t.Error("SaveNotification on a closed connection must fail")
	}
	if _, err := repo.NotificationsForUser(ctx, "usr_1", 10); err == nil {
		t.Error("NotificationsForUser on a closed connection must fail")
	}
	tok, err := notificationdomain.NewDeviceToken("usr_1", "ios", "tok_1")
	if err != nil {
		t.Fatalf("NewDeviceToken: %v", err)
	}
	if err := repo.SaveDeviceToken(ctx, tok); err == nil {
		t.Error("SaveDeviceToken on a closed connection must fail")
	}
	if _, err := repo.DeviceTokensForUser(ctx, "usr_1"); err == nil {
		t.Error("DeviceTokensForUser on a closed connection must fail")
	}
}

func TestNewFromPoolIsWiredForNotification(t *testing.T) {
	if repo := notificationpg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}
