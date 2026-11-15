package ui

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"lonchan/internal/caps"
	"lonchan/internal/model"
)

var md = goldmark.New(goldmark.WithExtensions(extension.Strikethrough, extension.Linkify))

// MessageBlock is one transcript row.
type MessageBlock struct {
	widget.BaseWidget
	Author   string
	When     time.Time
	Body     string
	Mine     bool
	ShowHead bool
	Caps     caps.Caps
	Hovered  caps.Mark

	relayout func(width float32)
}

func NewMessageBlock(msg model.LogMessage, mine, showHead bool, c caps.Caps, hovered caps.Mark) *MessageBlock {
	when, _ := time.Parse(time.RFC3339Nano, msg.Time)
	b := &MessageBlock{Author: msg.From, When: when, Body: msg.Body, Mine: mine, ShowHead: showHead, Caps: c, Hovered: hovered}
	b.ExtendBaseWidget(b)
	return b
}

func (b *MessageBlock) CreateRenderer() fyne.WidgetRenderer {
	authorColor := theme.ColorNameForeground
	if b.Mine {
		authorColor = theme.ColorNamePrimary
	}
	head := canvas.NewText(b.Author, theme.ColorForWidget(authorColor, b))
	head.TextStyle.Bold = true
	clock := canvas.NewText(b.When.Local().Format("15:04"), theme.ColorForWidget(theme.ColorNameDisabled, b))
	clock.TextSize = theme.SizeForWidget(theme.SizeNameCaptionText, b)
	if !b.ShowHead {
		head.Text = ""
		clock.Text = ""
	}
	if caps.HeaderGrey(b.Caps, b.Hovered) {
		head.Color = theme.ColorForWidget(theme.ColorNameDisabled, b)
	}
	rt := widget.NewRichText(segments(b.Body, b.Caps, b.Hovered)...)
	rt.Wrapping = fyne.TextWrapWord
	sep := canvas.NewRectangle(theme.ColorForWidget(theme.ColorNameSeparator, b))
	return &msgRenderer{block: b, head: head, clock: clock, body: rt, sep: sep}
}

type msgRenderer struct {
	block *MessageBlock
	head  *canvas.Text
	clock *canvas.Text
	body  *widget.RichText
	sep   *canvas.Rectangle
}

func (r *msgRenderer) Destroy() {}
func (r *msgRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.head, r.clock, r.body, r.sep}
}

// MinSize measures the body wrapped at the block's current width, so once the
// block has been laid out its reported height matches what it draws.
func (r *msgRenderer) MinSize() fyne.Size {
	if w := r.block.Size().Width; w > 0 && r.body.Size().Width != w {
		r.body.Resize(fyne.NewSize(w, r.body.MinSize().Height))
	}
	return fyne.NewSize(40, r.heightFor())
}

func (r *msgRenderer) heightFor() float32 {
	h := r.body.MinSize().Height + theme.Padding()*2 + 1
	if r.block.ShowHead {
		h += r.head.MinSize().Height + theme.InnerPadding()/2
	}
	return h
}

func (r *msgRenderer) Layout(size fyne.Size) {
	defer func() {
		// The first layout is the first time the body knows its width; if wrapping changed
		// its height, ask the transcript for another pass so rows do not overlap.
		if need := r.heightFor(); r.block.relayout != nil && (need-size.Height > 0.5 || size.Height-need > 0.5) {
			r.block.relayout(size.Width)
		}
	}()
	y := float32(0)
	if r.block.ShowHead {
		// Inset by the RichText's own padding so the author lines up with the body text.
		pad := theme.InnerPadding()
		r.head.Move(fyne.NewPos(pad, pad/2))
		r.head.Resize(fyne.NewSize(size.Width*0.7, r.head.MinSize().Height))
		cw := r.clock.MinSize().Width
		r.clock.Move(fyne.NewPos(size.Width-cw-pad, pad/2))
		r.clock.Resize(fyne.NewSize(cw, r.clock.MinSize().Height))
		y = r.head.MinSize().Height + pad/2
	}
	r.body.Move(fyne.NewPos(0, y))
	r.body.Resize(fyne.NewSize(size.Width, r.body.MinSize().Height))
	if bh := r.body.MinSize().Height; bh != r.body.Size().Height {
		r.body.Resize(fyne.NewSize(size.Width, bh))
	}
	r.sep.Move(fyne.NewPos(0, size.Height-1))
	r.sep.Resize(fyne.NewSize(size.Width, 1))
}
func (r *msgRenderer) Refresh() { canvas.Refresh(r.block) }

