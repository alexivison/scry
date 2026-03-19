// Package review manages review state: patch cache and generation guards.
package review

import (
	"crypto/sha256"
	"fmt"

	"github.com/alexivison/scry/internal/model"
)

// CacheLookup returns a cached PatchLoadState if the path has a Loaded entry
// in the current generation. Failed entries are not cached so that the user
// can retry by pressing Enter again.
func CacheLookup(state model.AppState, path string) (model.PatchLoadState, bool) {
	ps, ok := state.Patches[path]
	if !ok {
		return model.PatchLoadState{}, false
	}
	if ps.Generation != state.CacheGeneration {
		return model.PatchLoadState{}, false
	}
	if ps.Status != model.LoadLoaded {
		return model.PatchLoadState{}, false
	}
	return ps, true
}

// CacheStore writes a completed load result into the cache.
func CacheStore(state *model.AppState, path string, patch *model.FilePatch, err error) {
	status := model.LoadLoaded
	if err != nil {
		status = model.LoadFailed
	}
	var hash string
	if patch != nil {
		hash = PatchContentHash(patch)
	}
	state.Patches[path] = model.PatchLoadState{
		Status:      status,
		Patch:       patch,
		Err:         err,
		Generation:  state.CacheGeneration,
		ContentHash: hash,
	}
}

// PatchContentHash computes a SHA-256 hash of the patch's diff lines.
func PatchContentHash(patch *model.FilePatch) string {
	if patch == nil {
		return ""
	}
	h := sha256.New()
	for _, hunk := range patch.Hunks {
		fmt.Fprintf(h, "H:%d,%d,%d,%d:%s\n", hunk.OldStart, hunk.OldLen, hunk.NewStart, hunk.NewLen, hunk.Header)
		for _, line := range hunk.Lines {
			fmt.Fprintf(h, "%s:%s\n", line.Kind, line.Text)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// MarkLoading sets the cache entry for path to Loading in the current generation.
func MarkLoading(state *model.AppState, path string) {
	state.Patches[path] = model.PatchLoadState{
		Status:     model.LoadLoading,
		Generation: state.CacheGeneration,
	}
}

// IsStaleGeneration returns true if the message generation does not match
// the current state generation.
func IsStaleGeneration(msgGen, stateGen int) bool {
	return msgGen != stateGen
}

// BumpGeneration increments the cache generation counter.
func BumpGeneration(state *model.AppState) {
	state.CacheGeneration++
}

// ClearPatches removes all entries from the patch cache.
func ClearPatches(state *model.AppState) {
	state.Patches = make(map[string]model.PatchLoadState)
}
