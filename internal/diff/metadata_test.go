package diff

import (
	"context"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

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

// routeGit builds a mock runner that dispatches on the joined args string.
func routeGit(routes map[string]string) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if out, ok := routes[key]; ok {
			return out, nil
		}
		return "", gitErr(1, "unexpected: "+key)
	}
}

const diffRange = "abc123...def456"

func nameStatusCmd() string {
	return "diff --name-status -z -M " + diffRange
}

func numstatCmd() string {
	return "diff --numstat -z -M " + diffRange
}

func cmp() model.ResolvedCompare {
	return model.ResolvedCompare{DiffRange: diffRange}
}

func TestListFiles(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		runner  func(ctx context.Context, args ...string) (string, error)
		compare *model.ResolvedCompare // nil defaults to cmp()
		want    []model.FileSummary
		wantErr bool
	}{
		"simple modified file": {
			runner: routeGit(map[string]string{
				nameStatusCmd(): "M\x00main.go\x00",
				numstatCmd():    "10\t5\tmain.go\x00",
			}),
			want: []model.FileSummary{
				{Path: "main.go", Status: model.StatusModified, Additions: 10, Deletions: 5},
			},
		},
		"added deleted and type-change": {
			runner: routeGit(map[string]string{
				nameStatusCmd(): "A\x00new.txt\x00" +
					"D\x00old.txt\x00" +
					"T\x00changed.txt\x00",
				numstatCmd(): "30\t0\tnew.txt\x00" +
					"0\t20\told.txt\x00" +
					"1\t1\tchanged.txt\x00",
			}),
			want: []model.FileSummary{
				{Path: "new.txt", Status: model.StatusAdded, Additions: 30, Deletions: 0},
				{Path: "old.txt", Status: model.StatusDeleted, Additions: 0, Deletions: 20},
				{Path: "changed.txt", Status: model.StatusTypeChg, Additions: 1, Deletions: 1},
			},
		},
		"rename with score": {
			// name-status -z: R100\0old.txt\0new.txt\0
			// numstat -z rename: 5\t3\t\0old.txt\0new.txt\0
			runner: routeGit(map[string]string{
				nameStatusCmd(): "R100\x00old.txt\x00new.txt\x00",
				numstatCmd():    "5\t3\t\x00old.txt\x00new.txt\x00",
			}),
			want: []model.FileSummary{
				{
					Path: "new.txt", OldPath: "old.txt",
					Status: model.StatusRenamed, Additions: 5, Deletions: 3,
				},
			},
		},
		"copy with score": {
			runner: routeGit(map[string]string{
				nameStatusCmd(): "C075\x00src.go\x00dst.go\x00",
				numstatCmd():    "2\t0\t\x00src.go\x00dst.go\x00",
			}),
			want: []model.FileSummary{
				{
					Path: "dst.go", OldPath: "src.go",
					Status: model.StatusCopied, Additions: 2, Deletions: 0,
				},
			},
		},
		"binary file detection": {
			runner: routeGit(map[string]string{
				nameStatusCmd(): "M\x00image.png\x00",
				numstatCmd():    "-\t-\timage.png\x00",
			}),
			want: []model.FileSummary{
				{Path: "image.png", Status: model.StatusModified, IsBinary: true},
			},
		},
		"unmerged status": {
			runner: routeGit(map[string]string{
				nameStatusCmd(): "U\x00conflict.go\x00",
				numstatCmd():    "0\t0\tconflict.go\x00",
			}),
			want: []model.FileSummary{
				{Path: "conflict.go", Status: model.StatusUnmerged},
			},
		},
		"empty diff": {
			runner: routeGit(map[string]string{
				nameStatusCmd(): "",
				numstatCmd():    "",
			}),
			want: nil,
		},
		"missing numstat entry defaults to zero": {
			// name-status lists the file but numstat doesn't
			runner: routeGit(map[string]string{
				nameStatusCmd(): "M\x00orphan.go\x00",
				numstatCmd():    "",
			}),
			want: []model.FileSummary{
				{Path: "orphan.go", Status: model.StatusModified, Additions: 0, Deletions: 0},
			},
		},
		"mixed renames binaries and regular files": {
			runner: routeGit(map[string]string{
				nameStatusCmd(): "M\x00lib.go\x00" +
					"R090\x00old.go\x00new.go\x00" +
					"A\x00bin.dat\x00" +
					"D\x00gone.txt\x00",
				numstatCmd(): "4\t2\tlib.go\x00" +
					"10\t1\t\x00old.go\x00new.go\x00" +
					"-\t-\tbin.dat\x00" +
					"0\t15\tgone.txt\x00",
			}),
			want: []model.FileSummary{
				{Path: "lib.go", Status: model.StatusModified, Additions: 4, Deletions: 2},
				{Path: "new.go", OldPath: "old.go", Status: model.StatusRenamed, Additions: 10, Deletions: 1},
				{Path: "bin.dat", Status: model.StatusAdded, IsBinary: true},
				{Path: "gone.txt", Status: model.StatusDeleted, Additions: 0, Deletions: 15},
			},
		},
		"working tree mode uses base-only range": {
			runner: routeGit(map[string]string{
				"diff --name-status -z -M aaa111": "M\x00main.go\x00",
				"diff --numstat -z -M aaa111":     "10\t5\tmain.go\x00",
			}),
			compare: &model.ResolvedCompare{DiffRange: "aaa111", WorkingTree: true},
			want: []model.FileSummary{
				{Path: "main.go", Status: model.StatusModified, Additions: 10, Deletions: 5},
			},
		},
		"name-status git error": {
			runner: func(_ context.Context, args ...string) (string, error) {
				if args[1] == "--name-status" {
					return "", gitErr(128, "fatal: bad revision")
				}
				return "", nil
			},
			wantErr: true,
		},
		"numstat git error": {
			runner: func(_ context.Context, args ...string) (string, error) {
				if args[1] == "--numstat" {
					return "", gitErr(128, "fatal: bad revision")
				}
				// name-status succeeds
				return "M\x00file.go\x00", nil
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := cmp()
			if tc.compare != nil {
				c = *tc.compare
			}
			svc := &MetadataService{Runner: &mockRunner{fn: tc.runner}}
			got, err := svc.ListFiles(context.Background(), c)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d\n  got:  %+v\n  want: %+v", len(got), len(tc.want), got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] =\n  got  %+v\n  want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestListFilesPreservesOrder verifies emission order matches name-status output.
func TestListFilesPreservesOrder(t *testing.T) {
	t.Parallel()

	runner := routeGit(map[string]string{
		nameStatusCmd(): "M\x00c.go\x00" +
			"A\x00a.go\x00" +
			"D\x00b.go\x00",
		numstatCmd(): "1\t1\tc.go\x00" +
			"2\t0\ta.go\x00" +
			"0\t3\tb.go\x00",
	})

	svc := &MetadataService{Runner: &mockRunner{fn: runner}}
	got, err := svc.ListFiles(context.Background(), cmp())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPaths := []string{"c.go", "a.go", "b.go"}
	if len(got) != len(wantPaths) {
		t.Fatalf("len = %d, want %d", len(got), len(wantPaths))
	}
	for i, want := range wantPaths {
		if got[i].Path != want {
			t.Errorf("path[%d] = %q, want %q", i, got[i].Path, want)
		}
	}
}
