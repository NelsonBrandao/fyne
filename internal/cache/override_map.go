package cache

import (
	"reflect"
	"sync"
	"unsafe"
	"weak"

	"fyne.io/fyne/v2"
)

// overrideMap holds the theme scope of each overridden object without
// keeping the object alive, so an entry lasts exactly as long as its object.
//
// No renderer or canvas event says when an object is done with: a hidden
// widget's renderer expires while the content it draws is still in use, and a
// discarded view is simply never drawn again. Whether the object is still
// reachable is the only reliable answer, so entries are not deleted when a
// renderer or canvas expires, and forgetCollected drops the ones whose object
// has been collected.
//
// The scope must not reach its own object, or the entry keeps the object
// alive after all. A ThemeOverride's feature theme holds the override weakly
// for this reason.
type overrideMap struct {
	mu      sync.RWMutex
	entries map[overrideKey]overrideEntry

	// strong holds objects that cannot be referenced weakly: those that are
	// not pointers, or point to a zero-sized value. Neither occurs for a
	// CanvasObject in practice, as Move and Resize need a pointer receiver.
	strong map[fyne.CanvasObject]*overrideScope
}

type overrideKey struct {
	addr uintptr
	// typ tells apart two objects at one address, such as a struct and the
	// object embedded as its first field.
	typ reflect.Type
}

type overrideEntry struct {
	obj   weak.Pointer[byte]
	scope *overrideScope
}

func weakKey(o fyne.CanvasObject) (overrideKey, *byte, bool) {
	v := reflect.ValueOf(o)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Type().Elem().Size() == 0 {
		return overrideKey{}, nil, false
	}

	p := (*byte)(v.UnsafePointer())
	return overrideKey{addr: uintptr(unsafe.Pointer(p)), typ: v.Type()}, p, true
}

// Load returns the scope of an object, if it has one.
func (m *overrideMap) Load(o fyne.CanvasObject) (*overrideScope, bool) {
	if o == nil {
		return nil, false
	}

	key, p, ok := weakKey(o)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !ok {
		s, found := m.strong[o]
		return s, found
	}

	e, found := m.entries[key]
	// An entry whose object was collected may remain at a reused address
	// until the next sweep; its weak pointer no longer matches.
	if !found || e.obj.Value() != p {
		return nil, false
	}
	return e.scope, true
}

// Store sets the scope of an object.
func (m *overrideMap) Store(o fyne.CanvasObject, s *overrideScope) {
	if o == nil {
		return
	}

	key, p, ok := weakKey(o)
	m.mu.Lock()
	defer m.mu.Unlock()
	if !ok {
		if m.strong == nil {
			m.strong = make(map[fyne.CanvasObject]*overrideScope)
		}
		m.strong[o] = s
		return
	}

	if m.entries == nil {
		m.entries = make(map[overrideKey]overrideEntry)
	}
	m.entries[key] = overrideEntry{obj: weak.Make(p), scope: s}
}

// forgetCollected drops the entries of objects that have been collected.
func (m *overrideMap) forgetCollected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, e := range m.entries {
		if e.obj.Value() == nil {
			delete(m.entries, key)
		}
	}
}

// Len returns the number of entries, including any whose object has been
// collected since the last sweep.
func (m *overrideMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries) + len(m.strong)
}

// Clear drops every entry.
func (m *overrideMap) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.entries)
	clear(m.strong)
}
