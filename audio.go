package main

import (
	"bytes"
	"io"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

var (
	decodeda = make(map[string]*mp3.Stream)
	audioctx *audio.Context
)

func newsoundcnv(rd io.Reader) func(bool) *audio.Player {
	pl, _ := audio.NewPlayer(audioctx, rd)
	return func(stfu bool) *audio.Player {
		if !pl.IsPlaying() {
			pl.Rewind()
			pl.Play()
		}
		if stfu {
			pl.Pause()
		}
		return pl
	}
}

func newoneshot(rds ...io.Reader) func() {
	pls := make([]*audio.Player, len(rds))
	for i := range pls {
		pls[i], _ = audio.NewPlayer(audioctx, rds[i])
	}
	return func() {
		i := rand.Intn(len(pls))
		if !pls[i].IsPlaying() {
			pls[i].Rewind()
		}
		pls[i].Play()
	}
}

func audioinit() {
	audioctx = audio.NewContext(44100) // get from files
	dec := func(name string) *mp3.Stream {
		f, err := audiofs.ReadFile(name)
		if err != nil {
			panic(err)
		}
		fs := bytes.NewReader(f)
		s, _ := mp3.DecodeWithSampleRate(44100, fs)
		return s
	}

	decodeda = map[string]*mp3.Stream{
		"intro": dec("audio/menu_music.mp3"),
		"game1": dec("audio/level1_music.mp3"),
		"expl0": dec("audio/explode0.mp3"),
		"expl1": dec("audio/explode1.mp3"),
		"expl2": dec("audio/explode2.mp3"),
		"deaf":  dec("audio/minusears.mp3"),
		"outro": dec("audio/641.mp3"),
		"spawn": dec("audio/spawn_fadeout.mp3"),
		"buzz":  dec("audio/poweron.mp3"),
	}

}
