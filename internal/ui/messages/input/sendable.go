package input

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/diamondburned/cchat"
	"github.com/diamondburned/cchat-gtk/internal/gts"
	"github.com/diamondburned/cchat-gtk/internal/log"
	"github.com/diamondburned/cchat-gtk/internal/ui/messages/input/attachment"
	"github.com/diamondburned/cchat/text"
	"github.com/pkg/errors"
	"github.com/twmb/murmur3"
)

// globalID used for atomically generating nonces.
var globalID uint64

// generateNonce creates a nonce that should prevent collision. This function
// will always return a 16-byte long string.
func (f *Field) generateNonce() string {
	raw := fmt.Sprintf(
		"cchat-gtk/%s/%X/%X",
		f.UserID, time.Now().UnixNano(), atomic.AddUint64(&globalID, 1),
	)

	h1, h2 := murmur3.StringSum128(raw)
	nonce := make([]byte, 8*2)
	binary.LittleEndian.PutUint64(nonce[0:8], h1)
	binary.LittleEndian.PutUint64(nonce[8:16], h2)

	return base64.RawURLEncoding.EncodeToString(nonce)
}

func (f *Field) sendInput() {
	if f.Sender == nil {
		return
	}

	// Get the input text and the reply ID.
	text := f.getText()

	// Are we editing anything?
	if id := f.editingID; f.Editable(id) && id != "" {
		go func() {
			if err := f.editor.Edit(id, text); err != nil {
				log.Error(errors.Wrap(err, "Failed to edit message"))
			}
		}()

		f.StopEditing()
		return
	}

	// Get the attachments.
	var attachments = f.Attachments.Files()

	// Don't send if the message is empty.
	if text == "" && len(attachments) == 0 {
		return
	}

	// Derive the author. Prefer the author of the current user from the message
	// buffer over the one in the username feed, unless we can't find any.
	var author = f.ctrl.Author(f.UserID)
	if author == nil {
		author = newAuthor(f)
	}

	f.SendMessage(SendMessageData{
		time:    time.Now().UTC(),
		content: text,
		author:  author,
		nonce:   f.generateNonce(),
		replyID: f.replyingID,
		files:   attachments,
	})

	// Clear the input field after sending.
	f.clearText()

	// Refocus the textbox.
	f.text.GrabFocus()
}

func (f *Field) SendMessage(data PresendMessage) {
	// presend message into the container through the controller
	var onErr = f.ctrl.AddPresendMessage(data)

	// Copy the sender to prevent race conditions.
	var sender = f.Sender
	gts.Async(func() (func(), error) {
		if err := sender.Send(data); err != nil {
			return func() { onErr(err) }, errors.Wrap(err, "Failed to send message")
		}
		return nil, nil
	})
}

// Files is a list of attachments.
type Files []attachment.File

// Attachments returns the list of files as a list of cchat attachments.
func (files Files) Attachments() []cchat.MessageAttachment {
	var attachments = make([]cchat.MessageAttachment, len(files))
	for i, file := range files {
		attachments[i] = file.AsAttachment()
	}
	return attachments
}

// SendMessageData contains what is to be sent in a message. It behaves
// similarly to a regular CreateMessage.
type SendMessageData struct {
	time    time.Time
	content string
	author  cchat.Author
	nonce   string
	replyID cchat.ID
	files   Files
}

var _ cchat.SendableMessage = (*SendMessageData)(nil)

// PresendMessage is an interface for any message about to be sent.
type PresendMessage interface {
	cchat.MessageHeader // returns nonce and time
	cchat.SendableMessage
	cchat.Noncer

	// These methods are reserved for internal use.

	Author() cchat.Author
	Files() []attachment.File
}

var _ PresendMessage = (*SendMessageData)(nil)

// ID returns a pseudo ID for internal use.
func (s SendMessageData) ID() string                 { return s.nonce }
func (s SendMessageData) Time() time.Time            { return s.time }
func (s SendMessageData) Content() string            { return s.content }
func (s SendMessageData) Author() cchat.Author       { return s.author }
func (s SendMessageData) AsNoncer() cchat.Noncer     { return s }
func (s SendMessageData) Nonce() string              { return s.nonce }
func (s SendMessageData) Files() []attachment.File   { return s.files }
func (s SendMessageData) AsAttacher() cchat.Attacher { return s.files }
func (s SendMessageData) AsReplier() cchat.Replier   { return s }
func (s SendMessageData) ReplyingTo() cchat.ID       { return s.replyID }

type sendableAuthor struct {
	id        cchat.ID
	name      text.Rich
	avatarURL string
}

func newAuthor(f *Field) sendableAuthor {
	return sendableAuthor{
		f.UserID,
		f.Username.GetLabel(),
		f.Username.GetIconURL(),
	}
}

var _ cchat.Author = (*sendableAuthor)(nil)

func (a sendableAuthor) ID() string      { return a.id }
func (a sendableAuthor) Name() text.Rich { return a.name }
func (a sendableAuthor) Avatar() string  { return a.avatarURL }
