package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/solarlune/ldtkgo"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	tilesize    = 16
	plsize      = tilesize
	introticks  = 90
	explticks   = 3
	deathticks  = 12
	basefontsiz = 16
	basefontlin = 8
	shkamnt     = 30
	shkpik      = 60
	scorespeed  = 0.5
	tileprice   = 0.25
	flickerpat  = `` +
		`001001011111010000000000010000111111111111111111100000000000` +
		`000001111111111111000000000000100000000000000000101001010100` +
		`001011011011020010102020201212012210000000121233200213010100` +
		`213010021021300213335699787642153612312120010210201020010100` +
		`000000101001010021023030012020030303030404333334432222100000` +
		`000000000000000000000000000000000000000000000000000000000000` +
		`000000000000000000000000000000011111110000000111111110000000` +
		`000000000000000000000000000000000000000000000000000000000000`
)

var (
	redcol   = color.RGBA{0xd5, 0x1a, 0x3d, 0xff}
	orangcol = color.RGBA{0xff, 0xc9, 0x00, 0xff}
	blucol   = color.RGBA{0x63, 0x9b, 0xff, 0xff}
)

const (
	dstill = iota
	dup
	dleft
	dright
	ddown
)

const (
	ftarg = iota
	ftoas
	freg
)

const (
	sinit = iota
	sintro
	stitle
	stitle2
	splay
	sclear
	sdead
	sendgame
)

type flammable struct {
	typ uint
	w   uint
	h   uint
	img *ebiten.Image
	rot int

	dur   float64
	x     float64
	y     float64
	dead  bool
	deadt int
}

var (
	plsprites   = make([]*ebiten.Image, 0)
	explspts    = make([]*ebiten.Image, 0)
	spritesheet = make(map[int]*ebiten.Image)
	collider    = make(map[int]struct{})
	dbgstr      string
	intropic    *ebiten.Image
	logopic     *ebiten.Image
	lqwhite     *ebiten.Image
	dfont       font.Face
	playexpl    func()
	playdeaf    func()
	playspawn   func()
	introbgm    func(bool) *audio.Player
	normalbgm   func(bool) *audio.Player
	outrobgm    func(bool) *audio.Player
	flamimgs    = make(map[string]*ebiten.Image)
	dead11      *ebiten.Image
	dead22      *ebiten.Image
	dead21      *ebiten.Image
)

type game struct {
	ldtk  *ldtkgo.Project
	l2    *ldtkgo.Layer
	walls *ldtkgo.Layer
	floor *ldtkgo.Layer

	flams []flammable

	score     int
	newscore  int
	plx       float64
	ply       float64
	lvlw      float64
	lvlh      float64
	stamina   float64
	origsta   float64
	shkticker uint
	dead      bool
	lvl       int

	state int
	tick  uint
	view  *ebiten.Image
	bgm   func(stfu bool) *audio.Player
	buzz  func(stfu bool) *audio.Player
}

func swstate(g *game, state int) {
	g.state = state
	g.tick = 0
}

func parseflams(ent []*ldtkgo.Entity) []flammable {
	fls := make([]flammable, 0, 20)
	for _, e := range ent {
		fl := flammable{
			w:    uint(e.Width / tilesize),
			h:    uint(e.Height / tilesize),
			x:    float64(e.Position[0]) / tilesize,
			y:    float64(e.Position[1]) / tilesize,
			dead: false,
		}
		fl.dur = float64(fl.w*fl.h) * tileprice
		switch e.Identifier {
		case "Tv":
			fl.rot = e.PropertyByIdentifier("Rot").AsInt()
			fallthrough
		case "Microwave":
			fallthrough
		case "Toaster":
			fl.typ = freg
			pf := e.PropertyByIdentifier("Type").Value
			s := ""
			if pf == nil {
				s = e.Identifier
			} else {
				s = pf.(string)
			}
			fl.img = flamimgs[s]
		case "Target":
			fl.typ = ftarg
			fl.dur = 0
			fl.rot = e.PropertyByIdentifier("Rot").AsInt()
			fl.img = flamimgs["Target"]
		}
		if e.Identifier != "Player" {
			fls = append(fls, fl)
		}
	}
	return fls
}

