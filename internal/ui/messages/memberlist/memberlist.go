package memberlist

import (
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/cchat"
	"github.com/diamondburned/cchat-gtk/internal/gts"
	"github.com/diamondburned/cchat-gtk/internal/ui/primitives"
	"github.com/diamondburned/cchat-gtk/internal/ui/primitives/roundimage"
	"github.com/diamondburned/cchat-gtk/internal/ui/rich"
	"github.com/diamondburned/cchat-gtk/internal/ui/rich/labeluri"
	"github.com/diamondburned/cchat-gtk/internal/ui/rich/parser/markup"
	"github.com/diamondburned/cchat/text"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
	"github.com/pkg/errors"
)

var MemberListWidth = 250

type Controller interface {
	MemberListUpdated(c *Container)
}

type Container struct {
	*gtk.Revealer
	Scroll *gtk.ScrolledWindow
	Main   *gtk.Box
	ctrl   Controller

	// states

	// map id -> *Section
	Sections map[string]*Section
	stop     func()

	eventQueue eventQueue
}

var memberListCSS = primitives.PrepareClassCSS("member-list", `
	.member-list {
		background-color: @theme_base_color;
	}
`)

func New(ctrl Controller) *Container {
	main, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	main.SetSizeRequest(250, -1)
	main.Show()
	memberListCSS(main)

	sw, _ := gtk.ScrolledWindowNew(nil, nil)
	sw.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	sw.Add(main)
	sw.Show()

	rev, _ := gtk.RevealerNew()
	rev.SetTransitionType(gtk.REVEALER_TRANSITION_TYPE_SLIDE_LEFT)
	rev.SetTransitionDuration(75)
	rev.SetRevealChild(false)
	rev.Add(sw)

	return &Container{
		Revealer: rev,
		Scroll:   sw,
		Main:     main,
		ctrl:     ctrl,
		Sections: map[string]*Section{},
	}
}

// IsEmpty returns whether or not the member view container is empty.
func (c *Container) IsEmpty() bool {
	return len(c.Sections) == 0
}

// Reset removes all old sections.
func (c *Container) Reset() {
	if c.stop != nil {
		c.stop()
		c.stop = nil
	}

	c.Revealer.SetRevealChild(false)
	c.Sections = map[string]*Section{}
}

// TryAsyncList tries to set the member list from the given server. It does type
// assertions and handles asynchronicity. Reset must be called before this.
func (c *Container) TryAsyncList(server cchat.Messenger) {
	ls := server.AsMemberLister()
	if ls == nil {
		return
	}

	gts.Async(func() (func(), error) {
		f, err := ls.ListMembers(context.Background(), c)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to list members")
		}

		return func() { c.stop = f }, nil
	})
}

func (c *Container) SetSections(sections []cchat.MemberSection) {
	c.eventQueue.Add(func() { c.SetSectionsUnsafe(sections) })
}

func (c *Container) SetMember(sectionID string, member cchat.ListMember) {
	c.eventQueue.Add(func() { c.SetMemberUnsafe(sectionID, member) })
}

func (c *Container) RemoveMember(sectionID string, id string) {
	c.eventQueue.Add(func() { c.RemoveMemberUnsafe(sectionID, id) })
}

type sectionInsert struct {
	front    *Section
	sections []*Section
}

func (c *Container) SetSectionsUnsafe(sections []cchat.MemberSection) {
	// Lazily invalidate the container. We could delegate removing old sections
	// to this function instead of Reset to not halt for too long.
	primitives.RemoveChildren(c.Main)

	var newSections = make([]*Section, len(sections))

	for i, section := range sections {
		sc, ok := c.Sections[section.ID()]
		if !ok {
			sc = NewSection(section, &c.eventQueue)
		} else {
			sc.Update(section.Name(), section.Total())
		}

		newSections[i] = sc
	}

	// Remove all old sections.
	for id := range c.Sections {
		delete(c.Sections, id)
	}

	// Insert new sections.
	for _, section := range newSections {
		c.Main.Add(section)
		c.Sections[section.ID] = section
	}

	c.ctrl.MemberListUpdated(c)
}

func (c *Container) SetMemberUnsafe(sectionID string, member cchat.ListMember) {
	if s, ok := c.Sections[sectionID]; ok {
		s.SetMember(member)
	}
}

func (c *Container) RemoveMemberUnsafe(sectionID string, id string) {
	if s, ok := c.Sections[sectionID]; ok {
		s.RemoveMember(id)
	}
}

type Section struct {
	*gtk.Box

	ID string

	// state
	name  text.Rich
	total int

	Header *rich.Label
	Body   *gtk.ListBox

	// map id -> *Member
	Members map[string]*Member
}

var sectionHeaderCSS = primitives.PrepareClassCSS("section-header", `
	.section-header {
		margin: 8px 12px;
		margin-bottom: 2px;
	}
`)

var sectionBodyCSS = primitives.PrepareClassCSS("section-body", `
	.section-body {
		background: inherit;
	}
`)

