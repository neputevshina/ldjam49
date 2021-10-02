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
	"github.com/solarlune/ldtkgo"
	// "github.com/hajimehoshi/ebiten/v2/inpututil"
)

const tilesize = 16
const plsize = tilesize

//go:embed tempmap
var fieldstr []byte

//go:embed temptiles.png
var atlasdata []byte

//go:embed atlas2.png
var atlas2 []byte

//go:embed player.png
var kozin []byte

//go:embed 2021.ldtk
var ldtk []byte

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

type flammable struct {
	typ  uint
	dur  float64
	x    float64
	y    float64
	dead bool
}

var plsprites = make([]*ebiten.Image, 0)
var spritesheet = make(map[int]*ebiten.Image)
var collider = make(map[int]struct{})
var dbgstr string
var uilayer *ebiten.Image

type game struct {
	ldtk      *ldtkgo.Project
	l2        *ldtkgo.Layer
	walls     *ldtkgo.Layer
	floor     *ldtkgo.Layer
	flams     []flammable
	plx       float64
	ply       float64
	stamina   float64
	origsta   float64
	init      bool
	tick      uint
	dead      bool
	lvlclear  bool
	shkticker uint
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
	dbgstr = ""
	for _, e := range g.flams {
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
			g.stamina -= 0.05
			e.dur -= 0.05
			if e.dur < 0 {
				e.dead = true
			}
			if e.typ == ftg {
				g.lvlclear = true
			}
		}
	}
}

func (g *game) Update() error {
	if !g.init {
		gameinit(g)
		loadlevel(g, 0)
		g.init = true
	}
	const speed = 0.05
	const jitter = 0.06
	pplx, pply := g.plx, g.ply
	g.tick++

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
	return nil
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

func (g *game) Draw(screen *ebiten.Image) {
	drawsprites(g, screen)
	drawpl(g, screen)
	drawstaminabar(screen, g.stamina, g.origsta)
	ebitenutil.DebugPrint(screen, dbgstr)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 160, 120
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
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Hello, World!")
	gm := &game{}
	gm.plx = 1
	gm.ply = 1
	for i := 1; i <= 255; i++ {
		collider[i] = struct{}{}
	}
	pregameinit(gm)
	if err := ebiten.RunGame(gm); err != nil {
		log.Fatal(err)
	}
}