func suck(g *game) {
	// dbgstr = ""
	for i, e := range g.flams {
		if e.dead {
			g.flams[i].deadt++
			continue
		}
		r := math.Sqrt(float64(e.w*e.h)/math.Pi) * 1.5
		x := e.x + float64(e.w)/2
		y := e.y + float64(e.h)/2
		plx, ply := x-g.plx, y-g.ply
		dis := math.Sqrt(plx*plx + ply*ply)
		if dis <= r {
			// dbgstr = fmt.Sprint(g.score, e.typ, e.dur, e.dead)
			g.stamina -= 0.05
			g.flams[i].dur -= 0.05
			g.newscore += 5
			if e.dur <= 0 {
				g.flams[i].dead = true
				g.shkticker = 60
				playexpl()
			}
			if e.typ == ftarg {
				g.newscore += int(g.stamina * 10)
				swstate(g, sclear)
			}
		}
	}
}

func drawsuck(g *game, img *ebiten.Image) {
	w := float64(img.Bounds().Dx())
	h := float64(img.Bounds().Dy())
	for _, e := range g.flams {
		if e.dead && e.deadt < 48 {
			op := ebiten.DrawImageOptions{}
			if e.typ == freg {
				op.GeoM.Translate(
					(float64(e.x)-g.plx)*tilesize+(w)/2+tilesize/2,
					(float64(e.y)-g.ply)*tilesize+(h-2*plsize)/2+tilesize*2,
				)
			} else {
				op.GeoM.Translate(
					(float64(e.x)-g.plx)*tilesize+(w)/2,
					(float64(e.y)-g.ply)*tilesize+(h-2*plsize)/2+tilesize,
				)
			}
			op.CompositeMode = ebiten.CompositeModeLighter
			img.DrawImage(explspts[e.deadt/4], &op)
		}
	}
}

func drawintro(g *game, img *ebiten.Image) {
	ft := float64(g.tick)
	fade := 2 * (ft / introticks)
	if fade >= 1 {
		fade = 2 * (1 - ft/introticks)
	}
	op := ebiten.DrawImageOptions{}
	op.ColorM.ChangeHSV(0, 0, fade)
	img.DrawImage(intropic, &op)
}

func updplay(g *game) {
	const speed = 0.05
	const jitter = 0.06

	pplx, pply := g.plx, g.ply
	g.stamina -= 0.01
	if g.stamina < 0 {
		g.dead = true
		g.state = sdead
	}

	suck(g)

	switch {
	case ebiten.IsKeyPressed(ebiten.KeyW) && ebiten.IsKeyPressed(ebiten.KeyA):
		g.ply -= speed / math.Sqrt2
		g.plx -= speed / math.Sqrt2
	case ebiten.IsKeyPressed(ebiten.KeyA) && ebiten.IsKeyPressed(ebiten.KeyS):
		g.plx -= speed / math.Sqrt2
		g.ply += speed / math.Sqrt2
	case ebiten.IsKeyPressed(ebiten.KeyS) && ebiten.IsKeyPressed(ebiten.KeyD):
		g.ply += speed / math.Sqrt2
		g.plx += speed / math.Sqrt2
	case ebiten.IsKeyPressed(ebiten.KeyD) && ebiten.IsKeyPressed(ebiten.KeyW):
		g.plx += speed / math.Sqrt2
		g.ply -= speed / math.Sqrt2
	case ebiten.IsKeyPressed(ebiten.KeyW):
		g.ply -= speed
	case ebiten.IsKeyPressed(ebiten.KeyA):
		g.plx -= speed
	case ebiten.IsKeyPressed(ebiten.KeyS):
		g.ply += speed
	case ebiten.IsKeyPressed(ebiten.KeyD):
		g.plx += speed
	}

	g.plx += (rand.Float64() - 0.5) * 2 * jitter
	g.ply += (rand.Float64() - 0.5) * 2 * jitter

	plux := int(math.Trunc(g.plx))
	pluy := int(math.Trunc(g.ply))

	tl := g.walls.AutoTileAt(plux, pluy)
	f := func() bool {
		_, k := collider[tl.ID]
		return k
	}
	if tl != nil && f() || !inrang(g) {
		g.stamina -= 0.1
		g.ply = pply
		g.plx = pplx

	}
}

func inrang(g *game) bool {
	return g.plx > 0 && g.plx < g.lvlw/tilesize &&
		g.ply > 0 && g.ply < g.lvlh/tilesize
}

