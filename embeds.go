package main

import (
	"embed"
	_ "embed"
)

//go:embed data/atlas2.png
var atlas2 []byte

//go:embed data/player.png
var kozin []byte

//go:embed data/2021.ldtk
var ldtk []byte

//go:embed data/intro.png
var introdat []byte

//go:embed data/logo1.png
var logodat []byte

//go:embed data/PKMN-Mystery-Dungeon.ttf
var fontdat []byte

//go:embed data/expl.png
var expldat []byte

//go:embed audio/*
var audiofs embed.FS

//go:embed data/babah16.png
var dead11dat []byte

//go:embed data/babah32.png
var dead22dat []byte

//go:embed data/babah1632.png
var dead21dat []byte
