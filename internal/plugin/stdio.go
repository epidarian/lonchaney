package plugin

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"

	"lonchan/internal/model"
)

// Message is one newline-delimited JSON object on the plugin pipe.
type Message struct {
	Op           string            `json:"op"`
	ID           string            `json:"id,omitempty"`
	Config       map[string]string `json:"config,omitempty"`
	Marks        []string          `json:"marks,omitempty"`
	Session      string            `json:"session,omitempty"`
	Target       string            `json:"target,omitempty"`
	Markdown     string            `json:"markdown,omitempty"`
	Author       string            `json:"author,omitempty"`
	Time         string            `json:"time,omitempty"`
	Text         string            `json:"text,omitempty"`
	Authors      []string          `json:"authors,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	Fields       []model.Field     `json:"fields,omitempty"`
}

// Spec is implemented by a plugin that tells the host how to open its connection.
type Spec interface {
	Connection() (instructions string, fields []model.Field)
}

// Emitter writes one message back to the host.
type Emitter func(Message)

// Handler is implemented by an in-process or external plugin.
type Handler interface {
	Configure(map[string]string, Emitter) error
	Capabilities() []string
	Open(session, target string, emit Emitter) error
	Send(session, markdown string, emit Emitter) error
	Close(session string) error
}

// Serve runs a plugin on stdin and stdout.
func Serve(h Handler) {
	ServeIO(os.Stdin, os.Stdout, h)
}

// ServeIO speaks the host protocol on the given streams.
func ServeIO(in io.Reader, out io.Writer, h Handler) {
	w := bufio.NewWriter(out)
	write := func(m Message) {
		b, _ := json.Marshal(m)
		_, _ = w.Write(b)
		_ = w.WriteByte('\n')
		_ = w.Flush()
	}
	_, _ = io.WriteString(w, "lonchan-plugin 1\n")
	_ = w.Flush()
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var m Message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			write(Message{Op: "error", Text: err.Error()})
			continue
		}
		switch m.Op {
		case "configure":
			if err := h.Configure(m.Config, write); err != nil {
				write(Message{Op: "error", Text: err.Error()})
				continue
			}
			write(Message{Op: "ready", ID: pluginID(h)})
		case "capabilities":
			msg := Message{Op: "capabilities", Marks: h.Capabilities(), ID: pluginID(h)}
			if spec, ok := h.(Spec); ok {
				msg.Instructions, msg.Fields = spec.Connection()
			}
			write(msg)
		case "open":
			if err := h.Open(m.Session, m.Target, write); err != nil {
				write(Message{Op: "error", Session: m.Session, Text: err.Error()})
				continue
			}
			write(Message{Op: "status", Session: m.Session, Text: "open"})
		case "send":
			if err := h.Send(m.Session, m.Markdown, write); err != nil {
				write(Message{Op: "error", Session: m.Session, Text: err.Error()})
			}
		case "close":
			_ = h.Close(m.Session)
			write(Message{Op: "status", Session: m.Session, Text: "closed"})
		}
	}
}

type ider interface{ ID() string }

func pluginID(h Handler) string {
	if v, ok := h.(ider); ok {
		return v.ID()
	}
	return ""
}

// Proc is a running external plugin.
type Proc struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	mu     sync.Mutex
	events chan Message
}

// Start launches a plugin binary. stderr is appended to errW.
func Start(path string, errW io.Writer) (*Proc, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = errW
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &Proc{cmd: cmd, in: stdin, events: make(chan Message, 32)}
	go p.read(stdout)
	return p, nil
}

func (p *Proc) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			if line == "lonchan-plugin 1" || line == "lonchaney-plugin 1" {
				continue
			}
		}
		var m Message
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		p.events <- m
	}
	close(p.events)
}

// Events receives plugin messages.
func (p *Proc) Events() <-chan Message { return p.events }

// Send writes one host message.
func (p *Proc) Send(m Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = p.in.Write(b)
	return err
}

// Stop ends the process.
func (p *Proc) Stop() {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
	}
}
