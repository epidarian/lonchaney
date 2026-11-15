package ui

import (
	"math"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lonchan/internal/caps"
	"lonchan/internal/hub"
	"lonchan/internal/layout"
	"lonchan/internal/model"
	"lonchan/internal/skin"
	"lonchan/internal/store"
)

func TestComposerHeight(t *testing.T) {
	box := composerBox(widget.NewLabel("e"), widget.NewLabel("s"))
	if box.MinSize().Height != composerHeight() {
		t.Fatalf("composer height %v", box.MinSize().Height)
	}
}

func TestComposerDragResizes(t *testing.T) {
	p := newComposerPanel(widget.NewMultiLineEntry(), widget.NewLabel("bar"), nil, 4, 36, nil)
	before := p.MinSize().Height
	msg := newEdgeGrip(p, false)
	msg.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(0, -80)})
	if p.MinSize().Height <= before {
		t.Fatalf("message drag %v -> %v", before, p.MinSize().Height)
	}
	mid := p.MinSize().Height
	area := newEdgeGrip(p, true)
	area.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(0, -40)})
	if p.MinSize().Height <= mid {
		t.Fatalf("bar drag %v -> %v", mid, p.MinSize().Height)
	}
	transcript := widget.NewLabel("t")
	frame := NewFrame(skin.Plain(), "conversation", map[string]fyne.CanvasObject{
		"header": widget.NewLabel("h"), "transcript": transcript, "composer": p,
	}, nil)
	frame.CreateRenderer().Layout(fyne.NewSize(1000, 1000))
	if transcript.Size().Height >= 680 {
		t.Fatalf("transcript stayed %v", transcript.Size())
	}
	if p.Position().Y+p.Size().Height != 960 {
		t.Fatalf("composer bottom %v size %v", p.Position(), p.Size())
	}
}

