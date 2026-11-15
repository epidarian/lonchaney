package direct

import (
	"fmt"
	"strings"

	"lonchan/internal/model"
)

// FormatUser renders a signed user message.
func FormatUser(msg model.LogMessage) string {
	return fmt.Sprintf("<!-- lonchan id=%s from=%s time=%s sig=%s -->\n%s\n<!-- /lonchan -->\n",
		msg.ID, msg.From, msg.Time, msg.Sig, strings.TrimRight(msg.Body, "\n"))
}

// FormatControl renders a fenced control document.
func FormatControl(c model.Control) string {
	var b strings.Builder
	b.WriteString("```lonchan\n")
	fmt.Fprintf(&b, "type: %s\n", c.Type)
	if len(c.Marks) > 0 {
		fmt.Fprintf(&b, "marks: [%s]\n", strings.Join(c.Marks, ", "))
	}
	if c.Name != "" {
		fmt.Fprintf(&b, "name: %s\n", c.Name)
	}
	if c.ID != "" {
		fmt.Fprintf(&b, "id: %s\n", c.ID)
	}
	if c.State != "" {
		fmt.Fprintf(&b, "state: %s\n", c.State)
	}
	if c.Room != "" {
		fmt.Fprintf(&b, "room: %s\n", c.Room)
	}
	if c.Origin != "" {
		fmt.Fprintf(&b, "origin: %s\n", c.Origin)
	}
	b.WriteString("```\n")
	return b.String()
}

// ParseUser reads a signed user message. ok is false for other text.
func ParseUser(s string) (model.LogMessage, bool) {
	open, close := "<!-- lonchan ", "<!-- /lonchan -->"
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<!-- lonchaney ") {
		open, close = "<!-- lonchaney ", "<!-- /lonchaney -->"
	}
	if !strings.HasPrefix(s, open) || !strings.Contains(s, close) {
		return model.LogMessage{}, false
	}
	head, rest, _ := strings.Cut(s, "-->")
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), close))
	msg := model.LogMessage{Body: body}
	fields := strings.TrimPrefix(head, open)
	for _, part := range strings.Fields(fields) {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "id":
			msg.ID = v
		case "from":
			msg.From = v
		case "time":
			msg.Time = v
		case "sig":
			msg.Sig = v
		case "service":
			msg.Service = v
		}
	}
	return msg, msg.ID != ""
}

// ParseControl reads a fenced control document. ok is false for user text.
func ParseControl(s string) (model.Control, bool) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "```lonchaney"):
		s = strings.TrimPrefix(s, "```lonchaney")
	case strings.HasPrefix(s, "```lonchan"):
		s = strings.TrimPrefix(s, "```lonchan")
	default:
		return model.Control{}, false
	}
	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	var c model.Control
	for _, line := range strings.Split(s, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "type":
			c.Type = v
		case "name":
			c.Name = v
		case "id":
			c.ID = v
		case "state":
			c.State = v
		case "room":
			c.Room = v
		case "origin":
			c.Origin = v
		case "marks":
			v = strings.Trim(v, "[]")
			for _, part := range strings.Split(v, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					c.Marks = append(c.Marks, part)
				}
			}
		}
	}
	return c, c.Type != ""
}
