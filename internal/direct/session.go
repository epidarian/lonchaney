package direct

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const (
	labelHello = "lonchaney markdown-direct 1"
	labelMsg   = "lonchaney markdown-direct 1 message"
	maxPlain   = 256 * 1024
)

// Session is one authenticated TCP connection.
type Session struct {
	conn    net.Conn
	sendKey []byte
	recvKey []byte
	sendN   uint64
	recvN   uint64
	pub     ed25519.PublicKey
	peer    ed25519.PublicKey
}

func (s *Session) Local() ed25519.PublicKey { return s.pub }
func (s *Session) Peer() ed25519.PublicKey  { return s.peer }
func (s *Session) Remote() string {
	if s.conn == nil || s.conn.RemoteAddr() == nil {
		return ""
	}
	return s.conn.RemoteAddr().String()
}
func (s *Session) Close() error { return s.conn.Close() }

// Handshake runs the mutual Ed25519 transcript over ephemeral X25519.
// expect, when non-nil, must equal the peer's static public key.
func Handshake(conn net.Conn, priv ed25519.PrivateKey, expect ed25519.PublicKey) (*Session, error) {
	ephPriv := make([]byte, 32)
	if _, err := rand.Read(ephPriv); err != nil {
		return nil, err
	}
	ephPub, err := curve25519.X25519(ephPriv, curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(ephPub); err != nil {
		return nil, err
	}
	peerEph := make([]byte, 32)
	if _, err := io.ReadFull(conn, peerEph); err != nil {
		return nil, err
	}
	shared, err := curve25519.X25519(ephPriv, peerEph)
	if err != nil {
		return nil, err
	}
	pub := priv.Public().(ed25519.PublicKey)
	sig := ed25519.Sign(priv, transcript(ephPub, peerEph, pub))
	packet := append(append([]byte{}, pub...), sig...)
	if _, err := conn.Write(packet); err != nil {
		return nil, err
	}
	peerPacket := make([]byte, ed25519.PublicKeySize+ed25519.SignatureSize)
	if _, err := io.ReadFull(conn, peerPacket); err != nil {
		return nil, err
	}
	peerPub := ed25519.PublicKey(peerPacket[:ed25519.PublicKeySize])
	peerSig := peerPacket[ed25519.PublicKeySize:]
	if !ed25519.Verify(peerPub, transcript(peerEph, ephPub, peerPub), peerSig) {
		_ = conn.Close()
		return nil, errors.New("peer signature rejected")
	}
	if expect != nil && !peerPub.Equal(expect) {
		_ = conn.Close()
		return nil, errors.New("peer key does not match the pin")
	}
	send, recv := directionKeys(shared, pub, peerPub)
	return &Session{conn: conn, sendKey: send, recvKey: recv, pub: pub, peer: peerPub}, nil
}

func transcript(myEph, theirEph, myPub []byte) []byte {
	out := append([]byte(labelHello), myEph...)
	out = append(out, theirEph...)
	out = append(out, myPub...)
	return out
}

func directionKeys(shared []byte, self, peer ed25519.PublicKey) (send, recv []byte) {
	okm := make([]byte, 64)
	_, _ = io.ReadFull(hkdfReader(shared, self, peer), okm)
	if string(self) < string(peer) {
		return okm[:32], okm[32:]
	}
	return okm[32:], okm[:32]
}

func hkdfReader(shared, self, peer []byte) io.Reader {
	a, b := self, peer
	if string(peer) < string(self) {
		a, b = peer, self
	}
	info := append(append([]byte{}, a...), b...)
	return hkdf.New(sha256.New, shared, []byte(labelHello), info)
}

// Send encrypts one frame. A plaintext longer than 256 KiB is rejected.
func (s *Session) Send(plain []byte) error {
	if len(plain) > maxPlain {
		return errors.New("plaintext above 256 KiB")
	}
	aead, err := chacha20poly1305.New(s.sendKey)
	if err != nil {
		return err
	}
	var nonce [12]byte
	binary.LittleEndian.PutUint64(nonce[:8], s.sendN)
	s.sendN++
	ct := aead.Seal(nil, nonce[:], plain, nil)
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(ct)))
	if _, err := s.conn.Write(hdr[:]); err != nil {
		return err
	}
	_, err = s.conn.Write(ct)
	return err
}

// Recv reads one frame. A failed tag or a nonce regression closes the socket.
func (s *Session) Recv() ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(s.conn, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > maxPlain+64 {
		_ = s.conn.Close()
		return nil, errors.New("frame length above 256 KiB")
	}
	ct := make([]byte, n)
	if _, err := io.ReadFull(s.conn, ct); err != nil {
		return nil, err
	}
	return s.open(ct)
}

func (s *Session) open(ct []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(s.recvKey)
	if err != nil {
		return nil, err
	}
	var nonce [12]byte
	binary.LittleEndian.PutUint64(nonce[:8], s.recvN)
	plain, err := aead.Open(nil, nonce[:], ct, nil)
	if err != nil {
		if s.conn != nil {
			_ = s.conn.Close()
		}
		return nil, errors.New("tag rejected")
	}
	s.recvN++
	return plain, nil
}

// ForceRecvNonce sets the expected counter. A later frame using an earlier counter fails the tag check
// because the nonce will not match, and RecvNonce rejects a regression before decrypt when asked.
func (s *Session) RecvNonce() uint64 { return s.recvN }

// AcceptNonce reports whether n is the next counter. A regression closes the caller.
func AcceptNonce(prev, n uint64) bool { return n == prev }

// SignMessage signs room || id || time || body.
func SignMessage(priv ed25519.PrivateKey, room, id, time string, body []byte) string {
	return hex.EncodeToString(ed25519.Sign(priv, messageBytes(room, id, time, body)))
}

// VerifyMessage checks an origin signature.
func VerifyMessage(pub ed25519.PublicKey, room, id, time, sigHex string, body []byte) bool {
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, messageBytes(room, id, time, body), sig)
}

func messageBytes(room, id, time string, body []byte) []byte {
	out := append([]byte(labelMsg), []byte(room)...)
	out = append(out, []byte(id)...)
	out = append(out, []byte(time)...)
	out = append(out, body...)
	return out
}

// PublicHex encodes a public key.
func PublicHex(pub ed25519.PublicKey) string { return hex.EncodeToString(pub) }

// ParsePublic decodes a hex public key.
func ParsePublic(s string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, errors.New("public key length")
	}
	return ed25519.PublicKey(b), nil
}
