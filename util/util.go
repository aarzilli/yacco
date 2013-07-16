package util

import (
	"fmt"
	"strings"
	"time"
	"runtime"
	"image"
	"sort"
	"math"
	"github.com/skelterjohn/go.wde"
)

type Sel struct {
	S, E int
}

type AltingEntry struct {
	Seq string
	Glyph string
}

type WheelEvent struct {
	Where image.Point
	Count int
}

type MouseDownEvent struct {
	Where image.Point
	Which wde.Button
	Count int
}

func FilterEvents(in <-chan interface{}, altingList []AltingEntry, keyConversion map[string]string) (out chan interface{}) {
	dblclickp := image.Point{ 0, 0 }
	dblclickc := 0
	dblclickbtn := wde.LeftButton
	dblclickt := time.Now()

	out = make(chan interface{})
	go func() {
		alting := false

		resizeChan := make(chan bool, 1)
		var resizeEvent wde.ResizeEvent
		resizeFlag := false

		mouseChan := make(chan bool, 1)
		var mouseEvent interface{}
		mouseFlag := false

		altingSeq := ""

		scheduleMouseEvent := func(ei interface{}) {
			if !mouseFlag {
				mouseFlag = true
				mouseEvent = ei
				go func() {
					time.Sleep(20 * time.Millisecond)
					mouseChan <- true
				}()
			}
		}

		wheelTotal := 0
		var wheelEvent wde.MouseDownEvent
		wheelChan := make(chan bool, 1)
		scheduleWheelEvent := func(e wde.MouseDownEvent, n int) {
			if wheelTotal == 0 {
				wheelEvent = e
				go func() {
					time.Sleep(20 * time.Millisecond)
					wheelChan <- true
				}()
			}
			wheelTotal += n
		}

		fixButton := func(which *wde.Button, modifiers string) {
			switch *which {
			case wde.LeftButton:
				switch modifiers {
				case "control+":
					*which = wde.MiddleButton
				case "control+shift+":
					*which = wde.MiddleButton | wde.LeftButton
				}
			case wde.MiddleButton:
				if modifiers == "shift+" {
					*which = wde.MiddleButton | wde.LeftButton
				}
			}
		}

		for {
			runtime.Gosched()
			select {
			case ei := <- in:
				switch e := ei.(type) {
				case wde.KeyTypedEvent:
					if alting && (e.Glyph != "") {
						altingSeq += "+" + e.Glyph
						//println("altingSeq:", altingSeq)
						keepAlting := false
						for _, altingEntry := range altingList {
							if altingEntry.Seq == altingSeq {
								//println("Emitting:", altingEntry.Glyph)
								out <- wde.KeyTypedEvent{
									Glyph: altingEntry.Glyph,
									Chord: e.Chord,
								}
								alting = false
								break
							}
							if strings.HasPrefix(altingEntry.Seq, altingSeq) {
								keepAlting = true
							}
						}
						if !keepAlting {
							//println("Alting end")
							alting = false
						}
					} else if conv, ok := keyConversion[e.Chord]; ok {
						e.Chord = conv
						e.Key = conv
						out <- e
					} else {
						if e.Chord == "" {
							e.Chord = e.Key
						}
						out <- e
					}
					//println("Typed:", e.Glyph, e.Chord, "alting:", alting)

				case wde.KeyDownEvent:
					out <- ei

				case wde.KeyUpEvent:
					if e.Key == "Multi_key" {
						alting = true
						altingSeq = ""
					}
					out <- ei

				case wde.MouseExitedEvent:
					out <- ei

				case wde.MouseEnteredEvent:
					out <- ei

				case wde.MouseDraggedEvent:
					fixButton(&e.Which, e.Modifiers)
					scheduleMouseEvent(e)

				case wde.MouseMovedEvent:
					scheduleMouseEvent(e)

				case wde.MouseDownEvent:
					if e.Which == 0 {
						break
					}

					fixButton(&e.Which, e.Modifiers)
					switch e.Which {
					case wde.WheelUpButton:
						scheduleWheelEvent(e, -1)
					case wde.WheelDownButton:
						scheduleWheelEvent(e, +1)
					default:
						now := time.Now()
						dist := math.Sqrt(float64(dblclickp.X - e.Where.X) * float64(dblclickp.X - e.Where.X) + float64(dblclickp.Y - e.Where.Y) * float64(dblclickp.Y - e.Where.Y))

						if (e.Which == dblclickbtn) && (dist < 5) && (now.Sub(dblclickt) < time.Duration(200 * time.Millisecond)) {
							dblclickt = now
							dblclickc++
						} else {
							dblclickbtn = e.Which
							dblclickp = e.Where
							dblclickt = now
							dblclickc = 1
						}

						if dblclickc > 3 {
							dblclickc = 1
						}

						out <- e
						out <- MouseDownEvent{
							Where: e.Where,
							Which: e.Which,
							Count: dblclickc,
						}
					}

				case wde.MouseUpEvent:
					if e.Which == 0 {
						break
					}
					fixButton(&e.Which, e.Modifiers)
					out <- e

				case wde.ResizeEvent:
					if !resizeFlag {
						resizeFlag = true
						resizeEvent = e
						go func() {
							time.Sleep(20 * time.Millisecond)
							resizeChan <- true
						}()
					}

				default:
					out <- ei
				}

			case <- resizeChan:
				resizeFlag = false
				out <- resizeEvent

			case <- mouseChan:
				mouseFlag = false
				out <- mouseEvent

			case <- wheelChan:
				out <- WheelEvent{
					Count: wheelTotal,
					Where: wheelEvent.Where,
				}
				wheelTotal = 0
			}
		}
	}()
	return out
}

func Must(err error, msg string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %v", msg, err))
	}
}

func Dedup(v []string) []string {
	sort.Strings(v)
	dst := 0
	var prev *string = nil
	for src := 0; src < len(v); src++ {
		if (prev == nil) || (v[src] != *prev) {
			v[dst] = v[src]
			dst++
		}
		prev = &v[dst-1]
	}
	return v[:dst]
}

