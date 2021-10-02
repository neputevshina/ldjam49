package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
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
const plsize = tilesize * 2

//go:embed tempmap
var fieldstr []byte

//go:embed temptiles.png
var atlasdata []byte

//go:embed atlas2.png
var atlas2 []byte

//go:embed player.png
var kozin []byte

//go:embed 2021.ldtk
var playfield []byte

const (
	dstill = iota
	dup
	dleft
	dright
	ddown
)

var plsprites = make([]*ebiten.Image, 0)
var spritesheet = make(map[int]*ebiten.Image)
var collider = make(map[int]struct{})
var dbgstr string
var uilayer *ebiten.Image

type game struct {
	field   *ldtkgo.Layer
	ldtk    *ldtkgo.Project
	plx     float64
	ply     float64
	stamina float64
	init    bool
	tick    uint
}

func (g *game) Update() error {
	g.tick++
	const speed = 0.05
	const jitter = 0.03

	pplx, pply := g.plx, g.ply

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

	tl := g.field.AutoTileAt(plux, pluy)
	if tl != nil {
		if _, k := collider[tl.ID]; k {
			g.ply = pply
			g.plx = pplx
		}
	}
	dbgstr = fmt.Sprintf("x: %.2f y: %.2f\n", g.plx, g.ply)
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
	for _, t := range g.field.AllTiles() {
		op := &ebiten.DrawImageOptions{}
		// fuck ldtk
		i := t.Position[0] / tilesize
		j := t.Position[1] / tilesize
		op.GeoM.Translate(
			(float64(i)-g.plx)*tilesize+(W)/2,
			(float64(j)-g.ply)*tilesize+(H-2*plsize)/2,
		)
		screen.DrawImage(spritesheet[t.ID], op)
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	if !g.init {
		gameinit(g)
		g.init = true
	}
	drawsprites(g, screen)
	drawpl(g, screen)
	ebitenutil.DebugPrint(screen, dbgstr)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 240
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

func pregameinit(g *game) {
	proj, err := ldtkgo.Read(playfield)
	if err != nil {
		panic(err)
	}
	g.ldtk = proj
	g.field = g.ldtk.Levels[0].Layers[0]
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
