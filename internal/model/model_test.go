package model

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err    error
		target error
	}{
		"ErrOversized":  {err: ErrOversized, target: ErrOversized},
		"ErrBinaryFile": {err: ErrBinaryFile, target: ErrBinaryFile},
		"ErrSubmodule":  {err: ErrSubmodule, target: ErrSubmodule},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.err, tc.target) {
				t.Errorf("errors.Is(%v, %v) = false", tc.err, tc.target)
			}
		})
	}
}
