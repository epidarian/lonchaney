package hub

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lonchan/internal/caps"
	"lonchan/internal/direct"
	directplug "lonchan/internal/plugins/direct"
	"lonchan/internal/model"
	"lonchan/internal/plugin"
	"lonchan/internal/plugins/local"
	"lonchan/internal/store"
)

// Pending is an inbound markdown-direct peer with no pin yet.
type Pending struct {
	Address string
	Key     string
}

// Hub routes composer sends and plugin events through the store.
type Hub struct {
	Store   *store.Store
	Local   *local.Broker
	mu      sync.Mutex
	direct  map[string]*directplug.Manager
	procs   map[string]*plugin.Proc
	members map[string][]string
	pending []Pending
	status  map[string]string
	On      func()
}

func New(st *store.Store) *Hub {
	h := &Hub{
		Store:   st,
		Local:   local.New(),
		direct:  map[string]*directplug.Manager{},
		procs:   map[string]*plugin.Proc{},
		members: map[string][]string{},
		status:  map[string]string{},
	}
	return h
}

func (h *Hub) touch() {
	if h.On != nil {
		h.On()
	}
}

func (h *Hub) Caps(pluginName string) caps.Caps {
	switch pluginName {
	case model.PluginIRC:
		return caps.IRC()
	case model.PluginXMPP:
		return caps.XMPP()
	default:
		return caps.Full()
	}
}

func (h *Hub) Status(serviceID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.status[serviceID]
}

func (h *Hub) Members(target string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.members[target]...)
}

func (h *Hub) Pending() []Pending {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Pending(nil), h.pending...)
}

func (h *Hub) setStatus(serviceID, text string) {
	h.mu.Lock()
	h.status[serviceID] = text
	h.mu.Unlock()
	h.Store.SetServiceStatus(serviceID, text)
	h.touch()
}

// Connect brings a service online.
func (h *Hub) Connect(svc model.Service) error {
	switch svc.Plugin {
	case model.PluginLocal:
		h.setStatus(svc.ID, "connected")
		return nil
	case model.PluginDirect:
		return h.connectDirect(svc)
	default:
		return h.connectExternal(svc)
	}
}

func (h *Hub) connectDirect(svc model.Service) error {
	priv, err := h.Store.Private(svc.IdentityID)
	if err != nil {
		return err
	}
	mgr := directplug.New(priv, func(in directplug.Incoming) {
		h.onDirect(svc, in)
	})
	h.mu.Lock()
	if old := h.direct[svc.ID]; old != nil {
		old.Close()
	}
	h.direct[svc.ID] = mgr
	h.mu.Unlock()
	bind := listenAddr(svc.Config)
	relay := svc.Config["relay"] == "true"
	if err := mgr.Listen(bind, relay); err != nil {
		h.setStatus(svc.ID, err.Error())
		return err
	}
	if bind == "" {
		h.setStatus(svc.ID, "idle")
	} else {
		h.setStatus(svc.ID, "listening "+mgr.Addr())
	}
	return nil
}

