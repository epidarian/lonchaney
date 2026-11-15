package main

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/stanza"

	"lonchan/internal/model"
	"lonchan/internal/plugin"
	"lonchan/internal/xstyle"
)

type client struct {
	mu      sync.Mutex
	cfg     map[string]string
	session *xmpp.Session
	targets map[string]string
	emit    plugin.Emitter
	cancel  context.CancelFunc
	members map[string]map[string]struct{}
}

func (client) ID() string { return "xmpp" }
func (client) Capabilities() []string {
	return []string{"bold", "italic", "strike", "code", "code_block", "quote"}
}
func (client) Connection() (string, []model.Field) {
	text, fields, _ := plugin.Builtin(model.PluginXMPP)
	return text, fields
}

func (c *client) Configure(cfg map[string]string, emit plugin.Emitter) error {
	c.mu.Lock()
	c.cfg = cfg
	c.emit = emit
	c.targets = map[string]string{}
	c.members = map[string]map[string]struct{}{}
	c.mu.Unlock()
	return c.connect()
}

func (c *client) connect() error {
	c.mu.Lock()
	addr := c.cfg["jid"]
	pass := c.cfg["password"]
	host := c.cfg["host"]
	port := c.cfg["port"]
	c.mu.Unlock()
	if addr == "" {
		return nil
	}
	parsed, err := jid.Parse(addr)
	if err != nil {
		return err
	}
	if host == "" {
		host = parsed.Domainpart()
	}
	if port == "" {
		port = "5222"
	}
	ctx, cancel := context.WithCancel(context.Background())
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 20*time.Second)
	if err != nil {
		cancel()
		return err
	}
	session, err := xmpp.NewClientSession(ctx, parsed.Bare(), conn, xmpp.StartTLS(&tls.Config{ServerName: host}), xmpp.SASL("", pass, sasl.ScramSha256, sasl.ScramSha1, sasl.Plain), xmpp.BindResource())
	if err != nil {
		cancel()
		_ = conn.Close()
		return err
	}
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.cancel = cancel
	c.session = session
	c.mu.Unlock()
	go c.serve(session)
	return nil
}

func (c *client) serve(session *xmpp.Session) {
	_ = session.Serve(xmpp.HandlerFunc(func(t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
		if start.Name.Local != "message" {
			return nil
		}
		d := xml.NewTokenDecoder(xmlstream.MultiReader(xmlstream.Token(*start), t))
		var msg struct {
			stanza.Message
			Body string `xml:"body"`
		}
		if err := d.Decode(&msg); err != nil && err != io.EOF {
			return nil
		}
		from := msg.From.Bare().String()
		if msg.From.Resourcepart() != "" && msg.Type == stanza.GroupChatMessage {
			c.noteMember(from, msg.From.Resourcepart())
		}
		c.mu.Lock()
		emit := c.emit
		var sessionID string
		for id, target := range c.targets {
			if strings.EqualFold(target, from) || strings.EqualFold(target, msg.From.String()) {
				sessionID = id
			}
		}
		c.mu.Unlock()
		if emit == nil || msg.Body == "" {
			return nil
		}
		author := msg.From.Resourcepart()
		if author == "" {
			author = from
		}
		emit(plugin.Message{
			Op: "event", Session: sessionID, Author: author,
			Markdown: msg.Body, Time: time.Now().UTC().Format(time.RFC3339Nano),
		})
		return nil
	}))
}

func (c *client) noteMember(room, nick string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.members[room] == nil {
		c.members[room] = map[string]struct{}{}
	}
	c.members[room][nick] = struct{}{}
	if c.emit == nil {
		return
	}
	var names []string
	for name := range c.members[room] {
		names = append(names, name)
	}
	c.emit(plugin.Message{Op: "status", Text: "members " + strings.Join(names, ",")})
}

func (c *client) Open(session, target string, emit plugin.Emitter) error {
	c.mu.Lock()
	c.emit = emit
	if c.targets == nil {
		c.targets = map[string]string{}
	}
	c.targets[session] = target
	s := c.session
	c.mu.Unlock()
	userJID := !strings.Contains(target, "@conference") && !strings.Contains(target, "/") && strings.Count(target, "@") == 1
	if s == nil || userJID {
		return nil
	}
	to, err := jid.Parse(target)
	if err != nil {
		return err
	}
	return s.Send(context.Background(), stanza.Presence{To: to, Type: stanza.AvailablePresence}.Wrap(nil))
}

func (c *client) Send(sessionID, markdown string, _ plugin.Emitter) error {
	c.mu.Lock()
	target := c.targets[sessionID]
	s := c.session
	c.mu.Unlock()
	if s == nil || target == "" {
		return nil
	}
	to, err := jid.Parse(target)
	if err != nil {
		return err
	}
	kind := stanza.ChatMessage
	if strings.Contains(target, "conference") || strings.Contains(target, "@muc") {
		kind = stanza.GroupChatMessage
	}
	body := xstyle.ToXEP(markdown)
	return s.Send(context.Background(), stanza.Message{To: to, Type: kind}.Wrap(xmlstream.Wrap(xmlstream.Token(xml.CharData(body)), xml.StartElement{Name: xml.Name{Local: "body"}})))
}

func (c *client) Close(session string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.targets, session)
	return nil
}

func main() {
	plugin.Serve(&client{})
}
