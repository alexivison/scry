package panes

import (
	"github.com/alexivison/scry/internal/model"
)

// RenderPreview renders a tree-style changed file preview for the dashboard pane.
func RenderPreview(files []model.FileSummary, width, height int) string {
	rendered, _ := RenderFileList(files, 0, 0, width, height, true, FileListOpts{
		Cursor:     0,
		UseCursor:  true,
		HideCursor: true,
	})
	return rendered
}
