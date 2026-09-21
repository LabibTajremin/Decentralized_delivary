package domain

import "errors"

// Platform is what kind of device a token belongs to.
type Platform string

// The two platforms a device token can be registered for.
const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// Errors NewDeviceToken returns.
var (
	ErrNoPlatform    = errors.New("a device token needs a platform")
	ErrBadPlatform   = errors.New("platform must be ios or android")
	ErrNoDeviceToken = errors.New("a device registration needs a token")
	ErrNoDeviceUser  = errors.New("a device token needs an owner")
)

// DeviceToken is one device a user's app has asked to receive push on.
//
// One per user per platform: a phone re-registering after a reinstall
// replaces its own old token rather than accumulating a stale one nothing
// will ever clean up.
type DeviceToken struct {
	UserID   string
	Platform Platform
	Token    string
}

// NewDeviceToken validates a registration.
func NewDeviceToken(userID, platform, token string) (DeviceToken, error) {
	if userID == "" {
		return DeviceToken{}, ErrNoDeviceUser
	}
	if platform == "" {
		return DeviceToken{}, ErrNoPlatform
	}
	p := Platform(platform)
	if p != PlatformIOS && p != PlatformAndroid {
		return DeviceToken{}, ErrBadPlatform
	}
	if token == "" {
		return DeviceToken{}, ErrNoDeviceToken
	}
	return DeviceToken{UserID: userID, Platform: p, Token: token}, nil
}
