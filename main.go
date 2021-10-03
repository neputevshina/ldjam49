package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/solarlune/ldtkgo"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	// "github.com/hajimehoshi/ebiten/v2/inpututil"
)

const tilesize = 16
const plsize = tilesize
const introticks = 90
const explticks = 2
const basefontsiz = 16
const basefontlin = 8

const (
	dstill = iota
	dup
	dleft
	dright
	ddown
)

const (
	ftg = iota
	ftoas
	ftv
)

const (
	sinit = iota
	sintro
	stitle
	splay
	sclear
)

type flammable struct {
	typ  uint
	dur  float64
	x    float64
	y    float64
	dead bool
}

var (
	plsprites   = make([]*ebiten.Image, 0)
	spritesheet = make(map[int]*ebiten.Image)
	collider    = make(map[int]struct{})
	dbgstr      string
	intropic    *ebiten.Image
	logopic     *ebiten.Image
	lqwhite     *ebiten.Image
	dfont       font.Face
)

type game struct {
	ldtk  *ldtkgo.Project
	l2    *ldtkgo.Layer
	walls *ldtkgo.Layer
	floor *ldtkgo.Layer

	flams []flammable

	hiscore   int
	score     int
	plx       float64
	ply       float64
	stamina   float64
	origsta   float64
	shkticker uint
	dead      bool
	lvl       int

	state int
	tick  uint
	view  *ebiten.Image
}

func swstate(g *game, state int) {
	g.state = state
	g.tick = 0
}

func parseflams(ent []*ldtkgo.Entity) []flammable {
	fls := make([]flammable, 0, 20)
	for _, e := range ent {
		fl := flammable{
			x:    float64(e.Position[0]) / tilesize,
			y:    float64(e.Position[1]) / tilesize,
			dead: false,
		}
		switch e.Identifier {
		case "Tv":
			fl.dur = 1
			fl.typ = ftv
			fls = append(fls, fl)
		case "Toaster":
			fl.dur = 0.5
			fl.typ = ftoas
			fls = append(fls, fl)
		case "Target":
			fl.typ = ftg
			fls = append(fls, fl)
		}
	}
	return fls
}

func suck(g *game) {
	var trad = 0.7
	// dbgstr = ""
	for i, e := range g.flams {
		if e.dead {
			continue
		}
		pup := 0.5
		if e.typ == ftv {
			trad += 0.5
			pup = 1
		}
		x := e.x + pup
		y := e.y + pup
		plx, ply := x-g.plx, y-g.ply
		dis := math.Sqrt(plx*plx + ply*ply)
		if dis <= trad {
			// dbgstr = fmt.Sprint(g.score, e.typ, e.dur, e.dead)
			g.stamina -= 0.05
			g.flams[i].dur -= 0.05
			g.score += 5
			if e.dur <= 0 {
				g.flams[i].dead = true
				g.shkticker = 60
			}
			if e.typ == ftg {
				swstate(g, sclear)
			}
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
	if tl != nil {
		if _, k := collider[tl.ID]; k {
			g.stamina -= 0.1
			g.ply = pply
			g.plx = pplx
		}
	}
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

func drawmenu(g *game, screen *ebiten.Image) {
	screen.DrawImage(logopic, nil)
	lable := []string{
		`made by neputevshina, exiphase`,
		`and DISN for ludum dare 49`,
		`in three days`,
	}
	blink := []string{`use wasd`}
	scx := screen.Bounds().Dx() / 2
	sh := screen.Bounds().Dy()
	printlable(screen, lable, scx, 6*sh/7, color.White)
	if (g.tick/30)%2 == 0 {
		orang := color.RGBA{0xff, 0xc9, 0x00, 0xff}
		printlable(screen, blink, scx, 9*sh/14, orang)
	}
}

func updmenu(g *game) {
	if ebiten.IsKeyPressed(ebiten.KeyW) ||
		ebiten.IsKeyPressed(ebiten.KeyA) ||
		ebiten.IsKeyPressed(ebiten.KeyS) ||
		ebiten.IsKeyPressed(ebiten.KeyD) {
		g.state = splay
		g.lvl = 0
		loadlevel(g, g.lvl)
	}
}

func (g *game) Update() error {
	g.tick++
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
	case sclear:
	case stitle:
		updmenu(g)
	case splay:
		updplay(g)
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	const shkamnt = 30
	const shkpik = 60
	var fade float64
	switch g.state {
	case sintro:
		drawintro(g, screen)
	case stitle:
		drawmenu(g, screen)
	case sclear:
		ft := float64(g.tick)
		fade = 2 * (ft / explticks)
		if fade > 1 {
			fade = 1
		}
		op := ebiten.DrawImageOptions{}
		op.ColorM.Translate(0, 0, 0, fade-1)
		if g.tick > 30 {
			screen.Fill(color.White)

			return
		}
		// render explosion after level is rendered
		defer screen.DrawImage(lqwhite, &op)
		fallthrough
	case splay:
		g.view.Clear()
		drawsprites(g, g.view)
		drawpl(g, g.view)
		op := &ebiten.DrawImageOptions{}
		r := func() float64 {
			return rand.Float64() * shkamnt * float64(g.shkticker) / shkpik
		}
		op.GeoM.Translate(r(), r())
		screen.DrawImage(g.view, op)
		drawstaminabar(screen, g.stamina, g.origsta)
		ebitenutil.DebugPrint(screen, dbgstr)
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 200, 160
}

func fieldloader(file []byte) [][]int {
	fl := make([][]int, 0, 100)
	buf := make([]int, 0, 100)
	for _, v := range file {
		if v == '\n' {
			fl = append(fl, buf)
			buf = make([]int, 0, 100)
		} else if v == '1' {
			buf = append(buf, 1)
		} else if v == '0' {
			buf = append(buf, 0)
		}
	}
	fl = append(fl, buf)
	return fl
}

func newslicer(data []byte, size int) func(x, y int) *ebiten.Image {
	atlimg, _, _ := image.Decode(bytes.NewReader(data))
	atlas := ebiten.NewImageFromImage(atlimg)
	return func(x, y int) *ebiten.Image {
		x, y = x*size, y*size
		return ebiten.NewImageFromImage(atlas.SubImage(image.Rect(x, y, x+size, y+size)))
	}
}

func wholeimg(data []byte) *ebiten.Image {
	iimg, _, _ := image.Decode(bytes.NewReader(data))
	return ebiten.NewImageFromImage(iimg)
}

func gameinit(g *game) {
	spr2 := newslicer(atlas2, tilesize)
	for j := range [16]int{} {
		for i := range [16]int{} {
			spritesheet[i+16*j] = spr2(i, j)
		}
	}

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
}

func loadlevel(g *game, lv int) {
	g.walls = g.ldtk.Levels[lv].LayerByIdentifier("AutoWalls")
	g.floor = g.ldtk.Levels[lv].LayerByIdentifier("Flooring")
	g.l2 = g.ldtk.Levels[lv].LayerByIdentifier("EntityTiles")

	ent := g.ldtk.Levels[lv].LayerByIdentifier("Entities")
	pl := ent.EntityByIdentifier("Player")

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
}

func main() {
	ebiten.SetWindowResizable(false)
	ebiten.SetWindowSize(800, 640)
	ebiten.SetWindowTitle("Hello, World!")
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
