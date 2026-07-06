package core

import (
	"errors"

	"github.com/siabroo/micra/core/internal/lifecycle"
)

// ErrAppAlreadyStarted is returned by App.Run or App.RunOnce when the
// receiver has already been driven through its lifecycle. App is
// single-use; create a new one for a fresh lifecycle.
var ErrAppAlreadyStarted = errors.New("app already started")

// ErrStopTimeout is joined into App.Run's return value when one or
// more Components' Stop methods did not return before their stopCtx
// deadline. The process is exiting anyway, so this is a warning of
// resource leak rather than a hard failure.
//
// Aliases the lifecycle package's sentinel so errors.Is works across
// the public/internal boundary.
var ErrStopTimeout = lifecycle.ErrStopTimeout

// ErrDuplicateComponent is returned by App.Register when a Component
// with the same Name has already been registered. Names must be unique
// within an App so that log tags and error messages remain unambiguous.
var ErrDuplicateComponent = errors.New("duplicate component name")
