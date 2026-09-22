package sms

import "errors"

// ErrNotForProduction is returned when the development sender is constructed in
// a production deployment.
var ErrNotForProduction = errors.New("the logging SMS sender must not be used in production")

// ErrDemoNotForProduction is returned when the demo sender is constructed in a
// production deployment.
//
// Its own error rather than sharing LogSender's, because the two are wrong in
// production for different reasons and an operator reading a startup failure
// should be told which one they configured.
var ErrDemoNotForProduction = errors.New("the demo SMS sender must not be used in production")