func NewSection(sect cchat.MemberSection, evq EventQueuer) *Section {
	header := rich.NewLabel(text.Rich{})
	header.Show()
	sectionHeaderCSS(header)

	body, _ := gtk.ListBoxNew()
	body.SetSelectionMode(gtk.SELECTION_NONE)
	body.SetActivateOnSingleClick(true)
	body.SetSortFunc(listSortNameAsc) // A-Z
	body.Show()
	sectionBodyCSS(body)

	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	box.PackStart(header, false, false, 0)
	box.PackStart(body, false, false, 0)
	box.Show()

	var members = map[string]*Member{}

	// On row click, show the mention popup if any.
	body.Connect("row-activated", func(_ *gtk.ListBox, r *gtk.ListBoxRow) {
		var i = r.GetIndex()
		// Cold path; we can afford searching in the map.
		for _, member := range members {
			if member.ListBoxRow.GetIndex() == i {
				member.Popup(evq)
			}
		}
	})

	section := &Section{
		ID:      sect.ID(),
		Box:     box,
		Header:  header,
		Body:    body,
		Members: members,
	}

	section.Update(sect.Name(), sect.Total())

	return section
}

func (s *Section) Update(name text.Rich, total int) {
	s.name = name
	s.total = total

	var content = s.name.Content
	if total > 0 {
		content += fmt.Sprintf("—%d", total)
	}

	s.Header.SetLabelUnsafe(text.Rich{
		Content:  content,
		Segments: s.name.Segments,
	})
}

func (s *Section) SetMember(member cchat.ListMember) {
	if m, ok := s.Members[member.ID()]; ok {
		m.Update(member)
		return
	}

	m := NewMember(member)
	m.Show()

	s.Members[member.ID()] = m
	s.Body.Add(m)
}

func (s *Section) RemoveMember(id string) {
	if member, ok := s.Members[id]; ok {
		s.Body.Remove(member)
		delete(s.Members, id)
	}
}

func listSortNameAsc(r1, r2 *gtk.ListBoxRow) int {
	n1, _ := r1.GetName()
	n2, _ := r2.GetName()

	switch {
	case n1 < n2:
		return -1
	case n1 > n2:
		return 1
	default:
		return 0
	}
}

type Member struct {
	*gtk.ListBoxRow
	Main *gtk.Box

	Avatar *rich.Icon
	Name   *gtk.Label
	output markup.RenderOutput

	parent *gtk.ListBox
}

const AvatarSize = 34

var memberRowCSS = primitives.PrepareClassCSS("member-row", `
	.member-row {
		min-height: 42px;
	}
`)

var memberBoxCSS = primitives.PrepareClassCSS("member-box", `
	.member-box {
		margin: 3px 10px;
	}
`)

var avatarMemberCSS = primitives.PrepareClassCSS("avatar-member", `
	.avatar-member {
		padding-right: 10px;
	}
`)

func NewMember(member cchat.ListMember) *Member {
	evb, _ := gtk.EventBoxNew()
	evb.AddEvents(int(gdk.EVENT_ENTER_NOTIFY) | int(gdk.EVENT_LEAVE_NOTIFY))
	evb.Show()

	img, _ := roundimage.NewStaticImage(evb, 0)
	img.Show()

	icon := rich.NewCustomIcon(img, AvatarSize)
	icon.SetPlaceholderIcon("user-info-symbolic", AvatarSize)
	icon.Show()
	avatarMemberCSS(icon)

	lbl, _ := gtk.LabelNew("")
	lbl.SetUseMarkup(true)
	lbl.SetXAlign(0)
	lbl.SetEllipsize(pango.ELLIPSIZE_END)
	lbl.Show()

	box, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	box.PackStart(icon, false, false, 0)
	box.PackStart(lbl, true, true, 0)
	box.Show()
	memberBoxCSS(box)

	evb.Add(box)

	r, _ := gtk.ListBoxRowNew()
	memberRowCSS(r)
	r.Add(evb)

	m := &Member{
		ListBoxRow: r,
		Main:       box,
		Avatar:     icon,
		Name:       lbl,
	}

	m.Update(member)

	return m
}

var noMentionLinks = markup.RenderConfig{
	NoMentionLinks: true,
	NoReferencing:  true,
}

func (m *Member) Update(member cchat.ListMember) {
	m.ListBoxRow.SetName(member.Name().Content)

	if iconer := member.AsIconer(); iconer != nil {
		m.Avatar.AsyncSetIconer(iconer, "Failed to get member list icon")
	}

	m.output = markup.RenderCmplxWithConfig(member.Name(), noMentionLinks)

	txt := strings.Builder{}
	txt.WriteString(fmt.Sprintf(
		`<span color="#%06X" size="large">●</span> %s`,
		statusColors(member.Status()), m.output.Markup,
	))

	if bot := member.Secondary(); !bot.IsEmpty() {
		txt.WriteByte('\n')
		txt.WriteString(fmt.Sprintf(
			`<span alpha="85%%"><sup>%s</sup></span>`,
			markup.Render(bot),
		))
	}

	m.Name.SetMarkup(txt.String())
}

// Popup pops up the mention popover if any.
func (m *Member) Popup(evq EventQueuer) {
	if len(m.output.Mentions) == 0 {
		return
	}

	p := labeluri.NewPopoverMentioner(m, m.output.Input, m.output.Mentions[0])
	if p == nil {
		return
	}

	// Unbounded concurrency is kind of bad. We should deal with
	// this in the future.
	evq.Activate()
	p.Connect("closed", func(interface{}) { evq.Deactivate() })

	p.SetPosition(gtk.POS_LEFT)
	p.Popup()
}

func statusColors(status cchat.Status) uint32 {
	switch status {
	case cchat.StatusOnline:
		return 0x43B581
	case cchat.StatusBusy:
		return 0xF04747
	case cchat.StatusIdle:
		return 0xFAA61A
	case cchat.StatusOffline:
		fallthrough
	default:
		return 0x747F8D
	}
}
