package ircfmt

import "testing"

func TestMap(t *testing.T) {
	got := ToIRC("**hi** _there_ ~~x~~")
	if got != "\x02hi\x02 \x1dthere\x1d \x1ex\x1e" {
		t.Fatalf("%q", got)
	}
	back := FromIRC(got)
	if back != "**hi** _there_ ~~x~~" {
		t.Fatalf("%q", back)
	}
}