func drawpl(g *game, screen *ebiten.Image) {
	W := float64(screen.Bounds().Dx())
	H := float64(screen.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(
		(W-plsize)/2,
		(H-3*plsize)/2,
	)
	screen.DrawImage(plsprites[(g.tick/3)%6], op)
}

func drawsprites(g *game, screen *ebiten.Image) {
	W := float64(screen.Bounds().Dx())
	H := float64(screen.Bounds().Dx())
	spr := func(sp []*ldtkgo.Tile) {
		for _, t := range sp {
			op := &ebiten.DrawImageOptions{}
			// fuck ldtkgo
			i := t.Position[0] / tilesize
			j := t.Position[1] / tilesize
			op.GeoM.Translate(
				(float64(i)-g.plx)*tilesize+(W)/2,
				(float64(j)-g.ply)*tilesize+(H-2*plsize)/2,
			)
			screen.DrawImage(spritesheet[t.ID], op)
		}
	}
	spr(g.floor.AllTiles())
	spr(g.walls.AllTiles())
	spr(g.l2.AllTiles())
}

func drawstaminabar(scr *ebiten.Image, sta float64, maxsta float64) {
	w := float64(scr.Bounds().Dx())
	ebitenutil.DrawRect(scr, 0, 0, w*sta/maxsta, 4, color.RGBA{0xac, 0x1f, 0x9f, 0xff})
}

func printlable(screen *ebiten.Image, lable []string, x, y int, c color.Color) {
	skip := int(math.Ceil(basefontlin * 1.2))
	basey := y - (len(lable)-1)*(skip)/2
	for i, l := range lable {
		r := text.BoundString(dfont, l)
		cx := r.Bounds().Dx() / 2
		x := x - cx
		y := basey + i*skip
		text.Draw(screen, l, dfont, x, y, c)
	}
}

func printleft(screen *ebiten.Image, lable []string, x, y int, c color.Color) {
	skip := int(math.Ceil(basefontlin * 1.2))
	basey := y - (len(lable)-1)*(skip)/2
	for i, l := range lable {
		// r := text.BoundString(dfont, l)
		// cx := r.Bounds().Dx() / 2
		x := x
		y := basey + i*skip
		text.Draw(screen, l, dfont, x, y, c)
	}
}

func drawgroza(img *ebiten.Image, tick int) {
	const maxu24 = float64(^uint32(0) >> 16)
	pat := float64((flickerpat[(tick/4)%len(flickerpat)] - '0')) / ('9' - '0')
	colv := uint32(maxu24 * pat)
	col := color.RGBA{uint8((colv >> 16) & 0xff), uint8((colv >> 8) & 0xff), uint8(colv & 0xff), 0xff}
	img.Fill(col)
}

func drawmenu(g *game, screen *ebiten.Image) {
	screen.DrawImage(logopic, nil)
	lable := []string{
		`made by neputevshina, exiphase`,
		`and DISN for ludum dare 49`,
		`and lulz in three days`,
		`of october 2021`,
	}
	blink := []string{`press any of wasd`}
	blink2 := []string{`EPILEPSY WARNING`}
	scx := screen.Bounds().Dx() / 2
	sh := screen.Bounds().Dy()
	printlable(screen, lable, scx, 6*sh/7, color.White)
	if (g.tick/30)%2 == 0 {
		printlable(screen, blink, scx, 9*sh/14-8, orangcol)
	}
	if (g.tick/31)%2 == 0 {
		printlable(screen, blink2, scx, 10*sh/14-3, blucol)
	}
}

func drawmenu2(g *game, screen *ebiten.Image) {
	lable := []string{
		`destroy electronics. destroy the purple`,
		`energy counter to complete the level.`,
		`your charge is going down so better hurry.`,
		`don't hit the walls!`,
	}
	blink := []string{`press any of wasd`, `to continue`}
	scx := screen.Bounds().Dx() / 2
	sh := screen.Bounds().Dy()
	printlable(screen, lable, scx, 3*sh/7, color.White)
	if (g.tick/30)%2 == 0 {
		printlable(screen, blink, scx, 13*sh/14, orangcol)
	}
}

func x4(f func(screen *ebiten.Image, lable []string, x, y int, c color.Color)) func(
	screen *ebiten.Image, lable []string, x, y int, c color.Color) {
	return func(screen *ebiten.Image, lable []string, x, y int, c color.Color) {
		printlable(screen, lable, x+1, y, c)
		printlable(screen, lable, x-1, y, c)
		printlable(screen, lable, x, y+1, c)
		printlable(screen, lable, x, y-1, c)
	}
}

func drawoutro(g *game, screen *ebiten.Image) {
	lable1 := []string{`thanks for playing!`}
	lable2 := []string{
		`barabannaya matematika are...`,
		`code: neputevshina`,
		`music and sfx and idea: exiphase (kuamarin)`,
		`art and levels: DISN and other two`,
	}
	lable3 := []string{
		`made from scratch using go`,
		`and ebiten and ldtk and ldtkgo`,
		`and krita and gimp`,
		`and fl studio and audacity`,
	}
	scx := screen.Bounds().Dx() / 2
	sh := screen.Bounds().Dy()

	op := ebiten.DrawImageOptions{}
	dx := float64(intropic.Bounds().Dx()) / 2
	dy := float64(intropic.Bounds().Dy()) / 2
	printlable(intropic, []string{fmt.Sprint("your score is ", g.newscore)},
		100, int(dy/2), orangcol)
	op.GeoM.Translate(-dx, -dy)
	s := (math.Sin(float64(g.tick)/32) + 2) / 2
	op.GeoM.Scale(s, s)
	op.GeoM.Rotate(float64(g.tick) / (math.Pi * 8))
	op.GeoM.Translate(dx, dy)
	op.ColorM.ChangeHSV(1, 1, 0.5)
	screen.DrawImage(intropic, &op)

	x4(printlable)(screen, lable1, scx, 1*sh/7, color.Black)
	printlable(screen, lable1, scx, 1*sh/7, blucol)
	printleft(screen, lable2, 4, 3*sh/7, color.White)
	printleft(screen, lable3, 4, 5*sh/7, color.White)
	link := []string{`github.com)neputevshina)ldjam49`}
	x4(printlable)(screen, link, scx, 13*sh/14, color.Black)
	printlable(screen, link, scx, 13*sh/14, orangcol)
}

func drawflams(g *game, img *ebiten.Image) {
	W := float64(img.Bounds().Dx())
	H := float64(img.Bounds().Dx())
	for _, e := range g.flams {
		op := ebiten.DrawImageOptions{}
		op.GeoM.Translate(-float64(e.w*tilesize)/2, -float64(e.h*tilesize)/2)
		op.GeoM.Rotate(float64(e.rot) * math.Pi / 2)
		op.GeoM.Translate(float64(e.w*tilesize)/2, float64(e.h*tilesize)/2)
		op.GeoM.Translate(
			(e.x-g.plx)*tilesize+(W)/2,
			(e.y-g.ply)*tilesize+(H-2*plsize)/2,
		)
		if !e.dead {
			img.DrawImage(e.img, &op)
		} else {
			if e.h == 1 && e.w == 1 {
				img.DrawImage(dead11, &op)
			} else if e.h == 1 && e.w == 2 {
				img.DrawImage(dead21, &op)
			} else if e.h == 2 && e.w == 2 {
				img.DrawImage(dead22, &op)
			}
		}
	}
}

func anykey() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeyW) ||
		inpututil.IsKeyJustPressed(ebiten.KeyA) ||
		inpututil.IsKeyJustPressed(ebiten.KeyS) ||
		inpututil.IsKeyJustPressed(ebiten.KeyD)
}