func TestDesktopPass(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := test.NewApp()
	w := a.NewWindow("lonchan")
	w.Resize(fyne.NewSize(1200, 800))
	u := &UI{app: a, win: w, hub: hub.New(st), scrollOff: map[string]fyne.Position{}, focused: "pane-chat"}
	a.Settings().SetTheme(newLonTheme(st.Config().Theme))
	u.hub.On = func() {}
	w.SetContent(u.shellRoot())

	root := st.Config().Layout
	if math.Abs(root.Ratio-layout.OuterRatio) > 1e-9 {
		t.Fatalf("outer ratio %v", root.Ratio)
	}
	if root.Trailing == nil || math.Abs(root.Trailing.Ratio-layout.InnerRatio) > 1e-9 {
		t.Fatalf("inner ratio %v", root.Trailing)
	}

	u.mountInPane("pane-chat", "ctx-select")
	if layout.FindPane(st.Config().Layout, "pane-chat").ContextID != "ctx-select" {
		t.Fatal("selection did not mount in the center pane")
	}
	u.mountInPane("pane-chat", "ctx-chat")
	if layout.FindPane(st.Config().Layout, "pane-chat").ContextID != "ctx-chat" {
		t.Fatal("chat did not mount back into the center pane")
	}

	first := st.Identities()[0]
	second, err := st.AddIdentity("second")
	if err != nil {
		t.Fatal(err)
	}
	one, err := u.hub.EnsureConversation("svc-local", first.ID, model.TargetGroup, "room", "room")
	if err != nil {
		t.Fatal(err)
	}
	two := model.Context{
		ID: "ctx-two", Name: "room", Kind: model.KindConversation,
		ServiceID: "svc-local", IdentityID: second.ID, TargetKind: model.TargetGroup, Target: "room",
	}
	if err := st.PutContext(two); err != nil {
		t.Fatal(err)
	}
	if err := u.hub.Send(one, "from-one"); err != nil {
		t.Fatal(err)
	}
	if err := u.hub.Send(two, "from-two"); err != nil {
		t.Fatal(err)
	}
	msgs, err := st.ReadLog("svc-local", "room")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Body != "from-one" || msgs[1].Body != "from-two" {
		t.Fatalf("local room log %#v", msgs)
	}

	directSvc, ok := st.Service("svc-direct")
	if !ok {
		t.Fatal("missing svc-direct")
	}
	directSvc.Config = map[string]string{"bind": "127.0.0.1:0"}
	if err := st.PutService(directSvc); err != nil {
		t.Fatal(err)
	}
	if err := u.hub.Connect(directSvc); err != nil {
		t.Fatal(err)
	}
	addr := strings.TrimPrefix(u.hub.Status("svc-direct"), "listening ")
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("listener %q", u.hub.Status("svc-direct"))
	}
	peerSvc := model.Service{ID: "svc-peer", Plugin: model.PluginDirect, IdentityID: second.ID, Config: map[string]string{}}
	if err := st.PutService(peerSvc); err != nil {
		t.Fatal(err)
	}
	if err := u.hub.Connect(peerSvc); err != nil {
		t.Fatal(err)
	}
	out, err := u.hub.EnsureConversation("svc-peer", second.ID, model.TargetUser, addr, "direct")
	if err != nil {
		t.Fatal(err)
	}
	if err := u.hub.Send(out, "ping"); err != nil {
		t.Fatal(err)
	}
	var inbound []model.LogMessage
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		inbound, _ = st.ReadLog("svc-direct", second.Public)
		if len(inbound) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(inbound) == 0 || inbound[len(inbound)-1].Body != "ping" {
		t.Fatalf("loopback log %#v", inbound)
	}
	pending := u.hub.Pending()
	if len(pending) == 0 {
		t.Fatal("inbound peer was not offered")
	}
	accepted, err := u.hub.Accept(pending[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Target != pending[0].Key || accepted.ServiceID != "svc-direct" {
		t.Fatalf("accept %#v", accepted)
	}

	irc := model.Service{ID: "svc-irc", Plugin: model.PluginIRC, IdentityID: first.ID, Config: map[string]string{}}
	if err := st.PutService(irc); err != nil {
		t.Fatal(err)
	}
	chat, _ := st.Context("ctx-chat")
	chat.ServiceID = irc.ID
	if err := st.PutContext(chat); err != nil {
		t.Fatal(err)
	}
	suite := NewSuite(u)
	box := suite.CreateRenderer().Objects()[0].(*fyne.Container)
	var sawBold, sawHeading bool
	for _, obj := range box.Objects {
		if f, ok := obj.(*Frame); ok {
			obj = f.Fallback
		}
		tool := obj.(*tool)
		if tool.label == "bold" {
			sawBold = true
			if tool.grey {
				t.Fatal("bold tool grey on IRC")
			}
		}
		if tool.label == "heading" {
			sawHeading = true
			if !tool.grey {
				t.Fatal("heading tool not grey on IRC")
			}
		}
	}
	if !sawBold || !sawHeading {
		t.Fatal("suite missing bold or heading")
	}
	u.hovered = caps.Heading
	header := u.paneHeader(layout.FindPane(st.Config().Layout, "pane-chat"), chat)
	if _, ok := header.(*fyne.Container).Objects[0].(*widget.Label); !ok {
		t.Fatal("IRC header stayed active while heading was hovered")
	}

	u.openSettings()
	if !u.inSettings {
		t.Fatal("settings did not open")
	}
	if _, ok := u.win.Content().(*Split); !ok {
		t.Fatalf("settings content %T", u.win.Content())
	}
	if err := st.UpdateConfig(func(c *model.Config) {
		c.Theme.Colors["primary"] = "#112233"
		c.Theme.Preset = "Custom"
	}); err != nil {
		t.Fatal(err)
	}
	u.applyTheme()
	got := a.Settings().Theme().Color(theme.ColorNamePrimary, theme.VariantLight)
	if got != skin.ParseHex("#112233") {
		t.Fatalf("theme color %v", got)
	}
	btn := a.Settings().Theme().Color(theme.ColorNameButton, theme.VariantDark)
	br, bgc, bb, _ := btn.RGBA()
	if br < 0x8000 || bgc < 0x8000 || bb < 0x8000 {
		t.Fatalf("light page used a dark button color %v", btn)
	}

	chat.Draft = "still here"
	if err := st.PutContext(chat); err != nil {
		t.Fatal(err)
	}
	u.closeSettings()
	if u.inSettings {
		t.Fatal("settings stayed open")
	}
	entry := findComposer(u.win.Content(), map[fyne.CanvasObject]bool{})
	if entry == nil || entry.Text != "still here" {
		t.Fatalf("draft after settings %#v", entry)
	}

	headerObj := widget.NewLabel("h")
	transcript := widget.NewLabel("t")
	composer := widget.NewLabel("c")
	frame := NewFrame(skin.Plain(), "conversation", map[string]fyne.CanvasObject{
		"header": headerObj, "transcript": transcript, "composer": composer,
	}, nil)
	frame.CreateRenderer().Layout(fyne.NewSize(1000, 1000))
	if transcript.Position() != fyne.NewPos(40, 120) || transcript.Size() != fyne.NewSize(920, 680) {
		t.Fatalf("transcript at %v size %v", transcript.Position(), transcript.Size())
	}

	u.mountInPane("pane-chat", "ctx-select")
	u.resetLayout()
	if math.Abs(st.Config().Layout.Ratio-layout.OuterRatio) > 1e-9 {
		t.Fatal("reset outer")
	}
	if layout.FindPane(st.Config().Layout, "pane-chat").ContextID != "ctx-chat" {
		t.Fatal("reset did not restore the chat pane")
	}
	if st.Config().SettingsRatio != 0.20 {
		t.Fatal("reset settings ratio")
	}
}

func findComposer(o fyne.CanvasObject, seen map[fyne.CanvasObject]bool) *composer {
	if o == nil || seen[o] {
		return nil
	}
	seen[o] = true
	if c, ok := o.(*composer); ok {
		return c
	}
	var kids []fyne.CanvasObject
	switch w := o.(type) {
	case *fyne.Container:
		kids = w.Objects
	case fyne.Widget:
		kids = w.CreateRenderer().Objects()
	}
	for _, kid := range kids {
		if c := findComposer(kid, seen); c != nil {
			return c
		}
	}
	return nil
}
