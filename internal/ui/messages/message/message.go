package message

import (
	"time"

	"github.com/diamondburned/cchat"
	"github.com/diamondburned/cchat-gtk/internal/humanize"
	"github.com/diamondburned/cchat-gtk/internal/ui/primitives"
	"github.com/diamondburned/cchat-gtk/internal/ui/primitives/menu"
	"github.com/diamondburned/cchat-gtk/internal/ui/rich"
	"github.com/diamondburned/cchat-gtk/internal/ui/rich/labeluri"
	"github.com/diamondburned/cchat-gtk/internal/ui/rich/parser/markup"
	"github.com/diamondburned/cchat/text"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
)

type Container interface {
	ID() string
	Time() time.Time
	AuthorID() string
	AuthorName() string
	AuthorMarkup() string
	AvatarURL() string // avatar
	Nonce() string

	UpdateAuthor(cchat.Author)
	UpdateAuthorName(text.Rich)
	UpdateContent(c text.Rich, edited bool)
	UpdateTimestamp(time.Time)
}

// FillContainer sets the container's contents to the one from MessageCreate.
func FillContainer(c Container, msg cchat.MessageCreate) {
	c.UpdateAuthor(msg.Author())
	c.UpdateContent(msg.Content(), false)
	c.UpdateTimestamp(msg.Time())
}

// RefreshContainer sets the container's contents to the one from
// GenericContainer. This is mainly used for transferring between different
// containers.
//
// Right now, this only works with Timestamp, as that's the only state tracked.
func RefreshContainer(c Container, gc *GenericContainer) {
	c.UpdateTimestamp(gc.time)
}

// GenericContainer provides a single generic message container for subpackages
// to use.
type GenericContainer struct {
	*gtk.Box
	row   *gtk.ListBoxRow // contains Box
	class string

	id         string
	time       time.Time
	authorID   string
	authorName string
	avatarURL  string // avatar
	nonce      string

	Timestamp *gtk.Label
	Username  *labeluri.Label
	Content   gtk.IWidget // conceal widget implementation

	contentBox  *gtk.Box // basically what is in Content
	ContentBody *labeluri.Label

	menuItems []menu.Item
}

var _ Container = (*GenericContainer)(nil)

var timestampCSS = primitives.PrepareClassCSS("message-time", `
	.message-time {
		opacity: 0.3;
		font-size: 0.8em;
		margin-top: 0.2em;
		margin-bottom: 0.2em;
	}
`)

var authorCSS = primitives.PrepareClassCSS("message-author", `
	.message-author {
		color: mix(@theme_bg_color, @theme_fg_color, 0.8);
	}
`)

// NewContainer creates a new message container with the given ID and nonce. It
// does not update the widgets, so FillContainer should be called afterwards.
func NewContainer(msg cchat.MessageCreate) *GenericContainer {
	c := NewEmptyContainer()
	c.id = msg.ID()
	c.time = msg.Time()
	c.nonce = msg.Nonce()
	c.authorID = msg.Author().ID()

	return c
}

