package watch

import (
	"context"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

// mockRunner dispatches RunGit calls to a user-supplied function.
type mockRunner struct {
	fn func(ctx context.Context, args ...string) (string, error)
}

func (m *mockRunner) RunGit(ctx context.Context, args ...string) (string, error) {
	return m.fn(ctx, args...)
}

var _ gitexec.GitRunner = (*mockRunner)(nil)

func gitErr(code int, stderr string, args ...string) error {
	return &gitexec.GitError{Args: args, ExitCode: code, Stderr: stderr}
}

// --- Fingerprint (committed-ref mode) ------------------------------------

func TestFingerprint_CommittedRef(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		baseRef     string
		runner      func(ctx context.Context, args ...string) (string, error)
		want        string
		wantErr     bool
	}{
		"concatenates HEAD and base SHAs": {
			baseRef: "origin/main",
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse HEAD origin/main":
					return "deadbeef\nfeedface\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "deadbeef:feedface",
		},
		"stable fingerprint on identical state": {
			baseRef: "origin/main",
			runner: func(_ context.Context, args ...string) (string, error) {
				return "abc123\ndef456\n", nil
			},
			want: "abc123:def456",
		},
		"rev-parse failure returns error": {
			baseRef: "origin/main",
			runner: func(_ context.Context, args ...string) (string, error) {
				return "", gitErr(128, "fatal: bad revision")
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fp := &Fingerprinter{Runner: &mockRunner{fn: tc.runner}}
			got, err := fp.Fingerprint(context.Background(), tc.baseRef, false)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Fingerprint = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFingerprint_SymbolicRefAdvance verifies that when the base ref
// (e.g., origin/main) advances between ticks, the fingerprint changes.
// This is the key property that requires passing a symbolic ref, not a
// pre-resolved SHA.
func TestFingerprint_SymbolicRefAdvance(t *testing.T) {
	t.Parallel()

	call := 0
	runner := &mockRunner{fn: func(_ context.Context, args ...string) (string, error) {
		call++
		if call == 1 {
			return "headSHA\nbaseSHA1\n", nil
		}
		return "headSHA\nbaseSHA2\n", nil // origin/main advanced after fetch
	}}

	fp := &Fingerprinter{Runner: runner}

	first, err := fp.Fingerprint(context.Background(), "origin/main", false)
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	second, err := fp.Fingerprint(context.Background(), "origin/main", false)
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if first == second {
		t.Errorf("fingerprints should differ after base ref advance: both %q", first)
	}
}

// --- Fingerprint (working-tree mode) -------------------------------------

func TestFingerprint_WorkingTree(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		baseRef string
		runner  func(ctx context.Context, args ...string) (string, error)
		want    string
		wantErr bool
	}{
		"incorporates diff output for working-tree mode": {
			baseRef: "base111",
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse HEAD base111":
					return "deadbeef\nfeedface\n", nil
				case "diff --name-only HEAD":
					return "file1.go\nfile2.go\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "deadbeef:feedface:file1.go\nfile2.go",
		},
		"empty diff output still produces fingerprint": {
			baseRef: "base111",
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse HEAD base111":
					return "deadbeef\nfeedface\n", nil
				case "diff --name-only HEAD":
					return "", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "deadbeef:feedface:",
		},
		"unstaged changes included in fingerprint": {
			baseRef: "base111",
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse HEAD base111":
					return "deadbeef\nfeedface\n", nil
				case "diff --name-only HEAD":
					return "unstaged.go\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "deadbeef:feedface:unstaged.go",
		},
		"diff failure returns error": {
			baseRef: "base111",
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse HEAD base111":
					return "deadbeef\nfeedface\n", nil
				case "diff --name-only HEAD":
					return "", gitErr(128, "fatal: bad object HEAD")
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fp := &Fingerprinter{Runner: &mockRunner{fn: tc.runner}}
			got, err := fp.Fingerprint(context.Background(), tc.baseRef, true)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Fingerprint = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- ShouldRefresh -------------------------------------------------------

func TestShouldRefresh(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		state          model.AppState
		newFingerprint string
		want           bool
	}{
		"changed fingerprint triggers refresh": {
			state: model.AppState{
				LastFingerprint: "old",
				RefreshInFlight: false,
			},
			newFingerprint: "new",
			want:           true,
		},
		"same fingerprint skips refresh": {
			state: model.AppState{
				LastFingerprint: "same",
				RefreshInFlight: false,
			},
			newFingerprint: "same",
			want:           false,
		},
		"in-flight refresh skips even with changed fingerprint": {
			state: model.AppState{
				LastFingerprint: "old",
				RefreshInFlight: true,
			},
			newFingerprint: "new",
			want:           false,
		},
		"empty last fingerprint triggers refresh on first check": {
			state: model.AppState{
				LastFingerprint: "",
				RefreshInFlight: false,
			},
			newFingerprint: "initial",
			want:           true,
		},
		"empty new fingerprint with non-empty last still triggers": {
			state: model.AppState{
				LastFingerprint: "old",
				RefreshInFlight: false,
			},
			newFingerprint: "",
			want:           true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := ShouldRefresh(&tc.state, tc.newFingerprint)
			if got != tc.want {
				t.Errorf("ShouldRefresh = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- CheckCmd ------------------------------------------------------------

func TestCheckCmd(t *testing.T) {
	t.Parallel()

	t.Run("returns FingerprintMsg with computed fingerprint", func(t *testing.T) {
		t.Parallel()

		runner := &mockRunner{fn: func(_ context.Context, args ...string) (string, error) {
			return "aaa\nbbb\n", nil
		}}

		cmd := CheckCmd(context.Background(), &Fingerprinter{Runner: runner}, "origin/main", false)
		msg := cmd()

		fpMsg, ok := msg.(FingerprintMsg)
		if !ok {
			t.Fatalf("msg type = %T, want FingerprintMsg", msg)
		}
		if fpMsg.Err != nil {
			t.Fatalf("unexpected error: %v", fpMsg.Err)
		}
		if fpMsg.Fingerprint != "aaa:bbb" {
			t.Errorf("Fingerprint = %q, want %q", fpMsg.Fingerprint, "aaa:bbb")
		}
	})

	t.Run("propagates error in FingerprintMsg", func(t *testing.T) {
		t.Parallel()

		runner := &mockRunner{fn: func(_ context.Context, args ...string) (string, error) {
			return "", gitErr(128, "fatal: bad revision")
		}}

		cmd := CheckCmd(context.Background(), &Fingerprinter{Runner: runner}, "origin/main", false)
		msg := cmd()

		fpMsg, ok := msg.(FingerprintMsg)
		if !ok {
			t.Fatalf("msg type = %T, want FingerprintMsg", msg)
		}
		if fpMsg.Err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
