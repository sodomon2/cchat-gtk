package message

import (
	"fmt"
	"html"

	"github.com/diamondburned/cchat-gtk/internal/humanize"
	"github.com/diamondburned/cchat-gtk/internal/ui/messages/input"
	"github.com/diamondburned/cchat-gtk/internal/ui/messages/input/attachment"
	"github.com/diamondburned/cchat-gtk/internal/ui/primitives"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
)

var EmptyContentPlaceholder = fmt.Sprintf(
	`<span alpha="25%%">%s</span>`, html.EscapeString("<empty>"),
)

type PresendContainer interface {
	SetDone(id string)
	SetLoading()
	SetSentError(err error)
}

// PresendGenericContainer is the generic container with extra methods
// implemented for stateful mutability of the generic message container.
type GenericPresendContainer struct {
	*GenericContainer

	// states; to be cleared on SetDone()
	presend input.PresendMessage
	uploads *attachment.MessageUploader
}

var _ PresendContainer = (*GenericPresendContainer)(nil)

func NewPresendContainer(msg input.PresendMessage) *GenericPresendContainer {
	c := NewEmptyContainer()
	c.nonce = msg.Nonce()
	c.UpdateAuthor(msg.Author())
	c.UpdateTimestamp(msg.Time())

	p := &GenericPresendContainer{
		GenericContainer: c,

		presend: msg,
		uploads: attachment.NewMessageUploader(msg.Files()),
	}
	p.SetLoading()

	return p
}

func (m *GenericPresendContainer) SetSensitive(sensitive bool) {
	m.Content.SetSensitive(sensitive)
}

func (m *GenericPresendContainer) SetDone(id string) {
	// Apply the received ID.
	m.id = id
	m.nonce = ""

	// Reset the state to be normal. Especially setting presend to nil should
	// free it from memory.
	m.presend = nil
	m.uploads = nil
	m.Content.SetTooltipText("")

	// Remove everything in the content box.
	m.clearBox()

	// Re-add the content label.
	m.Content.Add(m.ContentBody)

	// Set the sensitivity from false in SetLoading back to true.
	m.SetSensitive(true)
}

func (m *GenericPresendContainer) SetLoading() {
	m.SetSensitive(false)
	m.Content.SetTooltipText("")

	// Clear everything inside the content container.
	m.clearBox()

	// Add the content label.
	m.Content.Add(m.ContentBody)

	// Add the attachment progress box back in, if any.
	if m.uploads != nil {
		m.uploads.Show() // show the bars
		m.Content.Add(m.uploads)
	}

	if content := m.presend.Content(); content != "" {
		m.ContentBody.SetText(content)
	} else {
		// Use a placeholder content if the actual content is empty.
		m.ContentBody.SetMarkup(EmptyContentPlaceholder)
	}
}

func (m *GenericPresendContainer) SetSentError(err error) {
	m.SetSensitive(true) // allow events incl right clicks
	m.Content.SetTooltipText(err.Error())

	// Remove everything again.
	m.clearBox()

	// Re-add the label.
	m.Content.Add(m.ContentBody)

	// Style the label appropriately by making it red.
	var content = EmptyContentPlaceholder
	if m.presend != nil && m.presend.Content() != "" {
		content = m.presend.Content()
	}
	m.ContentBody.SetMarkup(fmt.Sprintf(`<span color="red">%s</span>`, content))

	// Add a smaller label indicating an error.
	errl, _ := gtk.LabelNew("")
	errl.SetXAlign(0)
	errl.SetLineWrap(true)
	errl.SetLineWrapMode(pango.WRAP_WORD_CHAR)
	errl.SetMarkup(fmt.Sprintf(
		`<span size="small" color="red"><b>Error:</b> %s</span>`,
		html.EscapeString(humanize.Error(err)),
	))

	errl.Show()
	m.Content.Add(errl)
}

// clearBox clears everything inside the content container.
func (m *GenericPresendContainer) clearBox() {
	primitives.RemoveChildren(m.Content)
}
