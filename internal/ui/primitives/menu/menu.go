package menu

// TODO: move this package outside service

import (
	"github.com/diamondburned/cchat-gtk/internal/gts"
	"github.com/diamondburned/cchat-gtk/internal/ui/primitives"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

// LazyMenu is a menu with lazy-loaded capabilities.
type LazyMenu struct {
	items []Item
}

func NewLazyMenu(bindTo primitives.Connector) *LazyMenu {
	l := LazyMenu{}
	bindTo.Connect("button-press-event", l.popup)
	return &l
}

func (m *LazyMenu) popup(w gtk.IWidget, ev *gdk.Event) {
	// Is this a right click? Run the menu if yes.
	if gts.EventIsRightClick(ev) {
		m.PopupAtPointer(ev)
	}
}

func (m *LazyMenu) SetItems(items []Item) {
	m.items = items
}

func (m *LazyMenu) AddItems(items ...Item) {
	m.items = append(m.items, items...)
}

func (m *LazyMenu) AddSimpleItem(name string, fn func()) {
	m.AddItems(SimpleItem(name, fn))
}

func (m *LazyMenu) Reset() {
	m.items = nil
}

func (m *LazyMenu) PopupAtPointer(ev *gdk.Event) {
	// Do nothing if there are no menu items.
	if len(m.items) == 0 {
		return
	}

	menu, _ := gtk.MenuNew()
	MenuItems(menu, m.items)
	menu.PopupAtPointer(ev)
}

type MenuAppender interface {
	Append(gtk.IMenuItem)
}

var _ MenuAppender = (*gtk.Menu)(nil)

func MenuSeparator(menu MenuAppender) {
	s, _ := gtk.SeparatorMenuItemNew()
	s.Show()
	menu.Append(s)
}

func MenuItems(menu MenuAppender, items []Item) {
	for _, item := range items {
		menu.Append(item.ToMenuItem())
	}
}

// FindItemFunc iterates over the list of items and returns the first item with
// the matching name.
func FindItemFunc(items []Item, name string) func() {
	for _, item := range items {
		if item.Name == name {
			return item.Func
		}
	}
	return nil
}

type ToolbarInserter interface {
	Insert(gtk.IToolItem, int)
}

var _ ToolbarInserter = (*gtk.Toolbar)(nil)

func ToolbarSeparator(toolbar ToolbarInserter) {
	s, _ := gtk.SeparatorToolItemNew()
	s.Show()
	toolbar.Insert(s, -1)
}

// ToolbarItems insert the given items into the toolbar.
func ToolbarItems(toolbar ToolbarInserter, items []Item) {
	for _, item := range items {
		toolbar.Insert(item.ToToolButton(), -1)
	}
}

type Item struct {
	Icon  string
	Name  string
	Func  func()
	Extra func(*gtk.MenuItem)
}

func SimpleItem(name string, fn func()) Item {
	return Item{Name: name, Func: fn}
}

// func (item Item) ToModelButton() *gtk.ModelButton {
// 	b, _ := gtk.ModelButtonNew()
// 	b.SetLabel(action[0])
// 	b.SetActionName(action[1])
// 	b.Show()

// 	// Set the label's alignment in a hacky way.
// 	c, _ := b.GetChild()
// 	l := c.(LabelTweaker)
// 	l.SetUseMarkup(true)
// 	l.SetHAlign(gtk.ALIGN_START)
// }

func (item Item) ToMenuItem() *gtk.MenuItem {
	mb, _ := gtk.MenuItemNewWithLabel(item.Name)
	mb.Connect("activate", func(interface{}) { item.Func() })
	mb.Show()

	if item.Extra != nil {
		item.Extra(mb)
	}

	return mb
}

func (item Item) ToToolButton() *gtk.ToolButton {
	tb, _ := gtk.ToolButtonNew(nil, item.Name)
	tb.Connect("clicked", func(interface{}) { item.Func() })
	tb.Show()

	return tb
}
