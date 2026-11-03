package container

import (
	"runtime"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"

	"fyne.io/fyne/v2/internal/cache"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// discardThemeOverride builds a themed label, renders it, and returns weak
// pointers to the override and its content.
//
//go:noinline
func discardThemeOverride() (weak.Pointer[ThemeOverride], weak.Pointer[widget.Label]) {
	l := widget.NewLabel("themed")
	o := NewThemeOverride(l, test.Theme())
	cache.Renderer(o).Layout(o.MinSize())
	cache.DestroyRenderer(o)
	cache.DestroyRenderer(l)
	return weak.Make(o), weak.Make(l)
}

// The theme override's scope must not keep its own content, and so itself,
// alive once the app lets them go.
func TestADiscardedThemeOverrideIsCollected(t *testing.T) {
	test.NewTempApp(t)

	over, content := discardThemeOverride()
	runtime.GC()
	assert.Nil(t, content.Value(), "the content outlived its override")
	assert.Nil(t, over.Value(), "the override outlived its content")
}