func (h *Hub) onDirect(svc model.Service, in directplug.Incoming) {
	if in.Status == "open" && in.From != "" {
		if _, ok := h.Store.Pin(in.From); !ok {
			h.mu.Lock()
			h.pending = append(h.pending, Pending{Address: in.From, Key: in.From})
			h.mu.Unlock()
		}
		h.touch()
		return
	}
	if strings.Contains(in.Status, "does not match") || in.Mismatch != "" {
		h.Store.SetHandshake(svc.IdentityID, in.Status+" "+in.Mismatch)
		h.touch()
		return
	}
	if in.Body == "" {
		h.touch()
		return
	}
	if c, ok := direct.ParseControl(in.Body); ok {
		if c.Type == "profile" && c.Name != "" {
			addr := in.From
			key := in.From
			if pin, ok := h.Store.Pin(addr); ok {
				key = pin.Key
			}
			_ = h.Store.PutPin(model.Pin{Address: addr, Key: key, Name: c.Name})
			h.touch()
		}
		return
	}
	msg, ok := direct.ParseUser(in.Body)
	if !ok {
		msg = model.LogMessage{ID: newID(), Body: in.Body, Time: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	if msg.From == "" {
		msg.From = in.From
	}
	if msg.Time == "" {
		msg.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	msg.Service = svc.ID
	_ = h.ingest(svc.ID, in.From, msg)
}

func listenAddr(cfg map[string]string) string {
	if cfg == nil {
		return ""
	}
	if cfg["bind"] != "" {
		return cfg["bind"]
	}
	host, port := cfg["bindHost"], cfg["bindPort"]
	if host == "" && port == "" {
		return ""
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "0"
	}
	return net.JoinHostPort(host, port)
}

func (h *Hub) connectExternal(svc model.Service) error {
	path := h.pluginPath(svc.Plugin)
	if path == "" {
		h.setStatus(svc.ID, "plugin path empty")
		return fmt.Errorf("plugin path empty for %s", svc.Plugin)
	}
	logPath := filepath.Join(h.Store.Dir(), "logs", svc.Plugin+".log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	proc, err := plugin.Start(path, f)
	if err != nil {
		h.setStatus(svc.ID, err.Error())
		return err
	}
	h.mu.Lock()
	if old := h.procs[svc.ID]; old != nil {
		old.Stop()
	}
	h.procs[svc.ID] = proc
	h.mu.Unlock()
	go h.readProc(svc, proc)
	if err := proc.Send(plugin.Message{Op: "configure", Config: svc.Config}); err != nil {
		return err
	}
	return proc.Send(plugin.Message{Op: "capabilities"})
}

func (h *Hub) readProc(svc model.Service, proc *plugin.Proc) {
	for msg := range proc.Events() {
		switch msg.Op {
		case "status":
			if strings.HasPrefix(msg.Text, "members ") {
				h.mu.Lock()
				h.members[svc.ID] = strings.Split(strings.TrimPrefix(msg.Text, "members "), ",")
				h.mu.Unlock()
			} else {
				h.setStatus(svc.ID, msg.Text)
				continue
			}
		case "error":
			h.setStatus(svc.ID, msg.Text)
			continue
		case "event":
			target := msg.Session
			if c, ok := h.contextByID(msg.Session); ok {
				target = c.Target
			}
			body := msg.Markdown
			when := msg.Time
			if when == "" {
				when = time.Now().UTC().Format(time.RFC3339Nano)
			}
			_ = h.ingest(svc.ID, target, model.LogMessage{ID: newID(), From: msg.Author, Time: when, Body: body, Service: svc.ID})
			continue
		}
		h.touch()
	}
}

func (h *Hub) pluginPath(id string) string {
	cfg := h.Store.Config()
	for _, p := range cfg.Plugins {
		if p.ID == id && p.Enabled && p.Path != "" {
			return p.Path
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "plugin-" + id
	if id == model.PluginIRC {
		name = "plugin-irc"
	}
	if id == model.PluginXMPP {
		name = "plugin-xmpp"
	}
	return filepath.Join(filepath.Dir(exe), name+".exe")
}

func (h *Hub) contextByID(id string) (model.Context, bool) {
	return h.Store.Context(id)
}

// Disconnect closes a service session.
func (h *Hub) Disconnect(svc model.Service) {
	h.mu.Lock()
	if p := h.procs[svc.ID]; p != nil {
		_ = p.Send(plugin.Message{Op: "close", Session: svc.ID})
		p.Stop()
		delete(h.procs, svc.ID)
	}
	if d := h.direct[svc.ID]; d != nil {
		d.Close()
		delete(h.direct, svc.ID)
	}
	h.mu.Unlock()
	h.setStatus(svc.ID, "disconnected")
}

// Send appends a message and hands it to the service for ctx.
func (h *Hub) Send(ctx model.Context, markdown string) error {
	if ctx.Target == "" || markdown == "" {
		return nil
	}
	id, _ := h.Store.Identity(ctx.IdentityID)
	from := id.Public
	if from == "" {
		from = id.Label()
	}
	svc, ok := h.Store.Service(ctx.ServiceID)
	if ok && (svc.Plugin == model.PluginIRC || svc.Plugin == model.PluginXMPP) {
		if nick := svc.Config["nick"]; nick != "" {
			from = nick
		}
		if j := svc.Config["jid"]; j != "" && svc.Plugin == model.PluginXMPP {
			from = j
		}
	}
	msg := model.LogMessage{
		ID: newID(), From: from, Time: time.Now().UTC().Format(time.RFC3339Nano),
		Service: ctx.ServiceID, Body: markdown,
	}
	if ok && svc.Plugin == model.PluginDirect {
		if priv, err := h.Store.Private(ctx.IdentityID); err == nil {
			msg.Sig = direct.SignMessage(priv, ctx.Target, msg.ID, msg.Time, []byte(markdown))
		}
	}
	if err := h.ingest(ctx.ServiceID, ctx.Target, msg); err != nil {
		return err
	}
	switch {
	case !ok || svc.Plugin == model.PluginLocal:
		h.Local.Publish(ctx.Target, msg)
	case svc.Plugin == model.PluginDirect:
		return h.sendDirect(svc, ctx, msg)
	default:
		h.mu.Lock()
		proc := h.procs[svc.ID]
		h.mu.Unlock()
		if proc == nil {
			if err := h.connectExternal(svc); err != nil {
				return err
			}
			h.mu.Lock()
			proc = h.procs[svc.ID]
			h.mu.Unlock()
		}
		if proc == nil {
			return fmt.Errorf("plugin not running")
		}
		if err := proc.Send(plugin.Message{Op: "open", Session: ctx.ID, Target: ctx.Target}); err != nil {
			return err
		}
		return proc.Send(plugin.Message{Op: "send", Session: ctx.ID, Markdown: markdown})
	}
	return nil
}

func (h *Hub) sendDirect(svc model.Service, ctx model.Context, msg model.LogMessage) error {
	h.mu.Lock()
	mgr := h.direct[svc.ID]
	h.mu.Unlock()
	if mgr == nil {
		if err := h.connectDirect(svc); err != nil {
			return err
		}
		h.mu.Lock()
		mgr = h.direct[svc.ID]
		h.mu.Unlock()
	}
	if ctx.TargetKind == model.TargetGroup {
		wire := direct.FormatControl(model.Control{Type: "relay", Room: ctx.Target, Origin: msg.From}) + direct.FormatUser(msg)
		mgr.Broadcast(ctx.Target, []byte(wire))
		return nil
	}
	expect := ""
	if pin, ok := h.Store.Pin(ctx.Target); ok {
		expect = pin.Key
	}
	peer, err := mgr.Dial(ctx.Target, expect)
	if err != nil {
		h.Store.SetHandshake(ctx.IdentityID, err.Error())
		h.touch()
		return err
	}
	if _, ok := h.Store.Pin(ctx.Target); !ok {
		_ = h.Store.PutPin(model.Pin{Address: ctx.Target, Key: peer})
	} else if pin, _ := h.Store.Pin(ctx.Target); pin.Key != peer {
		h.Store.SetHandshake(ctx.IdentityID, "pin mismatch "+pin.Key+" "+peer)
		h.touch()
		return fmt.Errorf("pin mismatch")
	}
	wire := direct.FormatUser(msg)
	return mgr.Send(peer, []byte(wire))
}

func (h *Hub) ingest(serviceID, target string, msg model.LogMessage) error {
	existing, _ := h.Store.ReadLog(serviceID, target)
	for _, m := range existing {
		if m.ID == msg.ID {
			h.touch()
			return nil
		}
	}
	if err := h.Store.AppendLog(serviceID, target, msg); err != nil {
		return err
	}
	h.touch()
	return nil
}

// Accept pins an inbound peer and records a user conversation.
func (h *Hub) Accept(key string) (model.Context, error) {
	_ = h.Store.PutPin(model.Pin{Address: key, Key: key})
	h.mu.Lock()
	next := h.pending[:0]
	for _, p := range h.pending {
		if p.Key != key {
			next = append(next, p)
		}
	}
	h.pending = next
	h.mu.Unlock()
	svcID := "svc-direct"
	ident := ""
	if len(h.Store.Identities()) > 0 {
		ident = h.Store.Identities()[0].ID
	}
	ctx := model.Context{
		ID: newID(), Name: key[:min(8, len(key))], Kind: model.KindConversation,
		ServiceID: svcID, IdentityID: ident, TargetKind: model.TargetUser, Target: key,
	}
	if err := h.Store.PutContext(ctx); err != nil {
		return ctx, err
	}
	h.touch()
	return ctx, nil
}

// EnsureConversation returns an existing conversation for the pair or creates one.
func (h *Hub) EnsureConversation(serviceID, identityID, targetKind, target, name string) (model.Context, error) {
	for _, c := range h.Store.Contexts() {
		if c.Kind == model.KindConversation && c.ServiceID == serviceID && c.Target == target {
			return c, nil
		}
	}
	ctx := model.Context{
		ID: newID(), Name: name, Kind: model.KindConversation,
		ServiceID: serviceID, IdentityID: identityID, TargetKind: targetKind, Target: target,
	}
	err := h.Store.PutContext(ctx)
	return ctx, err
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, p := range h.procs {
		p.Stop()
	}
	for _, d := range h.direct {
		d.Close()
	}
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
