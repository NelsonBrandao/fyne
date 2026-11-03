package cache

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"fyne.io/fyne/v2"
)

// dummyObject is a canvas primitive: an object with no renderer of its own.
type dummyObject struct {
	fyne.CanvasObject
}

// themedWidget draws the objects it is given, as a widget's renderer does.
type themedWidget struct {
	fyne.Widget
	objects []fyne.CanvasObject
}

func (w *themedWidget) CreateRenderer() fyne.WidgetRenderer {
	return &dummyWidgetRenderer{widget: &dummyWidget{}, objects: w.objects}
}

// holder draws content it does not own, as a Scroll or Card draws its Content.
type holder struct {
	fyne.Widget
	content fyne.CanvasObject
}

func (h *holder) CreateRenderer() fyne.WidgetRenderer {
	return &dummyWidgetRenderer{widget: &dummyWidget{}, objects: []fyne.CanvasObject{h.content}}
}

// maker draws a new object each time its renderer is made, as most widgets do.
type maker struct {
	fyne.Widget
	made fyne.CanvasObject
}

func (m *maker) CreateRenderer() fyne.WidgetRenderer {
	m.made = &dummyObject{}
	return &dummyWidgetRenderer{widget: &dummyWidget{}, objects: []fyne.CanvasObject{&fyne.Container{Objects: []fyne.CanvasObject{m.made}}}}
}

// expireRenderers lets every renderer not painted since now expire.
func expireRenderers(tm *timeMock, painted ...fyne.Widget) {
	tm.setTime(12, 0)
	for _, w := range painted {
		Renderer(w)
	}
	lastClean = tm.createTime(11, 0)
	Clean(true)
}

// A widget inside a still-visible override is hidden long enough for its
// renderer to expire, then shown again. The app-owned content it draws is
// the same objects, and should keep the override's theme.
func TestHiddenHolderKeepsItsContentsTheme(t *testing.T) {
	testClearAll()
	tm := &timeMock{}
	tm.setTime(10, 0)

	text := &dummyObject{}
	content := &fyne.Container{Objects: []fyne.CanvasObject{text}}
	inner := &holder{content: content}
	outer := &themedWidget{objects: []fyne.CanvasObject{inner}}
	th := &struct{ fyne.Theme }{}
	OverrideTheme(outer, th)
	assert.Same(t, th, WidgetTheme(text))

	expireRenderers(tm, outer) // inner hidden: only the outer override is painted

	Renderer(inner) // shown again: a new renderer, the same content
	assert.Same(t, th, WidgetTheme(text), "content lost its override's theme")
	assert.Same(t, th, WidgetTheme(inner))
}

// The objects a widget's new renderer makes, after the old one expired, take
// the widget's theme, as the first renderer's did.
func TestANewRenderersObjectsTakeTheWidgetsTheme(t *testing.T) {
	testClearAll()
	tm := &timeMock{}
	tm.setTime(10, 0)

	inner := &maker{}
	outer := &themedWidget{objects: []fyne.CanvasObject{inner}}
	th := &struct{ fyne.Theme }{}
	OverrideTheme(outer, th)
	first := inner.made
	assert.Same(t, th, WidgetTheme(first))

	expireRenderers(tm, outer)
	Renderer(inner)
	assert.NotSame(t, first, inner.made)
	assert.Same(t, th, WidgetTheme(inner.made))
}

// overrideDiscarded themes a widget drawing a container with a primitive,
// renders it, and lets every reference go.
//
//go:noinline
func overrideDiscarded() {
	w := &themedWidget{objects: []fyne.CanvasObject{&fyne.Container{Objects: []fyne.CanvasObject{&dummyObject{}}}}}
	OverrideTheme(w, &struct{ fyne.Theme }{})
}

// A discarded themed view leaves the overrides once it is collected, though
// nothing told the cache it was discarded.
func TestTheOverridesOfACollectedViewAreForgotten(t *testing.T) {
	testClearAll()
	tm := &timeMock{}
	tm.setTime(10, 0)

	overrideDiscarded()
	assert.Equal(t, 2, overrides.Len(), "the widget and the primitive in its renderer's container")

	expireRenderers(tm) // the renderer cache holds the widget until it expires
	assert.Zero(t, renderers.Len())
	runtime.GC()
	overrides.forgetCollected()
	assert.Zero(t, overrides.Len(), "a collected object kept its override")
}

// A themed primitive that is still in use keeps its override through a
// collection, though it was dropped from every canvas.
func TestALiveObjectKeepsItsOverrideThroughACollection(t *testing.T) {
	testClearAll()
	tm := &timeMock{}
	tm.setTime(10, 0)

	text := &dummyObject{}
	th := &struct{ fyne.Theme }{}
	OverrideTheme(text, th)
	SetCanvasForObject(text, &dummyCanvas{}, nil)

	tm.setTime(12, 0)
	lastClean = tm.createTime(11, 0)
	Clean(true)
	assert.Zero(t, canvases.Len())
	runtime.GC()
	overrides.forgetCollected()
	assert.Same(t, th, WidgetTheme(text))
	runtime.KeepAlive(text)
}

// Two objects at one address, a struct and the object embedded first in it,
// keep separate overrides.
func TestAnEmbeddedObjectKeepsItsOwnOverride(t *testing.T) {
	testClearAll()

	type wrapper struct {
		dummyObject
	}
	outer := &wrapper{}
	inner := &outer.dummyObject
	a, b := &struct{ fyne.Theme }{}, &struct{ fyne.Theme }{}
	OverrideTheme(outer, a)
	OverrideTheme(inner, b)
	assert.Same(t, a, WidgetTheme(outer))
	assert.Same(t, b, WidgetTheme(inner))
}
