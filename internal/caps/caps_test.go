package caps

import "testing"

func TestGrey(t *testing.T) {
	irc := IRC()
	full := Full()
	xmpp := XMPP()
	if !ToolGrey(irc, Heading) || ToolGrey(irc, Bold) {
		t.Fatal("irc tools")
	}
	if !HeaderGrey(irc, Heading) || HeaderGrey(irc, Bold) {
		t.Fatal("irc header")
	}
	if !SpanGrey([]Mark{Heading}, irc, Heading) || SpanGrey([]Mark{Bold}, irc, Bold) {
		t.Fatal("irc span")
	}
	if ToolGrey(full, List) || HeaderGrey(full, Bold) {
		t.Fatal("full")
	}
	if !ToolGrey(xmpp, Heading) || !ToolGrey(xmpp, List) || !ToolGrey(xmpp, Link) {
		t.Fatal("xmpp grey")
	}
	if ToolGrey(xmpp, Quote) || ToolGrey(xmpp, CodeBlock) {
		t.Fatal("xmpp live")
	}
	if SpanGrey([]Mark{Bold}, xmpp, "") {
		t.Fatal("no hover")
	}
}
