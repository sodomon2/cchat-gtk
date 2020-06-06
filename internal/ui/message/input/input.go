package input

import (
	"github.com/diamondburned/cchat"
	"github.com/diamondburned/cchat-gtk/internal/gts"
	"github.com/diamondburned/cchat-gtk/internal/log"
	"github.com/diamondburned/cchat-gtk/internal/ui/rich"
	"github.com/diamondburned/cchat/text"
	"github.com/gotk3/gotk3/gtk"
	"github.com/pkg/errors"
)

type usernameContainer struct {
	*gtk.Revealer
	label *rich.Label
}

func newUsernameContainer() *usernameContainer {
	label := rich.NewLabel(text.Rich{})
	label.SetMaxWidthChars(35)
	label.SetVAlign(gtk.ALIGN_START)
	label.SetMarginTop(inputmargin)
	label.SetMarginBottom(inputmargin)
	label.SetMarginStart(10)
	label.SetMarginEnd(10)
	label.Show()

	rev, _ := gtk.RevealerNew()
	rev.SetRevealChild(false)
	rev.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_SLIDE_RIGHT)
	rev.SetTransitionDuration(50)
	rev.Add(label)

	return &usernameContainer{rev, label}
}

// GetLabel is not thread-safe.
func (u *usernameContainer) GetLabel() text.Rich {
	return u.label.GetLabel()
}

// SetLabel is thread-safe.
func (u *usernameContainer) SetLabel(content text.Rich) {
	gts.ExecAsync(func() {
		u.label.SetLabelUnsafe(content)

		// Reveal if the name is not empty.
		u.SetRevealChild(!u.label.GetLabel().Empty())
	})
}

type Field struct {
	*gtk.Box
	username *usernameContainer

	TextScroll *gtk.ScrolledWindow
	text       *gtk.TextView
	buffer     *gtk.TextBuffer

	UserID string

	sender cchat.ServerMessageSender
	ctrl   Controller
}

type Controller interface {
	PresendMessage(msg PresendMessage) (onErr func(error))
}

const inputmargin = 4

func NewField(ctrl Controller) *Field {
	username := newUsernameContainer()
	username.Show()

	text, _ := gtk.TextViewNew()
	text.SetSensitive(false)
	text.SetWrapMode(gtk.WRAP_WORD_CHAR)
	text.SetProperty("top-margin", inputmargin)
	text.SetProperty("left-margin", inputmargin)
	text.SetProperty("right-margin", inputmargin)
	text.SetProperty("bottom-margin", inputmargin)
	text.Show()

	buf, _ := text.GetBuffer()

	sw, _ := gtk.ScrolledWindowNew(nil, nil)
	sw.Add(text)
	sw.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	sw.SetProperty("propagate-natural-height", true)
	sw.SetProperty("max-content-height", 150)
	sw.Show()

	box, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	box.PackStart(username, false, false, 0)
	box.PackStart(sw, true, true, 0)
	box.Show()

	field := &Field{
		Box:        box,
		username:   username,
		TextScroll: sw,
		text:       text,
		buffer:     buf,
		ctrl:       ctrl,
	}

	text.SetFocusHAdjustment(sw.GetHAdjustment())
	text.SetFocusVAdjustment(sw.GetVAdjustment())
	text.Connect("key-press-event", field.keyDown)

	return field
}

// SetSender changes the sender of the input field. If nil, the input will be
// disabled.
func (f *Field) SetSender(session cchat.Session, sender cchat.ServerMessageSender) {
	f.UserID = session.ID()

	// Does sender (aka Server) implement ServerNickname?
	var err error
	if nicknamer, ok := sender.(cchat.ServerNickname); ok {
		err = errors.Wrap(nicknamer.Nickname(f.username), "Failed to get nickname")
	} else {
		err = errors.Wrap(session.Name(f.username), "Failed to get username")
	}

	// Do a bit of trivial error handling.
	if err != nil {
		log.Warn(err)
	}

	// Set the sender.
	f.sender = sender
	f.text.SetSensitive(sender != nil) // grey if sender is nil

	// reset the input
	f.buffer.Delete(f.buffer.GetBounds())
}

// SendMessage yanks the text from the input field and sends it to the backend.
// This function is not thread-safe.
func (f *Field) SendMessage() {
	if f.sender == nil {
		return
	}

	var text = f.yankText()
	if text == "" {
		return
	}

	var sender = f.sender
	var data = NewSendMessageData(text, f.username.GetLabel(), f.UserID)

	// presend message into the container through the controller
	var done = f.ctrl.PresendMessage(data)

	go func() {
		err := sender.SendMessage(data)

		gts.ExecAsync(func() {
			done(err)
		})

		if err != nil {
			log.Error(errors.Wrap(err, "Failed to send message"))
		}
	}()
}

// yankText cuts the text from the input field and returns it.
func (f *Field) yankText() string {
	start, end := f.buffer.GetBounds()

	text, _ := f.buffer.GetText(start, end, false)
	if text != "" {
		f.buffer.Delete(start, end)
	}

	return text
}
