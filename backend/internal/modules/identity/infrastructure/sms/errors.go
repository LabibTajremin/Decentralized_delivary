package sms

import "errors"

// ErrNotForProduction is returned when the development sender is constructed in
// a production deployment.
var ErrNotForProduction = errors.New("the logging SMS sender must not be used in production")
