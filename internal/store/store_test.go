package store

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"lonchan/internal/model"
)

func TestLogRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.md")
	msg := model.LogMessage{ID: "aa", From: "bb", Time: "2026-11-13T08:00:00Z", Sig: "cc", Service: "svc", Body: "hello **there**"}
	if err := AppendLog(path, msg); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Body != msg.Body || got[0].From != "bb" || got[0].Sig != "cc" {
		t.Fatalf("%+v", got)
	}
	old := "<!-- lonchaney id=aa from=bb time=2026-11-13T08:00:00Z sig=cc service=svc -->\nhello\n<!-- /lonchaney -->\n"
	got, err = ParseLog(old)
	if err != nil || len(got) != 1 || got[0].Body != "hello" {
		t.Fatalf("old log %+v %v", got, err)
	}
}

func TestSeed(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Identities()) != 1 || len(s.Services()) != 2 || len(s.Contexts()) != 6 {
		t.Fatalf("seed counts id=%d svc=%d ctx=%d", len(s.Identities()), len(s.Services()), len(s.Contexts()))
	}
	owned := 0
	for _, c := range s.Contexts() {
		if c.Plugin != "" && c.ServiceID != "" && c.Config != nil {
			owned++
		}
	}
	if owned != 2 {
		t.Fatalf("connections %d", owned)
	}
	if s.Config().Skin != "plain" || s.Config().Layout == nil {
		t.Fatal("config")
	}
	if _, err := os.Stat(filepath.Join(dir, "keys", s.Identities()[0].ID+".json")); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Private(s.Identities()[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestProfile(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := s.Identities()[0]
	if len(id.Services) != 2 {
		t.Fatalf("services %v", id.Services)
	}
	id.Name = "Ada"
	id.Username = "ada"
	id.Pronouns = "she/her"
	id.Notes = []model.Note{{Name: "About", Text: "hello"}}
	if err := s.PutIdentity(id); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Identity(id.ID)
	if !ok || got.Username != "ada" || got.Pronouns != "she/her" || got.Notes[0].Text != "hello" || got.Public != id.Public {
		t.Fatalf("%+v", got)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIcon(id.ID, "face.png", buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Identity(id.ID)
	if got.Icon == "" || s.AbsIcon(got.Icon) == "" {
		t.Fatal(got.Icon)
	}
	if err := s.AssignService(id.ID, "svc-local", false); err != nil {
		t.Fatal(err)
	}
	svc, ok := s.Service("svc-local")
	if !ok || svc.IdentityID != "" {
		t.Fatalf("%+v", svc)
	}
	got, _ = s.Identity(id.ID)
	for _, sid := range got.Services {
		if sid == "svc-local" {
			t.Fatal("still mapped")
		}
	}
}

func TestWrap(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := s.Identities()[0].ID
	if err := s.Wrap("secret"); err != nil {
		t.Fatal(err)
	}
	locked, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if !locked.NeedsUnlock() {
		t.Fatal("expected lock")
	}
	if err := locked.Unlock("nope"); err == nil {
		t.Fatal("bad pass opened")
	}
	if err := locked.Unlock("secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := locked.Private(id); err != nil {
		t.Fatal(err)
	}
}

func TestPanesContextAddedToOlderStores(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteContext(PanesContextID); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := s.Context(PanesContextID); !ok || c.Kind != model.KindPanes {
		t.Fatalf("panes context %#v %v", c, ok)
	}
}

func TestConnectionRepair(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range s.Contexts() {
		if c.Plugin != "" {
			if err := s.DeleteContext(c.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var local, direct bool
	for _, c := range s.Contexts() {
		if c.Plugin == model.PluginLocal && c.ServiceID == "svc-local" {
			local = true
		}
		if c.Plugin == model.PluginDirect && c.ServiceID == "svc-direct" {
			direct = true
		}
	}
	if !local || !direct {
		t.Fatal("missing connection context")
	}
	if BindingOn(model.Config{}, "send") != true {
		t.Fatal("missing binding")
	}
	if BindingOn(model.Config{Enabled: map[string]bool{"send": false}}, "send") {
		t.Fatal("send stayed on")
	}
}
