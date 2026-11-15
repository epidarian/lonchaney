package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/caps"
	"lonchan/internal/fontsuite"
	"lonchan/internal/model"
)

// Suite is the hovering format toolbar.
type Suite struct {
	widget.BaseWidget
	ui *UI
}

func NewSuite(ui *UI) *Suite {
	s := &Suite{ui: ui}
	s.ExtendBaseWidget(s)
	return s
}

func (s *Suite) CreateRenderer() fyne.WidgetRenderer {
	order := s.ui.hub.Store.Config().Theme.MarkOrder
	if len(order) == 0 {
		for _, m := range caps.All() {
			order = append(order, string(m))
		}
	}
	row := container.NewHBox()
	ctx := s.ui.focusedConversation()
	svc, _ := s.ui.hub.Store.Service(ctx.ServiceID)
	focused := s.ui.hub.Caps(svc.Plugin)
	for _, name := range order {
		mark := caps.Mark(name)
		btn := newTool(name, caps.ToolGrey(focused, mark), func() {
			if caps.ToolGrey(focused, mark) {
				return
			}
			s.ui.applyMark(mark)
		}, func(hover bool) {
			if hover {
				s.ui.hovered = mark
			} else if s.ui.hovered == mark {
				s.ui.hovered = ""
			}
			s.ui.refresh()
		})
		pack, dir := s.ui.activePack()
		row.Add(s.ui.newFrame(pack, dir, "suite-tool", nil, btn))
	}
	return widget.NewSimpleRenderer(row)
}

type tool struct {
	widget.BaseWidget
	label string
	grey  bool
	tap   func()
	hover func(bool)
}

func newTool(label string, grey bool, tap func(), hover func(bool)) *tool {
	t := &tool{label: label, grey: grey, tap: tap, hover: hover}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tool) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 8
	col := theme.Color(theme.ColorNameForeground)
	if t.grey {
		col = theme.Color(theme.ColorNameDisabled)
	}
	text := canvas.NewText(markGlyph(t.label), col)
	text.Alignment = fyne.TextAlignCenter
	text.TextSize = 16
	text.TextStyle = fyne.TextStyle{Bold: t.label == "bold", Italic: t.label == "italic"}
	return &toolRenderer{bg: bg, text: text, objs: []fyne.CanvasObject{bg, text}}
}

func markGlyph(name string) string {
	switch name {
	case "bold":
		return "B"
	case "italic":
		return "I"
	case "strike":
		return "S"
	case "code":
		return "</>"
	case "code_block":
		return "{}"
	case "link":
		return "🔗"
	case "heading":
		return "H"
	case "quote":
		return "❝"
	case "list":
		return "☰"
	default:
		return "✦"
	}
}

func formatBar(ui *UI, ctx model.Context, entry *composer, send fyne.CanvasObject) fyne.CanvasObject {
	order := ui.hub.Store.Config().Theme.MarkOrder
	if len(order) == 0 {
		for _, m := range caps.All() {
			order = append(order, string(m))
		}
	}
	svc, _ := ui.hub.Store.Service(ctx.ServiceID)
	have := ui.hub.Caps(svc.Plugin)
	row := container.NewHBox()
	for _, name := range order {
		mark := caps.Mark(name)
		tool := newTool(name, caps.ToolGrey(have, mark), func() {
			if caps.ToolGrey(have, mark) {
				return
			}
			ui.composer = entry
			ui.applyMark(mark)
		}, func(hover bool) {
			if hover {
				ui.hovered = mark
			} else if ui.hovered == mark {
				ui.hovered = ""
			}
			ui.refresh()
		})
		row.Add(tool)
	}
	row.Add(layout.NewSpacer())
	row.Add(send)
	return row
}

func (t *tool) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (t *tool) Tapped(*fyne.PointEvent) {
	if t.tap != nil {
		t.tap()
	}
}
func (t *tool) MouseIn(*desktop.MouseEvent) {
	if t.hover != nil {
		t.hover(true)
	}
}
func (t *tool) MouseMoved(*desktop.MouseEvent) {}
func (t *tool) MouseOut() {
	if t.hover != nil {
		t.hover(false)
	}
}

