package plugin

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"lonchan/internal/model"
)

type echo struct{}

func (echo) ID() string                                 { return "echo" }
func (echo) Configure(map[string]string, Emitter) error { return nil }
func (echo) Capabilities() []string                     { return []string{"bold"} }
func (echo) Open(string, string, Emitter) error         { return nil }
func (echo) Send(string, string, Emitter) error         { return nil }
func (echo) Close(string) error                         { return nil }

type specEcho struct{ echo }

func (specEcho) Connection() (string, []model.Field) {
	return "Dial the host.", []model.Field{{Key: "host", Label: "Host"}}
}

func TestConnectionSpec(t *testing.T) {
	hostToPluginR, hostToPluginW := io.Pipe()
	pluginToHostR, pluginToHostW := io.Pipe()
	go ServeIO(hostToPluginR, pluginToHostW, specEcho{})
	if line, err := readLine(pluginToHostR); err != nil || line != "lonchan-plugin 1" {
		t.Fatalf("%q %v", line, err)
	}
	if _, err := hostToPluginW.Write([]byte("{\"op\":\"capabilities\"}\n")); err != nil {
		t.Fatal(err)
	}
	got := mustMsg(t, pluginToHostR)
	if got.Instructions != "Dial the host." || len(got.Fields) != 1 || got.Fields[0].Key != "host" {
		t.Fatalf("%+v", got)
	}
	_ = hostToPluginW.Close()
}

func TestServeRoundTrip(t *testing.T) {
	hostToPluginR, hostToPluginW := io.Pipe()
	pluginToHostR, pluginToHostW := io.Pipe()
	go ServeIO(hostToPluginR, pluginToHostW, echo{})
	line, err := readLine(pluginToHostR)
	if err != nil || line != "lonchan-plugin 1" {
		t.Fatalf("%q %v", line, err)
	}
	if _, err := hostToPluginW.Write([]byte("{\"op\":\"capabilities\"}\n")); err != nil {
		t.Fatal(err)
	}
	got := mustMsg(t, pluginToHostR)
	if got.Op != "capabilities" || len(got.Marks) != 1 || got.Marks[0] != "bold" {
		t.Fatalf("%+v", got)
	}
	_ = hostToPluginW.Close()
}

func readLine(r io.Reader) (string, error) {
	var buf []byte
	tmp := make([]byte, 1)
	for {
		_, err := r.Read(tmp)
		if err != nil {
			return "", err
		}
		if tmp[0] == '\n' {
			return string(buf), nil
		}
		buf = append(buf, tmp[0])
	}
}

func mustMsg(t *testing.T, r io.Reader) Message {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		s, err := readLine(r)
		if err != nil {
			done <- ""
			return
		}
		done <- s
	}()
	select {
	case s := <-done:
		var m Message
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			t.Fatal(err)
		}
		return m
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
		return Message{}
	}
}