func NewEmptyContainer() *GenericContainer {
	ts, _ := gtk.LabelNew("")
	ts.SetEllipsize(pango.ELLIPSIZE_MIDDLE)
	ts.SetXAlign(0.5) // centre align
	ts.SetVAlign(gtk.ALIGN_END)
	ts.Show()

	user := labeluri.NewLabel(text.Rich{})
	user.SetXAlign(0) // left align
	user.SetVAlign(gtk.ALIGN_START)
	user.SetTrackVisitedLinks(false)
	user.Show()

	ctbody := labeluri.NewLabel(text.Rich{})
	ctbody.SetVExpand(true)
	ctbody.SetHAlign(gtk.ALIGN_START)
	ctbody.SetEllipsize(pango.ELLIPSIZE_NONE)
	ctbody.SetLineWrap(true)
	ctbody.SetLineWrapMode(pango.WRAP_WORD_CHAR)
	ctbody.SetXAlign(0) // left align
	ctbody.SetSelectable(true)
	ctbody.SetTrackVisitedLinks(false)
	ctbody.Show()

	// Wrap the content label inside a content box.
	ctbox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	ctbox.SetHExpand(true)
	ctbox.PackStart(ctbody, false, false, 0)
	ctbox.Show()

	box, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	box.Show()

	row, _ := gtk.ListBoxRowNew()
	row.Add(box)
	row.Show()

	// Add CSS classes.
	primitives.AddClass(ts, "message-time")
	primitives.AddClass(row, "message-row")
	primitives.AddClass(user, "message-author")
	primitives.AddClass(ctbody, "message-content")
	timestampCSS(ts)
	authorCSS(ts)

	gc := &GenericContainer{
		Box: box,
		row: row,

		Timestamp:   ts,
		Username:    user,
		Content:     ctbox,
		contentBox:  ctbox,
		ContentBody: ctbody,

		// Time is important, as it is used to sort messages, so we have to be
		// careful with this.
		time: time.Now(),
	}

	// Bind the custom popup menu to the content label.
	gc.ContentBody.Connect("populate-popup", func(l *gtk.Label, m *gtk.Menu) {
		menu.MenuSeparator(m)
		menu.MenuItems(m, gc.menuItems)
	})

	return gc
}

// Row returns the internal list box row. It is used to satisfy MessageRow.
func (m *GenericContainer) Row() *gtk.ListBoxRow { return m.row }

// SetClass sets the internal row's class.
func (m *GenericContainer) SetClass(class string) {
	if m.class != "" {
		primitives.RemoveClass(m.row, m.class)
	}

	primitives.AddClass(m.row, class)
	m.class = class
}

// SetReferenceHighlighter sets the reference highlighter into the message.
func (m *GenericContainer) SetReferenceHighlighter(r labeluri.ReferenceHighlighter) {
	m.Username.SetReferenceHighlighter(r)
	m.ContentBody.SetReferenceHighlighter(r)
}

func (m *GenericContainer) ID() string {
	return m.id
}

func (m *GenericContainer) Time() time.Time {
	return m.time
}

func (m *GenericContainer) AuthorID() string {
	return m.authorID
}

func (m *GenericContainer) AuthorName() string {
	return m.authorName
}

func (m *GenericContainer) AuthorMarkup() string {
	return m.Username.Label.Label.GetLabel()
}

func (m *GenericContainer) AvatarURL() string {
	return m.avatarURL
}

func (m *GenericContainer) Nonce() string {
	return m.nonce
}

func (m *GenericContainer) UpdateTimestamp(t time.Time) {
	m.time = t
	m.Timestamp.SetText(humanize.TimeAgo(t))
	m.Timestamp.SetTooltipText(t.Format(time.Stamp))
}

func (m *GenericContainer) UpdateAuthor(author cchat.Author) {
	m.authorID = author.ID()
	m.avatarURL = author.Avatar()
	m.UpdateAuthorName(author.Name())
}

func (m *GenericContainer) UpdateAuthorName(name text.Rich) {
	cfg := markup.RenderConfig{}
	cfg.NoReferencing = true
	cfg.SetForegroundAnchor(m.ContentBody)

	m.authorName = name.String()
	m.Username.SetOutput(markup.RenderCmplxWithConfig(name, cfg))
}

func (m *GenericContainer) UpdateContent(content text.Rich, edited bool) {
	m.ContentBody.SetLabelUnsafe(content)

	if edited {
		markup := m.ContentBody.Output().Markup
		markup += " " + rich.Small("(edited)")
		m.ContentBody.SetMarkup(markup)
	}
}

// AttachMenu connects signal handlers to handle a list of menu items from
// the container.
func (m *GenericContainer) AttachMenu(newItems []menu.Item) {
	m.menuItems = newItems
}

// MenuItems returns the list of menu items for this message.
func (m *GenericContainer) MenuItems() []menu.Item {
	return m.menuItems
}

func (m *GenericContainer) Focusable() gtk.IWidget {
	return m.Content
}
