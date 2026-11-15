package store

import (
	"fmt"
	"os"
	"strings"

	"lonchan/internal/model"
)

// AppendLog writes one user message using the shared comment delimiters.
func AppendLog(path string, msg model.LogMessage) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(FormatLog(msg))
	return err
}

// ReadLog returns user messages. Control fences are skipped.
func ReadLog(path string) ([]model.LogMessage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return ParseLog(string(b))
}

// FormatLog renders one message.
func FormatLog(msg model.LogMessage) string {
	return fmt.Sprintf("<!-- lonchan id=%s from=%s time=%s sig=%s service=%s -->\n%s\n<!-- /lonchan -->\n",
		msg.ID, msg.From, msg.Time, msg.Sig, msg.Service, strings.TrimRight(msg.Body, "\n"))
}

// ParseLog reads user messages from a log file body.
func ParseLog(s string) ([]model.LogMessage, error) {
	var out []model.LogMessage
	rest := s
	for {
		i, open := firstIndex(rest, "<!-- lonchaney ", "<!-- lonchan ")
		if i < 0 {
			break
		}
		rest = rest[i+len(open):]
		end := strings.Index(rest, "-->")
		if end < 0 {
			break
		}
		header := rest[:end]
		rest = rest[end+3:]
		if strings.HasPrefix(strings.TrimSpace(rest), "\n") {
			rest = strings.TrimPrefix(rest, "\n")
			if strings.HasPrefix(rest, "\r\n") {
				rest = rest[2:]
			}
		}
		rest = strings.TrimPrefix(rest, "\r\n")
		rest = strings.TrimPrefix(rest, "\n")
		closeAt, close := firstIndex(rest, "<!-- /lonchaney -->", "<!-- /lonchan -->")
		if closeAt < 0 {
			break
		}
		body := strings.TrimRight(rest[:closeAt], "\n")
		rest = rest[closeAt+len(close):]
		fields := map[string]string{}
		for _, part := range strings.Fields(header) {
			k, v, ok := strings.Cut(part, "=")
			if ok {
				fields[k] = v
			}
		}
		out = append(out, model.LogMessage{
			ID:      fields["id"],
			From:    fields["from"],
			Time:    fields["time"],
			Sig:     fields["sig"],
			Service: fields["service"],
			Body:    body,
		})
	}
	return out, nil
}

func firstIndex(s string, marks ...string) (int, string) {
	best := -1
	which := ""
	for _, m := range marks {
		i := strings.Index(s, m)
		if i >= 0 && (best < 0 || i < best) {
			best = i
			which = m
		}
	}
	return best, which
}

func dirOf(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i < 0 {
		return "."
	}
	return path[:i]
}
