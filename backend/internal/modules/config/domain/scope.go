package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Errors returned when building a scope.
var (
	// ErrMissingScopeCode means a non-global scope was given no code.
	ErrMissingScopeCode = errors.New("scope code is required")
	// ErrUnexpectedScopeCode means the global scope was given a code.
	ErrUnexpectedScopeCode = errors.New("the global scope has no code")
	// ErrUnknownLevel means the level is not one of the four.
	ErrUnknownLevel = errors.New("unknown scope level")
)

// Level is how specific a setting is.
//
// The numeric order is the resolution order: a lower number wins. That is not
// incidental — Resolve sorts by it, so adding a level between two others means
// renumbering here and nowhere else.
type Level uint8

const (
	// LevelArea is the most specific: one neighbourhood.
	LevelArea Level = iota
	// LevelDistrict covers every area in a district.
	LevelDistrict
	// LevelDivision covers every district in a division.
	LevelDivision
	// LevelGlobal is the fallback that always exists.
	LevelGlobal
)

// String renders the level for the admin API and the audit log.
func (l Level) String() string {
	switch l {
	case LevelArea:
		return "area"
	case LevelDistrict:
		return "district"
	case LevelDivision:
		return "division"
	case LevelGlobal:
		return "global"
	default:
		return "unknown"
	}
}

// ParseLevel reads a level back from its stored form.
func ParseLevel(s string) (Level, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "area":
		return LevelArea, nil
	case "district":
		return LevelDistrict, nil
	case "division":
		return LevelDivision, nil
	case "global":
		return LevelGlobal, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrUnknownLevel, s)
	}
}

// Scope identifies where a setting applies.
type Scope struct {
	Level Level
	// Code is the area, district or division code. Empty for global.
	Code string
}

// GlobalScope is the fallback scope.
var GlobalScope = Scope{Level: LevelGlobal}

// NewScope builds and validates a scope.
func NewScope(level Level, code string) (Scope, error) {
	code = strings.TrimSpace(code)
	switch level {
	case LevelGlobal:
		if code != "" {
			return Scope{}, fmt.Errorf("%w: got %q", ErrUnexpectedScopeCode, code)
		}
		return GlobalScope, nil
	case LevelArea, LevelDistrict, LevelDivision:
		if code == "" {
			return Scope{}, fmt.Errorf("%w: %s scope", ErrMissingScopeCode, level)
		}
		return Scope{Level: level, Code: code}, nil
	default:
		return Scope{}, fmt.Errorf("%w: %d", ErrUnknownLevel, level)
	}
}

// String renders the scope for logs and audit entries.
func (s Scope) String() string {
	if s.Level == LevelGlobal {
		return "global"
	}
	return s.Level.String() + ":" + s.Code
}

// Placement is where a decision is being made — the output of geo's
// ResolveArea, restated in config's own terms so the two modules stay
// independent.
type Placement struct {
	AreaCode     string
	DistrictCode string
	DivisionCode string
}

// Chain returns the scopes to consult, most specific first.
//
// This is the resolution order from Appendix B: area → district → division →
// global. A placement missing a level simply contributes no scope there, so a
// coordinate that resolved only as far as a division still gets an answer
// rather than an error.
func (p Placement) Chain() []Scope {
	chain := make([]Scope, 0, 4)
	if code := strings.TrimSpace(p.AreaCode); code != "" {
		chain = append(chain, Scope{Level: LevelArea, Code: code})
	}
	if code := strings.TrimSpace(p.DistrictCode); code != "" {
		chain = append(chain, Scope{Level: LevelDistrict, Code: code})
	}
	if code := strings.TrimSpace(p.DivisionCode); code != "" {
		chain = append(chain, Scope{Level: LevelDivision, Code: code})
	}
	return append(chain, GlobalScope)
}

// Override is an admin-set value at one scope.
type Override struct {
	Key   Key
	Scope Scope
	Value Value
	// Pinned stops the auto-tuner changing this variable in this scope. An
	// admin who has deliberately set a value does not want it tuned away.
	Pinned bool
}
