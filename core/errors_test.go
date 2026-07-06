package core

import (
	"errors"
	"testing"
)

func TestErrors_AreSentinels(t *testing.T) {
	wrapped := errors.Join(ErrAppAlreadyStarted, errors.New("wrap"))
	if !errors.Is(wrapped, ErrAppAlreadyStarted) {
		t.Error("ErrAppAlreadyStarted does not survive errors.Join")
	}

	wrapped2 := errors.Join(ErrStopTimeout, errors.New("wrap"))
	if !errors.Is(wrapped2, ErrStopTimeout) {
		t.Error("ErrStopTimeout does not survive errors.Join")
	}

	wrapped3 := errors.Join(ErrDuplicateComponent, errors.New("wrap"))
	if !errors.Is(wrapped3, ErrDuplicateComponent) {
		t.Error("ErrDuplicateComponent does not survive errors.Join")
	}
}

func TestErrors_HaveDescriptiveMessages(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{ErrAppAlreadyStarted, "app already started"},
		{ErrStopTimeout, "stop timeout exceeded"},
		{ErrDuplicateComponent, "duplicate component name"},
	}
	for _, c := range cases {
		if c.err.Error() != c.want {
			t.Errorf("got %q, want %q", c.err.Error(), c.want)
		}
	}
}
