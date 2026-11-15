package caps

// Mark is one formatting tool in the suite.
type Mark string

const (
	Bold      Mark = "bold"
	Italic    Mark = "italic"
	Strike    Mark = "strike"
	Code      Mark = "code"
	CodeBlock Mark = "code_block"
	Link      Mark = "link"
	Heading   Mark = "heading"
	Quote     Mark = "quote"
	List      Mark = "list"
)

// All is the suite order.
func All() []Mark {
	return []Mark{Bold, Italic, Strike, Code, CodeBlock, Link, Heading, Quote, List}
}

// Caps is the set of marks a protocol can carry.
type Caps struct {
	Marks map[Mark]bool
}

func New(marks ...Mark) Caps {
	m := map[Mark]bool{}
	for _, mark := range marks {
		m[mark] = true
	}
	return Caps{Marks: m}
}

func Full() Caps { return New(All()...) }

func IRC() Caps { return New(Bold, Italic, Strike) }

func XMPP() Caps { return New(Bold, Italic, Strike, Code, CodeBlock, Quote) }

func (c Caps) Has(m Mark) bool { return c.Marks[m] }

// ToolGrey reports that the focused conversation cannot carry the mark.
func ToolGrey(focused Caps, m Mark) bool { return !focused.Has(m) }

// HeaderGrey reports that a visible pane should draw its header in the disabled color.
func HeaderGrey(pane Caps, hovered Mark) bool {
	return hovered != "" && !pane.Has(hovered)
}

// SpanGrey reports that a transcript span carrying hovered should use the disabled color.
func SpanGrey(span []Mark, pane Caps, hovered Mark) bool {
	if hovered == "" || pane.Has(hovered) {
		return false
	}
	for _, m := range span {
		if m == hovered {
			return true
		}
	}
	return false
}

func FromNames(names []string) Caps {
	var marks []Mark
	for _, n := range names {
		marks = append(marks, Mark(n))
	}
	return New(marks...)
}

func (c Caps) Names() []string {
	var out []string
	for _, m := range All() {
		if c.Has(m) {
			out = append(out, string(m))
		}
	}
	return out
}
