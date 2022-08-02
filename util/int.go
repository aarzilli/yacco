package util

import (
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"image"
)

type AltingEntry struct {
	Seq   string
	Glyph rune
}

type WheelEvent struct {
	Where image.Point
	Count int
}

type MouseDownEvent struct {
	Where     image.Point
	Which     mouse.Button
	Modifiers key.Modifiers
	Count     int
}
