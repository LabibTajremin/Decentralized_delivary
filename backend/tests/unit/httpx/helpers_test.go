package httpx

import "errors"

// asErr is errors.As with the concrete type, kept in one place so the
// assertions above read as assertions rather than as plumbing.
func asErr(err error, target any) bool { return errors.As(err, target) }
