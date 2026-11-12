package cache

import (
	"strconv"
	"sync/atomic"

	"fyne.io/fyne/v2"
)

var (
	overrides     overrideMap
	overrideCount atomic.Uint32
)

type overrideScope struct {
	th      fyne.Theme
	cacheID string
}

// OverrideTheme allows an app to specify that a single object should use a different theme to the app.
// This should be used sparingly to avoid a jarring user experience.
// If the object is a container it will theme the children, if it is a canvas primitive it will do nothing.
//
// Since: 2.5
func OverrideTheme(o fyne.CanvasObject, th fyne.Theme) {
	id := overrideCount.Add(1)
	s := &overrideScope{th: th, cacheID: strconv.Itoa(int(id))}
	overrideThemeWithScope(o, s)
}

func OverrideThemeMatchingScope(o, parent fyne.CanvasObject) bool {
	scope, ok := overrides.Load(parent)
	if !ok { // not overridden in parent
		return false
	}

	overrideThemeWithScope(o, scope)
	return true
}

func WidgetScopeID(o fyne.CanvasObject) string {
	scope, ok := overrides.Load(o)
	if !ok {
		return ""
	}

	return scope.cacheID
}

func WidgetTheme(o fyne.CanvasObject) fyne.Theme {
	scope, ok := overrides.Load(o)
	if !ok {
		return nil
	}

	return scope.th
}

func overrideContainer(c *fyne.Container, s *overrideScope) {
	for _, o := range c.Objects {
		overrideThemeWithScope(o, s)
	}
}

func overrideThemeWithScope(o fyne.CanvasObject, s *overrideScope) {
	if _, ok := o.(interface{ SetDeviceIsMobile(bool) }); ok { // ThemeOverride without the import loop
		return // do not apply this theme over a new scope
	}

	switch c := o.(type) {
	case fyne.Widget:
		overrideWidget(c, s)
	case *fyne.Container:
		overrideContainer(c, s)
	default:
		overrides.Store(c, s)
	}
}

func overrideWidget(w fyne.Widget, s *overrideScope) {
	ResetThemeCaches()
	overrides.Store(w, s)

	r, ok := CachedRenderer(w)
	if !ok {
		Renderer(w) // a new renderer takes the widget's scope as it is made
		return
	}

	for _, o := range r.Objects() {
		overrideThemeWithScope(o, s)
	}
}

// reapplyScope themes the objects of a widget's new renderer with the
// widget's scope. A renderer can be made again after the old one expired,
// such as when a hidden widget is shown, and its new objects would otherwise
// take the app theme. Objects already in the scope, such as app content the
// renderer draws again, are left as they are.
func reapplyScope(objs []fyne.CanvasObject, s *overrideScope) {
	for _, o := range objs {
		if _, ok := o.(interface{ SetDeviceIsMobile(bool) }); ok { // ThemeOverride, as in overrideThemeWithScope
			continue
		}

		switch c := o.(type) {
		case fyne.Widget:
			if current, ok := overrides.Load(c); ok && current == s {
				continue
			}
			overrides.Store(c, s)
			if r, ok := CachedRenderer(c); ok {
				reapplyScope(r.Objects(), s)
			} else {
				Renderer(c) // a new renderer takes the scope as it is made
			}
		case *fyne.Container:
			reapplyScope(c.Objects, s)
		default:
			overrides.Store(c, s)
		}
	}
}
