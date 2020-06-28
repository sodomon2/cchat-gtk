package cozy

import (
	"github.com/diamondburned/cchat"
	"github.com/diamondburned/cchat-gtk/internal/ui/messages/container"
	"github.com/diamondburned/cchat-gtk/internal/ui/messages/input"
	"github.com/diamondburned/cchat-gtk/internal/ui/messages/message"
	"github.com/gotk3/gotk3/gtk"
)

// Collapsed is a message that follows after FullMessage. It does not show
// the header, and the avatar is invisible.
type CollapsedMessage struct {
	// Author is still updated normally.
	*message.GenericContainer
}

func NewCollapsedMessage(msg cchat.MessageCreate) *CollapsedMessage {
	msgc := WrapCollapsedMessage(message.NewContainer(msg))
	msgc.Timestamp.SetXAlign(0.5) // middle align
	message.FillContainer(msgc, msg)
	return msgc
}

func WrapCollapsedMessage(gc *message.GenericContainer) *CollapsedMessage {
	// Set Timestamp's padding accordingly to Avatar's.
	gc.Timestamp.SetSizeRequest(AvatarSize, -1)
	gc.Timestamp.SetVAlign(gtk.ALIGN_START)
	gc.Timestamp.SetMarginStart(container.ColumnSpacing * 2)

	// Set Content's padding accordingly to FullMessage's main box.
	gc.Content.SetMarginEnd(container.ColumnSpacing * 2)

	return &CollapsedMessage{
		GenericContainer: gc,
	}
}

func (c *CollapsedMessage) Collapsed() bool { return true }

func (c *CollapsedMessage) Unwrap(grid *gtk.Grid) *message.GenericContainer {
	// Remove GenericContainer's widgets from the containers.
	grid.Remove(c.Timestamp)
	grid.Remove(c.Content)

	// Return after removing.
	return c.GenericContainer
}

func (c *CollapsedMessage) Attach(grid *gtk.Grid, row int) {
	container.AttachRow(grid, row, c.Timestamp, c.Content)
}

type CollapsedSendingMessage struct {
	message.PresendContainer
	CollapsedMessage
}

func NewCollapsedSendingMessage(msg input.PresendMessage) *CollapsedSendingMessage {
	var msgc = message.NewPresendContainer(msg)

	return &CollapsedSendingMessage{
		PresendContainer: msgc,
		CollapsedMessage: *WrapCollapsedMessage(msgc.GenericContainer),
	}
}
