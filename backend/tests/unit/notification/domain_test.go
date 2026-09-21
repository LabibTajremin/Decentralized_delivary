package notification

import (
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
)

func TestNewNotificationValidates(t *testing.T) {
	if _, err := domain.NewNotification("ntf_1", "", "t", "b", domain.ChannelPush, domain.StatusSent, at); err != domain.ErrNoUser {
		t.Errorf("err = %v", err)
	}
	if _, err := domain.NewNotification("ntf_1", "usr_1", "", "b", domain.ChannelPush, domain.StatusSent, at); err != domain.ErrNoTitle {
		t.Errorf("err = %v", err)
	}
	if _, err := domain.NewNotification("ntf_1", "usr_1", "t", "", domain.ChannelPush, domain.StatusSent, at); err != domain.ErrNoBody {
		t.Errorf("err = %v", err)
	}
	n, err := domain.NewNotification("ntf_1", "usr_1", "t", "b", domain.ChannelPush, domain.StatusSent, at)
	if err != nil {
		t.Fatalf("NewNotification: %v", err)
	}
	if n.ID != "ntf_1" || n.UserID != "usr_1" || n.Channel != domain.ChannelPush || n.Status != domain.StatusSent {
		t.Fatalf("n = %+v", n)
	}
}

func TestNewDeviceTokenValidates(t *testing.T) {
	if _, err := domain.NewDeviceToken("", "ios", "tok"); err != domain.ErrNoDeviceUser {
		t.Errorf("err = %v", err)
	}
	if _, err := domain.NewDeviceToken("usr_1", "", "tok"); err != domain.ErrNoPlatform {
		t.Errorf("err = %v", err)
	}
	if _, err := domain.NewDeviceToken("usr_1", "windows-phone", "tok"); err != domain.ErrBadPlatform {
		t.Errorf("err = %v", err)
	}
	if _, err := domain.NewDeviceToken("usr_1", "ios", ""); err != domain.ErrNoDeviceToken {
		t.Errorf("err = %v", err)
	}
	tok, err := domain.NewDeviceToken("usr_1", "android", "tok_1")
	if err != nil {
		t.Fatalf("NewDeviceToken: %v", err)
	}
	if tok.UserID != "usr_1" || tok.Platform != domain.PlatformAndroid || tok.Token != "tok_1" {
		t.Fatalf("tok = %+v", tok)
	}
}
