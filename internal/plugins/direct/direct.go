package directplug

import (
	"crypto/ed25519"
	"fmt"
	"net"
	"sync"
	"time"

	"lonchan/internal/caps"
	"lonchan/internal/direct"
	"lonchan/internal/model"
)

// Incoming is a user message or a control notice from a peer.
type Incoming struct {
	Session string
	From    string
	Body    string
	Time    string
	ID      string
	Sig     string
	Room    string
	Members []string
	Status  string
	Mismatch string
}

// Manager dials and listens for markdown-direct sessions.
type Manager struct {
	mu       sync.Mutex
	priv     ed25519.PrivateKey
	listener net.Listener
	peers    map[string]*direct.Session
	rooms    map[string]map[string]*direct.Session
	relay    bool
	on       func(Incoming)
}

func New(priv ed25519.PrivateKey, on func(Incoming)) *Manager {
	return &Manager{
		priv:  priv,
		peers: map[string]*direct.Session{},
		rooms: map[string]map[string]*direct.Session{},
		on:    on,
	}
}

func (m *Manager) Capabilities() caps.Caps { return caps.Full() }

// Listen binds the address. An empty address leaves the listener stopped.
func (m *Manager) Listen(addr string, relay bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listener != nil {
		_ = m.listener.Close()
		m.listener = nil
	}
	m.relay = relay
	if addr == "" {
		return nil
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	m.listener = ln
	go m.accept(ln)
	return nil
}

func (m *Manager) Addr() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listener == nil {
		return ""
	}
	return m.listener.Addr().String()
}

func (m *Manager) accept(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		sess, err := direct.Handshake(c, m.priv, nil)
		if err != nil {
			_ = c.Close()
			continue
		}
		id := direct.PublicHex(sess.Peer())
		m.mu.Lock()
		m.peers[id] = sess
		m.mu.Unlock()
		m.sendCaps(sess)
		if m.on != nil {
			m.on(Incoming{Session: id, Status: "open", From: id})
		}
		go m.readLoop(id, sess)
	}
}

// Dial opens a session to host:port. expect may be empty for the first pin.
func (m *Manager) Dial(addr, expectHex string) (string, error) {
	var expect ed25519.PublicKey
	if expectHex != "" {
		pub, err := direct.ParsePublic(expectHex)
		if err != nil {
			return "", err
		}
		expect = pub
	}
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return "", err
	}
	sess, err := direct.Handshake(c, m.priv, expect)
	if err != nil {
		_ = c.Close()
		return "", err
	}
	id := direct.PublicHex(sess.Peer())
	m.mu.Lock()
	m.peers[id] = sess
	m.mu.Unlock()
	m.sendCaps(sess)
	go m.readLoop(id, sess)
	return id, nil
}

func (m *Manager) sendCaps(sess *direct.Session) {
	_ = sess.Send([]byte(direct.FormatControl(model.Control{Type: "caps", Marks: m.Capabilities().Names()})))
}

func (m *Manager) readLoop(id string, sess *direct.Session) {
	for {
		plain, err := sess.Recv()
		if err != nil {
			if m.on != nil {
				m.on(Incoming{Session: id, Status: "closed"})
			}
			return
		}
		text := string(plain)
		if c, ok := direct.ParseControl(text); ok {
			if c.Type == "relay" && c.Room != "" {
				m.join(c.Room, id, sess)
			}
			if m.on != nil {
				m.on(Incoming{Session: id, From: c.Origin, Room: c.Room, Status: c.Type, Body: text})
			}
			continue
		}
		if m.on != nil {
			m.on(Incoming{Session: id, From: id, Body: text})
		}
		m.fanout(id, plain)
	}
}

func (m *Manager) join(room, id string, sess *direct.Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rooms[room] == nil {
		m.rooms[room] = map[string]*direct.Session{}
	}
	m.rooms[room][id] = sess
}

func (m *Manager) fanout(from string, plain []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.relay {
		return
	}
	for room, peers := range m.rooms {
		if _, ok := peers[from]; !ok {
			continue
		}
		for id, peer := range peers {
			if id == from {
				continue
			}
			_ = peer.Send(plain)
			_ = room
		}
	}
}

// Send writes markdown to an open peer session.
func (m *Manager) Send(peerHex string, plain []byte) error {
	m.mu.Lock()
	sess := m.peers[peerHex]
	m.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("no session for %s", peerHex)
	}
	return sess.Send(plain)
}

// Broadcast writes a relayed payload to every peer already in the room.
func (m *Manager) Broadcast(room string, plain []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, peer := range m.rooms[room] {
		_ = peer.Send(plain)
	}
}

// SendRoom signs and sends a relayed user message.
func (m *Manager) SendRoom(peerHex, room string, msg model.LogMessage) error {
	fence := direct.FormatControl(model.Control{Type: "relay", Room: room, Origin: msg.From})
	body := fence + direct.FormatUser(msg)
	if err := m.Send(peerHex, []byte(body)); err != nil {
		return err
	}
	m.mu.Lock()
	if m.rooms[room] == nil {
		m.rooms[room] = map[string]*direct.Session{}
	}
	if sess := m.peers[peerHex]; sess != nil {
		m.rooms[room][peerHex] = sess
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listener != nil {
		_ = m.listener.Close()
	}
	for _, s := range m.peers {
		_ = s.Close()
	}
}
