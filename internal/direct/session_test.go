package direct

import (
	"bytes"
	"crypto/ed25519"
	"io"
	"net"
	"testing"
	"time"
)

func pair(t *testing.T) (*Session, *Session) {
	t.Helper()
	aPub, aPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	bPub, bPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	type result struct {
		s   *Session
		err error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			ch <- result{err: err}
			return
		}
		s, err := Handshake(c, bPriv, aPub)
		ch <- result{s: s, err: err}
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	client, err := Handshake(c, aPriv, bPub)
	if err != nil {
		t.Fatal(err)
	}
	got := <-ch
	if got.err != nil {
		t.Fatal(got.err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		_ = got.s.Close()
	})
	return client, got.s
}

func TestHandshakeAndFrame(t *testing.T) {
	client, server := pair(t)
	if err := client.Send([]byte("hello **md**")); err != nil {
		t.Fatal(err)
	}
	got, err := server.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello **md**" {
		t.Fatalf("%q", got)
	}
	if err := server.Send([]byte("back")); err != nil {
		t.Fatal(err)
	}
	got, err = client.Recv()
	if err != nil || string(got) != "back" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestBadSignature(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 32)
		_, _ = io.ReadFull(c, buf)
		_, _ = c.Write(buf)
		pkt := make([]byte, ed25519.PublicKeySize+ed25519.SignatureSize)
		_, _ = io.ReadFull(c, pkt)
		pkt[len(pkt)-1] ^= 0xff
		_, _ = c.Write(pkt)
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := Handshake(c, priv, nil); err == nil {
		t.Fatal("expected bad signature")
	}
}

func TestBadTagAndReplay(t *testing.T) {
	client, server := pair(t)
	if err := client.Send([]byte("a")); err != nil {
		t.Fatal(err)
	}
	frame := readFrame(t, server.conn)
	plain, err := server.open(frame)
	if err != nil || string(plain) != "a" {
		t.Fatalf("%q %v", plain, err)
	}
	if _, err := server.open(frame); err == nil {
		t.Fatal("replayed frame opened")
	}
	bad := bytes.Clone(frame)
	bad[len(bad)-1] ^= 0x01
	server.recvN = 0
	if _, err := server.open(bad); err == nil {
		t.Fatal("tampered tag opened")
	}
	if AcceptNonce(2, 1) {
		t.Fatal("regression accepted")
	}
}

func readFrame(t *testing.T, c net.Conn) []byte {
	t.Helper()
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(c, hdr); err != nil {
		t.Fatal(err)
	}
	n := int(hdr[0])<<24 | int(hdr[1])<<16 | int(hdr[2])<<8 | int(hdr[3])
	buf := make([]byte, n)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatal(err)
	}
	return buf
}

func TestSign(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	sig := SignMessage(priv, "room", "id", "t", []byte("body"))
	if !VerifyMessage(pub, "room", "id", "t", sig, []byte("body")) {
		t.Fatal("verify")
	}
	if VerifyMessage(pub, "room", "id", "t", sig, []byte("body2")) {
		t.Fatal("tampered body")
	}
}
