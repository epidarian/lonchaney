package main

import (
	"bufio"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"time"

	"lonchan/internal/ircfmt"
	"lonchan/internal/model"
	"lonchan/internal/plugin"
)

type irc struct {
	mu      sync.Mutex
	cfg     map[string]string
	conn    net.Conn
	reader  *bufio.Reader
	targets map[string]string
	emit    plugin.Emitter
}

func (irc) ID() string { return "irc" }
func (irc) Capabilities() []string {
	return []string{"bold", "italic", "strike"}
}
func (irc) Connection() (string, []model.Field) {
	text, fields, _ := plugin.Builtin(model.PluginIRC)
	return text, fields
}

func (c *irc) Configure(cfg map[string]string, emit plugin.Emitter) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
	c.emit = emit
	c.targets = map[string]string{}
	return c.dial()
}

func (c *irc) dial() error {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	addr := c.cfg["host"]
	if port := c.cfg["port"]; port != "" && !strings.Contains(addr, ":") {
		addr += ":" + port
	}
	if addr == "" {
		return nil
	}
	var conn net.Conn
	var err error
	if c.cfg["tls"] == "true" {
		conn, err = tls.Dial("tcp", addr, &tls.Config{ServerName: strings.Split(addr, ":")[0]})
	} else {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
	}
	if err != nil {
		return err
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	nick := c.cfg["nick"]
	if nick == "" {
		nick = "lonchan"
	}
	_, _ = conn.Write([]byte("NICK " + nick + "\r\nUSER " + nick + " 0 * :lonchan\r\n"))
	if pass := c.cfg["password"]; pass != "" {
		_, _ = conn.Write([]byte("PASS " + pass + "\r\n"))
	}
	go c.read()
	return nil
}

func (c *irc) read() {
	for {
		c.mu.Lock()
		r := c.reader
		emit := c.emit
		c.mu.Unlock()
		if r == nil {
			return
		}
		line, err := r.ReadString('\n')
		if err != nil {
			if emit != nil {
				emit(plugin.Message{Op: "status", Text: "closed"})
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "PING") {
			pong := "PONG" + strings.TrimPrefix(line, "PING")
			c.mu.Lock()
			if c.conn != nil {
				_, _ = c.conn.Write([]byte(pong + "\r\n"))
			}
			c.mu.Unlock()
			continue
		}
		if !strings.Contains(line, " PRIVMSG ") {
			continue
		}
		from, rest, _ := strings.Cut(strings.TrimPrefix(line, ":"), " ")
		nick, _, _ := strings.Cut(from, "!")
		_, msg, ok := strings.Cut(rest, " :")
		if !ok || emit == nil {
			continue
		}
		c.mu.Lock()
		var session string
		for id, target := range c.targets {
			if strings.EqualFold(target, nick) || strings.Contains(rest, " "+target+" ") {
				session = id
				break
			}
		}
		c.mu.Unlock()
		emit(plugin.Message{Op: "event", Session: session, Author: nick, Markdown: ircfmt.FromIRC(msg), Time: time.Now().UTC().Format(time.RFC3339Nano)})
	}
}

func (c *irc) Open(session, target string, emit plugin.Emitter) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emit = emit
	if c.targets == nil {
		c.targets = map[string]string{}
	}
	c.targets[session] = target
	if c.conn == nil {
		return nil
	}
	if strings.HasPrefix(target, "#") {
		_, err := c.conn.Write([]byte("JOIN " + target + "\r\n"))
		return err
	}
	return nil
}

func (c *irc) Send(session, markdown string, _ plugin.Emitter) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	target := c.targets[session]
	if c.conn == nil || target == "" {
		return nil
	}
	line := "PRIVMSG " + target + " :" + ircfmt.ToIRC(markdown) + "\r\n"
	_, err := c.conn.Write([]byte(line))
	return err
}

func (c *irc) Close(session string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.targets, session)
	return nil
}

func main() {
	plugin.Serve(&irc{})
}