type toolRenderer struct {
	bg   *canvas.Rectangle
	text *canvas.Text
	objs []fyne.CanvasObject
}

func (r *toolRenderer) Destroy() {}
func (r *toolRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	th := r.text.MinSize().Height
	r.text.Move(fyne.NewPos(0, (s.Height-th)/2))
	r.text.Resize(fyne.NewSize(s.Width, th))
}
func (r *toolRenderer) MinSize() fyne.Size           { return fyne.NewSize(32, 32) }
func (r *toolRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *toolRenderer) Refresh()                     {}

func (u *UI) showSuite(c *composer) {
	u.composer = c
	u.hideSuite()
	if u.win == nil {
		return
	}
	pack, dir := u.activePack()
	pop := widget.NewPopUp(u.newFrame(pack, dir, "suite", nil, NewSuite(u)), u.win.Canvas())
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(c)
	y := pos.Y - pop.MinSize().Height - 8
	if y < 0 {
		y = 0
	}
	pop.ShowAtPosition(fyne.NewPos(pos.X, y))
	u.suite = pop
}

func (u *UI) hideSuite() {
	if u.suite != nil {
		u.suite.Hide()
		u.suite = nil
	}
}

func (u *UI) applyMark(mark caps.Mark) {
	ctx := u.focusedConversation()
	if ctx.ID == "" {
		return
	}
	svc, _ := u.hub.Store.Service(ctx.ServiceID)
	if !u.hub.Caps(svc.Plugin).Has(mark) {
		return
	}
	c := u.composer
	if c == nil {
		text, _, _ := fontsuite.Wrap(ctx.Draft, len([]rune(ctx.Draft)), len([]rune(ctx.Draft)), mark)
		ctx.Draft = text
		_ = u.hub.Store.PutContext(ctx)
		u.refresh()
		return
	}
	a, b := plainSelection(c)
	if a != b {
		c.spans = append(c.spans, span{start: a, end: b, mark: mark})
	}
	ctx.Draft = c.wireText()
	_ = u.hub.Store.PutContext(ctx)
	c.poke()
}

func entrySelection(c *composer) (int, int) {
	sel := []rune(c.SelectedText())
	rs := []rune(c.Text)
	cur := c.CursorTextOffset()
	n := len(sel)
	if n == 0 || cur < 0 {
		return 0, 0
	}
	if cur >= n && string(rs[cur-n:cur]) == string(sel) {
		return cur - n, cur
	}
	if cur+n <= len(rs) && string(rs[cur:cur+n]) == string(sel) {
		return cur, cur + n
	}
	return 0, 0
}

func plainSelection(c *composer) (int, int) {
	a, b := c.pinA, c.pinB
	if a == b {
		a, b = entrySelection(c)
	}
	n := len([]rune(c.Text))
	if a < 0 {
		a = 0
	}
	if b > n {
		b = n
	}
	if a > b {
		a = b
	}
	return a, b
}

func caretIndex(text string, row, col int) int {
	lines := strings.Split(text, "\n")
	if row < 0 {
		row = 0
	}
	idx := 0
	for i := 0; i < row && i < len(lines); i++ {
		idx += len([]rune(lines[i])) + 1
	}
	if row < len(lines) {
		line := []rune(lines[row])
		if col > len(line) {
			col = len(line)
		}
		if col > 0 {
			idx += col
		}
	}
	return idx
}

func setCaret(c *composer, text string, caret int) {
	if caret < 0 {
		caret = 0
	}
	runes := []rune(text)
	if caret > len(runes) {
		caret = len(runes)
	}
	row, col := 0, 0
	for i, r := range runes {
		if i == caret {
			break
		}
		if r == '\n' {
			row++
			col = 0
			continue
		}
		col++
	}
	c.CursorRow = row
	c.CursorColumn = col
}
