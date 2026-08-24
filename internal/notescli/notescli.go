// Package notescli exposes persistent notes as JSON commands.
package notescli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/alexivison/scry/internal/notes"
	flag "github.com/spf13/pflag"
)

type Options struct {
	WorkingDir string
	ConfigDir  string
	SetupErr   error
	Stdout     io.Writer
	Stderr     io.Writer
}

func Run(args []string, options Options) int {
	if options.SetupErr != nil {
		return writeError(options, "storage", options.SetupErr.Error())
	}
	if len(args) == 0 {
		return writeError(options, "invalid_arguments", "note command is required")
	}
	switch args[0] {
	case "list":
		return runList(args[1:], options)
	case "add":
		return runAdd(args[1:], options)
	case "edit":
		return runEdit(args[1:], options)
	case "remove":
		return runRemove(args[1:], options)
	case "sync":
		return runSync(args[1:], options)
	default:
		return writeError(options, "invalid_arguments", fmt.Sprintf("unknown note command %q", args[0]))
	}
}

func runList(args []string, options Options) int {
	fs, worktree := newFlagSet("list", options)
	state := fs.String("state", "", "filter by state")
	if code, done := parse(fs, args, options); done {
		return code
	}
	if fs.NArg() != 0 {
		return writeError(options, "invalid_arguments", "list accepts no arguments")
	}
	store, canonical, code := openStore(*worktree, options)
	if code != 0 {
		return code
	}
	var filter *notes.State
	if fs.Changed("state") {
		value := notes.State(*state)
		filter = &value
	}
	listed, err := store.List(filter)
	if err != nil {
		return writeNotesError(options, err)
	}
	return writeJSON(options.Stdout, struct {
		Worktree string       `json:"worktree"`
		Notes    []notes.Note `json:"notes"`
	}{canonical, listed})
}

func runAdd(args []string, options Options) int {
	fs, worktree := newFlagSet("add", options)
	file := fs.String("file", "", "repository-relative file")
	line := fs.Int("line", 0, "source line")
	body := fs.String("body", "", "note text")
	author := fs.String("author", "", "note author")
	if code, done := parse(fs, args, options); done {
		return code
	}
	if fs.NArg() != 0 {
		return writeError(options, "invalid_arguments", "add accepts no arguments")
	}
	for _, name := range []string{"file", "line", "body", "author"} {
		if !fs.Changed(name) {
			return writeError(options, "invalid_arguments", "--"+name+" is required")
		}
	}
	store, canonical, code := openStore(*worktree, options)
	if code != 0 {
		return code
	}
	note, err := store.Add(notes.AddInput{File: *file, Line: *line, Body: *body, Author: notes.Author(*author)})
	if err != nil {
		return writeNotesError(options, err)
	}
	return writeNote(options, canonical, note)
}

func runEdit(args []string, options Options) int {
	fs, worktree := newFlagSet("edit", options)
	body := fs.String("body", "", "note text")
	file := fs.String("file", "", "repository-relative file")
	line := fs.Int("line", 0, "source line")
	state := fs.String("state", "", "note state")
	if code, done := parse(fs, args, options); done {
		return code
	}
	if fs.NArg() != 1 {
		return writeError(options, "invalid_arguments", "edit requires one note ID")
	}
	input := notes.EditInput{}
	if fs.Changed("body") {
		input.Body = body
	}
	if fs.Changed("file") {
		input.File = file
	}
	if fs.Changed("line") {
		input.Line = line
	}
	if fs.Changed("state") {
		value := notes.State(*state)
		input.State = &value
	}
	store, canonical, code := openStore(*worktree, options)
	if code != 0 {
		return code
	}
	note, err := store.Edit(fs.Arg(0), input)
	if err != nil {
		return writeNotesError(options, err)
	}
	return writeNote(options, canonical, note)
}

func runRemove(args []string, options Options) int {
	fs, worktree := newFlagSet("remove", options)
	if code, done := parse(fs, args, options); done {
		return code
	}
	if fs.NArg() != 1 {
		return writeError(options, "invalid_arguments", "remove requires one note ID")
	}
	store, canonical, code := openStore(*worktree, options)
	if code != 0 {
		return code
	}
	note, err := store.Remove(fs.Arg(0))
	if err != nil {
		return writeNotesError(options, err)
	}
	return writeNote(options, canonical, note)
}

func runSync(args []string, options Options) int {
	fs, worktree := newFlagSet("sync", options)
	if code, done := parse(fs, args, options); done {
		return code
	}
	if fs.NArg() != 0 {
		return writeError(options, "invalid_arguments", "sync accepts no arguments")
	}
	store, canonical, code := openStore(*worktree, options)
	if code != 0 {
		return code
	}
	result, err := store.Sync()
	if err != nil {
		return writeNotesError(options, err)
	}
	staled := result.Staled
	if staled == nil {
		staled = []notes.Note{}
	}
	return writeJSON(options.Stdout, struct {
		Worktree string       `json:"worktree"`
		Checked  int          `json:"checked"`
		Staled   []notes.Note `json:"staled"`
	}{canonical, result.Checked, staled})
}

func newFlagSet(name string, options Options) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet("scry note "+name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs, fs.String("worktree", options.WorkingDir, "local worktree")
}

func parse(fs *flag.FlagSet, args []string, options Options) (int, bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(output(options.Stdout))
			fs.PrintDefaults()
			return 0, true
		}
		return writeError(options, "invalid_arguments", err.Error()), true
	}
	return 0, false
}

func openStore(worktree string, options Options) (*notes.Store, string, int) {
	store, err := notes.NewStore(worktree, options.ConfigDir)
	if err != nil {
		return nil, "", writeNotesError(options, err)
	}
	return store, store.Worktree(), 0
}

func writeNote(options Options, worktree string, note notes.Note) int {
	return writeJSON(options.Stdout, struct {
		Worktree string     `json:"worktree"`
		Note     notes.Note `json:"note"`
	}{worktree, note})
}

func writeNotesError(options Options, err error) int {
	var noteErr *notes.Error
	if !errors.As(err, &noteErr) {
		return writeError(options, "storage", err.Error())
	}
	return writeError(options, noteErr.Code, noteErr.Message)
}

func writeError(options Options, code, message string) int {
	writeJSON(output(options.Stderr), struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{code, message}})
	if code == "busy" || code == "corrupt_ledger" || code == "storage" || code == "unsupported_platform" {
		return 1
	}
	return 2
}

func writeJSON(writer io.Writer, value any) int {
	if err := json.NewEncoder(output(writer)).Encode(value); err != nil {
		return 1
	}
	return 0
}

func output(writer io.Writer) io.Writer {
	if writer == nil {
		return io.Discard
	}
	return writer
}
