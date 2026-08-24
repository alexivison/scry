// Package notes stores private source notes for one local worktree.
package notes

import "time"

type Author string

const (
	AuthorUser  Author = "user"
	AuthorAgent Author = "agent"
)

type State string

const (
	StateOpen     State = "open"
	StateResolved State = "resolved"
	StateStale    State = "stale"
)

type Note struct {
	ID              string    `json:"id"`
	File            string    `json:"file"`
	Line            int       `json:"line"`
	LineFingerprint string    `json:"lineFingerprint"`
	Body            string    `json:"body"`
	Author          Author    `json:"author"`
	State           State     `json:"state"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type AddInput struct {
	File   string
	Line   int
	Body   string
	Author Author
}

type EditInput struct {
	Body  *string
	File  *string
	Line  *int
	State *State
}

type SyncResult struct {
	Checked int
	Staled  []Note
}

type Store struct {
	worktree   string
	ledgerPath string
	lockPath   string
}

func (s *Store) Worktree() string { return s.worktree }

type ledger struct {
	Version  int    `json:"version"`
	Worktree string `json:"worktree"`
	Notes    []Note `json:"notes"`
}

// Error reports a stable failure code for the command boundary.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func noteError(code, message string) error {
	return &Error{Code: code, Message: message}
}
