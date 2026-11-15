package ui

import (
	"image"
	"image/color"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"lonchan/internal/icon"
	"lonchan/internal/model"
)

// avatar is a round mark: the identity's icon, or the first letter of a name.
type avatar struct {
	widget.BaseWidget
	name string
	icon string
	size float32
	tap  func()
}

func newAvatar(name, icon string, tap func()) *avatar {
	return newAvatarSize(name, icon, 28, tap)
}

func newAvatarSize(name, icon string, size float32, tap func()) *avatar {
	a := &avatar{name: name, icon: icon, size: size, tap: tap}
	a.ExtendBaseWidget(a)
	return a
}

func (a *avatar) Tapped(*fyne.PointEvent) {
	if a.tap != nil {
		a.tap()
	}
}

func (a *avatar) Cursor() desktop.Cursor {
	if a.tap == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (a *avatar) CreateRenderer() fyne.WidgetRenderer {
	px := int(a.size)
	if px < 1 {
		px = 1
	}
	img := a.rounded()
	if img == nil {
		img = letterDisc(a.name, px)
	}
	pic := canvas.NewImageFromImage(img)
	pic.FillMode = canvas.ImageFillContain
	pic.ScaleMode = canvas.ImageScaleSmooth
	return &avatarRenderer{size: a.size, objs: []fyne.CanvasObject{pic}}
}

func (a *avatar) rounded() image.Image {
	if a.icon == "" {
		return nil
	}
	b, err := os.ReadFile(a.icon)
	if err != nil {
		return nil
	}
	src, _, err := icon.Decode(a.icon, b)
	if err != nil {
		return nil
	}
	px := int(a.size)
	if px < 1 {
		px = 1
	}
	return roundImage(src, px)
}

type avatarRenderer struct {
	size float32
	objs []fyne.CanvasObject
}

func (r *avatarRenderer) Destroy()                     {}
func (r *avatarRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *avatarRenderer) MinSize() fyne.Size           { return fyne.NewSize(r.size, r.size) }
func (r *avatarRenderer) Refresh()                     {}
func (r *avatarRenderer) Layout(s fyne.Size) {
	for _, o := range r.objs {
		o.Resize(s)
		o.Move(fyne.NewPos(0, 0))
	}
}

var (
	avatarFontOnce sync.Once
	avatarFont     *opentype.Font
)

func avatarTypeface() *opentype.Font {
	avatarFontOnce.Do(func() {
		f, err := opentype.Parse(theme.DefaultTextBoldFont().Content())
		if err == nil {
			avatarFont = f
		}
	})
	return avatarFont
}

// letterDisc draws the first letter so its ink, not the font box, sits in the middle of the circle.
func letterDisc(name string, px int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, px, px))
	fillCircle(dst, avatarColor(name))
	face := avatarFace(float64(px) * 0.56)
	if face == nil {
		return dst
	}
	defer closeFace(face)
	scratch := image.NewNRGBA(image.Rect(0, 0, px*2, px*2))
	d := font.Drawer{Dst: scratch, Src: image.NewUniform(color.NRGBA{255, 255, 255, 255}), Face: face}
	d.Dot = fixed.P(px/2, px)
	d.DrawString(avatarLetter(name))
	minX, minY, maxX, maxY := px*2, px*2, 0, 0
	any := false
	for y := 0; y < px*2; y++ {
		for x := 0; x < px*2; x++ {
			if scratch.NRGBAAt(x, y).A == 0 {
				continue
			}
			any = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if !any {
		return dst
	}
	ox := (px - (maxX - minX + 1)) / 2
	oy := (px - (maxY - minY + 1)) / 2
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			p := scratch.NRGBAAt(x, y)
			if p.A == 0 {
				continue
			}
			blendWhite(dst, ox+(x-minX), oy+(y-minY), p.A)
		}
	}
	return dst
}

func blendWhite(dst *image.NRGBA, x, y int, a uint8) {
	if !image.Pt(x, y).In(dst.Bounds()) {
		return
	}
	i := dst.PixOffset(x, y)
	bg := color.NRGBA{dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3]}
	af := uint32(a)
	dst.Pix[i] = uint8((255*af + uint32(bg.R)*(255-af)) / 255)
	dst.Pix[i+1] = uint8((255*af + uint32(bg.G)*(255-af)) / 255)
	dst.Pix[i+2] = uint8((255*af + uint32(bg.B)*(255-af)) / 255)
	dst.Pix[i+3] = 255
}

func avatarFace(size float64) font.Face {
	f := avatarTypeface()
	if f == nil {
		return nil
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil
	}
	return face
}

func closeFace(face font.Face) {
	if c, ok := face.(interface{ Close() error }); ok {
		_ = c.Close()
	}
}

func fillCircle(dst *image.NRGBA, c color.Color) {
	b := dst.Bounds()
	px := b.Dx()
	radius := float64(px) / 2
	for y := 0; y < px; y++ {
		for x := 0; x < px; x++ {
			dx := float64(x) + 0.5 - radius
			dy := float64(y) + 0.5 - radius
			if dx*dx+dy*dy <= radius*radius {
				dst.Set(x, y, c)
			}
		}
	}
}