func segments(body string, c caps.Caps, hovered caps.Mark) []widget.RichTextSegment {
	src := []byte(body)
	doc := md.Parser().Parse(text.NewReader(src))
	var out []widget.RichTextSegment
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.Kind() {
		case ast.KindText, ast.KindString:
			var value string
			switch leaf := n.(type) {
			case *ast.Text:
				value = string(leaf.Segment.Value(src))
				if leaf.SoftLineBreak() || leaf.HardLineBreak() {
					value += " "
				}
			case *ast.String:
				value = string(leaf.Value)
			}
			marks := ancestorMarks(n)
			style := fyne.TextStyle{}
			for _, m := range marks {
				switch m {
				case caps.Bold:
					style.Bold = true
				case caps.Italic:
					style.Italic = true
				case caps.Code, caps.CodeBlock:
					style.Monospace = true
				}
			}
			colorName := theme.ColorNameForeground
			if caps.SpanGrey(marks, c, hovered) {
				colorName = theme.ColorNameDisabled
			}
			out = append(out, &widget.TextSegment{Text: value, Style: widget.RichTextStyle{Inline: true, TextStyle: style, ColorName: colorName}})
		case ast.KindFencedCodeBlock, ast.KindCodeBlock:
			if n.PreviousSibling() != nil {
				out = append(out, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyle{Inline: false}})
			}
			colorName := theme.ColorNameForeground
			if caps.SpanGrey([]caps.Mark{caps.CodeBlock}, c, hovered) {
				colorName = theme.ColorNameDisabled
			}
			lines := n.Lines()
			for i := 0; i < lines.Len(); i++ {
				if i > 0 {
					out = append(out, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyle{Inline: false}})
				}
				seg := lines.At(i)
				line := strings.TrimRight(string(seg.Value(src)), "\r\n")
				out = append(out, &widget.TextSegment{Text: line, Style: widget.RichTextStyle{Inline: true, TextStyle: fyne.TextStyle{Monospace: true}, ColorName: colorName}})
			}
			return ast.WalkSkipChildren, nil
		case ast.KindList, ast.KindListItem:
			if n.PreviousSibling() != nil {
				out = append(out, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyle{Inline: false}})
			}
			if n.Kind() == ast.KindListItem {
				bullet := "•  "
				if l, ok := n.Parent().(*ast.List); ok && l.IsOrdered() {
					idx := l.Start
					for p := n.PreviousSibling(); p != nil; p = p.PreviousSibling() {
						idx++
					}
					bullet = itoa(idx) + ".  "
				}
				out = append(out, &widget.TextSegment{Text: bullet, Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNamePlaceHolder}})
			}
		case ast.KindTextBlock, ast.KindParagraph:
			if n.PreviousSibling() != nil {
				out = append(out, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyle{Inline: false}})
			}
		}
		return ast.WalkContinue, nil
	})
	if len(out) == 0 {
		out = append(out, &widget.TextSegment{Text: body, Style: widget.RichTextStyle{Inline: true}})
	}
	return out
}

func ancestorMarks(n ast.Node) []caps.Mark {
	var marks []caps.Mark
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case ast.KindEmphasis:
			if em, ok := p.(*ast.Emphasis); ok && em.Level >= 2 {
				marks = append(marks, caps.Bold)
			} else {
				marks = append(marks, caps.Italic)
			}
		case ast.KindCodeSpan:
			marks = append(marks, caps.Code)
		case ast.KindFencedCodeBlock, ast.KindCodeBlock:
			marks = append(marks, caps.CodeBlock)
		case ast.KindHeading:
			marks = append(marks, caps.Heading)
		case ast.KindBlockquote:
			marks = append(marks, caps.Quote)
		case ast.KindList, ast.KindListItem:
			marks = append(marks, caps.List)
		case ast.KindLink, ast.KindAutoLink:
			marks = append(marks, caps.Link)
		default:
			if strings.Contains(strings.ToLower(p.Kind().String()), "strikethrough") {
				marks = append(marks, caps.Strike)
			}
		}
	}
	return marks
}

// Plain strips markdown markers for the selection preview line.
func Plain(markdown string) string {
	s := markdown
	for _, mark := range []string{"**", "~~", "`", "# ", "> ", "- ", "_", "[", "]", "(", ")"} {
		s = strings.ReplaceAll(s, mark, "")
	}
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