func updmenu(g *game) {
	if anykey() {
		swstate(g, stitle2)
	}
}

func updmenu2(g *game) {
	if anykey() {
		swstate(g, splay)
		g.lvl = 0
		loadlevel(g, g.lvl)
	}
}

func (g *game) Update() error {
	defer func() { g.tick++ }()
	if g.shkticker != 0 {
		g.shkticker--
	}
	switch g.state {
	case sinit:
		gameinit(g)
		loadlevel(g, 0)
		swstate(g, sintro)
	case sintro:
		if g.tick > introticks {
			swstate(g, stitle)
		}
	case stitle:
		updmenu(g)
		g.bgm(false)
	case stitle2:
		updmenu2(g)
		g.bgm(false)
		g.score = 0
	case splay:
		if g.tick == 1 {
			g.score = g.newscore
			playspawn()
			swbgm(g, normalbgm)
		}
		g.bgm(false)
		updplay(g)
	case sclear:
		if g.tick == 1 {
			playdeaf()
		}
		if g.tick > 60 && anykey() {
			g.lvl++
			if g.lvl < len(g.ldtk.Levels) {
				loadlevel(g, g.lvl)
				g.bgm = nil
				swstate(g, splay)
			} else {
				swstate(g, sendgame)
			}
		}
	case sdead:
		g.bgm(true)
		if g.tick > 120 && anykey() {
			loadlevel(g, g.lvl)
			g.newscore = 0
			g.score = 0
			swstate(g, stitle)
		}
	case sendgame:
		if g.tick == 1 {
			g.score = g.newscore
			swbgm(g, outrobgm)
		}
		g.bgm(false)
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	// dbgstr = fmt.Sprint("w: ", g.lvlw, ", h: ", g.lvlh)
	defer ebitenutil.DebugPrint(screen, dbgstr)
	var fade float64
	switch g.state {
	case sintro:
		drawintro(g, screen)
	case stitle:
		drawmenu(g, screen)
	case stitle2:
		drawmenu2(g, screen)
	case sdead:
		ft := float64(g.tick)
		fade = 2 * (ft / deathticks)
		if fade > 1 {
			fade = 1
		}
		screen.Fill(color.White)
		w := screen.Bounds().Dx()
		h := screen.Bounds().Dy()
		cw := w / 2
		ch := h / 2
		op := ebiten.DrawImageOptions{}
		op.ColorM.Translate(0, -1, -1, fade-1)
		drawplayfield(g, screen)
		if g.tick < 60 {
			drawplayfield(g, screen)
			screen.DrawImage(lqwhite, &op)
			return
		}
		screen.Fill(color.RGBA{0xff, 0, 0, 0xff})
		printlable(screen, []string{"YOU ARE DEAD"}, cw, ch, color.Black)
		if float64(g.tick) >= 90 && (g.tick/30)%2 == 0 {
			blink := []string{`press any of wasd`, `to go to title screen`}
			printlable(screen, blink, cw, 13*h/14, color.Black)
		}

	case sclear:
		ft := float64(g.tick)
		fade = 2 * (ft / explticks)
		if fade > 1 {
			fade = 1
		}
		op := ebiten.DrawImageOptions{}
		op.ColorM.Translate(0, 0, 0, fade-1)
		if g.tick < 60 {
			drawplayfield(g, screen)
			screen.DrawImage(lqwhite, &op)
			return
		}
		screen.Fill(color.White)
		w := screen.Bounds().Dx()
		h := screen.Bounds().Dy()
		cw := w / 2
		ch := h / 2
		printlable(screen, []string{"Level complete"}, cw, ch,
			redcol)

		scorepoint := 90 + float64(g.newscore-g.score)*scorespeed
		if g.tick >= 90 {
			if float64(g.tick) < scorepoint {
				cur := float64(g.score) + float64(g.tick-90)/scorespeed
				tickstr := fmt.Sprintf("%.0f", cur)
				printlable(screen, []string{tickstr}, cw, ch+16, redcol)
			} else {
				printlable(screen, []string{fmt.Sprint(g.newscore)}, cw, ch+16, orangcol)
			}
		}
		if float64(g.tick) >= scorepoint && (g.tick/30)%2 == 0 {
			blink := []string{`press any of wasd`, `to continue`}
			printlable(screen, blink, cw, 13*h/14, blucol)
		}
	case splay:
		drawplayfield(g, screen)
		drawsuck(g, screen)
		drawstaminabar(screen, g.stamina, g.origsta)
	case sendgame:
		drawoutro(g, screen)
	}
}

func drawplayfield(g *game, screen *ebiten.Image) {
	drawgroza(g.view, int(g.tick))
	drawsprites(g, g.view)
	drawflams(g, g.view)
	drawpl(g, g.view)
	op := &ebiten.DrawImageOptions{}
	r := func() float64 {
		return rand.Float64() * shkamnt * float64(g.shkticker) / shkpik
	}
	op.GeoM.Translate(r(), r())
	screen.DrawImage(g.view, op)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 200, 160
}

func newslicer(data []byte, s int) func(x, y int) *ebiten.Image {
	return newslicer2(data, s, s)
}

func newslicer2(data []byte, w, h int) func(x, y int) *ebiten.Image {
	atlimg, _, _ := image.Decode(bytes.NewReader(data))
	atlas := ebiten.NewImageFromImage(atlimg)
	return func(x, y int) *ebiten.Image {
		x, y = x*w, y*h
		return ebiten.NewImageFromImage(atlas.SubImage(image.Rect(x, y, x+w, y+h)))
	}
}

func wholeimg(data []byte) *ebiten.Image {
	iimg, _, _ := image.Decode(bytes.NewReader(data))
	return ebiten.NewImageFromImage(iimg)
}

func swbgm(g *game, f func(bool) *audio.Player) {
	if g.bgm != nil {
		g.bgm(true)
	}
	g.bgm = f
}

func gameinit(g *game) {
	s11 := newslicer(atlas2, tilesize)
	for j := range [16]int{} {
		for i := range [16]int{} {
			spritesheet[i+16*j] = s11(i, j)
		}
	}

	s22 := newslicer(atlas2, tilesize*2)
	s21 := newslicer2(atlas2, tilesize*2, tilesize)
	flamimgs["Tv"] = s22(0, 1)
	flamimgs["Wash"] = s22(1, 1)
	flamimgs["Microwave"] = s21(0, 4)
	flamimgs["Toaster"] = s11(2, 4)
	flamimgs["Target"] = s11(3, 4)
	flamimgs[""] = s11(2, 4)

	dead11 = newslicer(dead11dat, tilesize)(0, 0)
	dead21 = newslicer2(dead21dat, tilesize*2, tilesize)(0, 0)
	dead22 = newslicer(dead22dat, tilesize*2)(0, 0)

	plat := newslicer(kozin, plsize)
	for i := range [6]int{} {
		plsprites = append(plsprites, plat(0, i))
	}

	intropic = wholeimg(introdat)
	logopic = wholeimg(logodat)
	lqwhite = ebiten.NewImage(g.view.Size())
	lqwhite.Fill(color.White)

	tt, err := opentype.Parse(fontdat)
	if err != nil {
		log.Fatal(err)
	}
	const dpi = 72
	dfont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    basefontsiz,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	playexpl = newoneshot(decodeda["expl0"], decodeda["expl1"], decodeda["expl2"])
	playdeaf = newoneshot(decodeda["deaf"])
	playspawn = newoneshot(decodeda["spawn"])

	introbgm = newsoundcnv(decodeda["intro"])
	normalbgm = newsoundcnv(decodeda["game1"])
	outrobgm = newsoundcnv(decodeda["outro"])
	g.bgm = introbgm

	esl := newslicer(expldat, 32)
	for j := range [3]int{} {
		for i := range [4]int{} {
			explspts = append(explspts, esl(i, j))
		}
	}

}

func loadlevel(g *game, lv int) {
	g.lvlw = float64(g.ldtk.Levels[lv].Width)
	g.lvlh = float64(g.ldtk.Levels[lv].Height)

	g.walls = g.ldtk.Levels[lv].LayerByIdentifier("AutoWalls")
	g.floor = g.ldtk.Levels[lv].LayerByIdentifier("Flooring")
	g.l2 = g.ldtk.Levels[lv].LayerByIdentifier("EntityTiles")

	ent := g.ldtk.Levels[lv].LayerByIdentifier("Entities")
	pl := ent.EntityByIdentifier("Player")
	if pl == nil {
		panic(fmt.Sprint("no player in level ", lv))
	}

	g.stamina = pl.PropertyByIdentifier("Stamina").AsFloat64()
	g.plx = float64(pl.Position[0] / tilesize)
	g.ply = float64(pl.Position[1] / tilesize)
	g.origsta = g.stamina
	g.flams = parseflams(ent.Entities)
}

func pregameinit(g *game) {
	proj, err := ldtkgo.Read(ldtk)
	if err != nil {
		panic(err)
	}
	g.ldtk = proj
	audioinit()
}

func main() {
	ebiten.SetWindowResizable(false)
	ebiten.SetWindowSize(800, 640)
	ebiten.SetWindowTitle("Lightning Ball Rampage")
	gm := &game{}
	gm.plx = 1
	gm.ply = 1
	for i := 1; i <= 255; i++ {
		collider[i] = struct{}{}
	}
	gm.lvl = 0
	gm.view = ebiten.NewImage(gm.Layout(800, 640))
	pregameinit(gm)
	if err := ebiten.RunGame(gm); err != nil {
		log.Fatal(err)
	}
}