func roundImage(src image.Image, px int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, px, px))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw < 1 || sh < 1 {
		return dst
	}
	scale := float64(px) / float64(sw)
	if int(float64(sh)*scale+0.5) < px {
		scale = float64(px) / float64(sh)
	}
	dw := float64(sw) * scale
	dh := float64(sh) * scale
	ox := (dw - float64(px)) / 2
	oy := (dh - float64(px)) / 2
	radius := float64(px) / 2
	for y := 0; y < px; y++ {
		for x := 0; x < px; x++ {
			dx := float64(x) + 0.5 - radius
			dy := float64(y) + 0.5 - radius
			if dx*dx+dy*dy > radius*radius {
				continue
			}
			sx := b.Min.X + int((float64(x)+ox)/scale)
			sy := b.Min.Y + int((float64(y)+oy)/scale)
			if sx >= b.Max.X {
				sx = b.Max.X - 1
			}
			if sy >= b.Max.Y {
				sy = b.Max.Y - 1
			}
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func avatarLetter(name string) string {
	if name == "" {
		return "?"
	}
	r, _ := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError {
		return "?"
	}
	return string(r)
}

func avatarColor(name string) color.Color {
	palette := []color.NRGBA{
		{0x1B, 0x5E, 0xBF, 0xFF},
		{0x1E, 0x7A, 0x46, 0xFF},
		{0x9E, 0x2B, 0x2B, 0xFF},
		{0x6B, 0x4C, 0x9A, 0xFF},
		{0xB5, 0x6B, 0x1B, 0xFF},
		{0x0E, 0x74, 0x80, 0xFF},
	}
	n := 0
	for _, r := range name {
		n += int(r)
	}
	return palette[n%len(palette)]
}

func displayName(ui *UI, key string) string {
	if id, ok := ui.hub.Store.Identity(key); ok {
		return id.Label()
	}
	for _, id := range ui.hub.Store.Identities() {
		if key == id.Public || key == id.Name {
			return id.Label()
		}
	}
	return key
}

func (u *UI) iconFor(name string) string {
	for _, id := range u.hub.Store.Identities() {
		if name == id.Label() || name == id.Name || name == id.Username || name == id.Public {
			return u.hub.Store.AbsIcon(id.Icon)
		}
	}
	return ""
}

func peerBanner(ui *UI, ctx model.Context) fyne.CanvasObject {
	cards := peerCards(ui, ctx)
	if len(cards) == 0 {
		return widget.NewLabel("")
	}
	row := container.NewHBox()
	for _, c := range cards {
		name := widget.NewLabelWithStyle(c.name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		block := container.NewVBox(name)
		if c.detail != "" {
			sub := widget.NewLabel(c.detail)
			sub.Importance = widget.LowImportance
			block.Add(sub)
		}
		row.Add(container.NewHBox(newAvatar(c.name, c.icon, nil), block))
	}
	bg := newTintRect(theme.ColorNameBackground, 0xCC)
	return container.NewStack(bg, container.NewPadded(row))
}

type peerCard struct {
	name   string
	detail string
	icon   string
}

func peerCards(ui *UI, ctx model.Context) []peerCard {
	svc, _ := ui.hub.Store.Service(ctx.ServiceID)
	conn := connectionLabel(svc)
	self := ""
	if id, ok := ui.hub.Store.Identity(ctx.IdentityID); ok {
		self = id.Label()
	}
	one := func(name, addr string) peerCard {
		if name == "" {
			name = addr
		}
		var bits []string
		if id := matchIdentity(ui, name, addr); id.ID != "" {
			if id.Username != "" && !strings.EqualFold(id.Username, name) {
				bits = append(bits, "@"+id.Username)
			}
			if id.Pronouns != "" {
				bits = append(bits, id.Pronouns)
			}
		}
		if addr != "" && addr != name {
			bits = append(bits, addr)
		}
		if conn != "" {
			bits = append(bits, conn)
		}
		return peerCard{name: name, detail: strings.Join(bits, " · "), icon: ui.iconFor(name)}
	}
	if ctx.TargetKind == model.TargetGroup {
		var out []peerCard
		for _, m := range ui.groupMembers(ctx) {
			n := displayName(ui, m)
			if n == "" || n == self {
				continue
			}
			out = append(out, one(n, m))
		}
		if len(out) == 0 && (ctx.Name != "" || ctx.Target != "") {
			out = append(out, one(ctx.Name, ctx.Target))
		}
		return out
	}
	if ctx.Target == "" && ctx.Name == "" {
		return nil
	}
	return []peerCard{one(ctx.Name, ctx.Target)}
}

func matchIdentity(ui *UI, name, addr string) model.Identity {
	for _, id := range ui.hub.Store.Identities() {
		if addr != "" && (id.ID == addr || id.Public == addr || id.Label() == addr || id.Name == addr || id.Username == addr) {
			return id
		}
		if name != "" && (id.Public == name || id.Label() == name || id.Name == name || id.Username == name) {
			return id
		}
	}
	return model.Identity{}
}

func fade(c color.Color, a uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), a}
}

func (u *UI) showIdentityPop(ctx model.Context) {
	if u.win == nil {
		return
	}
	box := container.NewVBox(widget.NewLabel("Speak as"))
	var pop *widget.PopUp
	for _, id := range u.hub.Store.Identities() {
		ident := id
		label := ident.Label()
		btn := widget.NewButton(label, func() {
			if pop != nil {
				pop.Hide()
			}
			u.setIdentity(ctx, ident.ID)
		})
		btn.Alignment = widget.ButtonAlignLeading
		btn.Importance = widget.LowImportance
		if ident.ID == ctx.IdentityID {
			btn.Importance = widget.HighImportance
		}
		box.Add(container.NewBorder(nil, nil, newAvatar(label, u.hub.Store.AbsIcon(ident.Icon), nil), nil, btn))
	}
	card := container.NewPadded(box)
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameMenuBackground))
	bg.CornerRadius = 8
	pop = widget.NewModalPopUp(container.NewStack(bg, card), u.win.Canvas())
	pop.Show()
}
