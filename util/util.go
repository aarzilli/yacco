package util

import (
	"fmt"
	"strings"
	"time"
	"runtime"
	"yacco/config"
	"github.com/skelterjohn/go.wde"
)

type Sel struct {
	S, E int
}

func FilterEvents(in <-chan interface{}) (out chan interface{}) {
	out = make(chan interface{})
	go func() {
		shift := false
		ctrl := false
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

		fixButton := func(which *wde.Button) {
			if *which == wde.LeftButton {
				if shift {
					*which = wde.RightButton
				} else if ctrl {
					*which = wde.MiddleButton
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
						for _, altingEntry := range config.AltingList {
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
					} else {
						if e.Chord == "" {
							e.Chord = e.Key
						}
						out <- e
					}
					//println("Typed:", e.Glyph, e.Chord, "alting:", alting)

				case wde.KeyDownEvent:
					switch e.Key {
					case "left_shift": fallthrough
					case "right_shift":
						shift = true
					case "left_control":
						ctrl = true
					}
					out <- ei

				case wde.KeyUpEvent:
					switch e.Key {
					case "left_shift":
						shift = false
					case "left_control":
						ctrl = false
					case "Multi_key":
						alting = true
						altingSeq = ""
					}
					out <- ei

				case wde.MouseExitedEvent:
					ctrl = false
					shift = false
					alting = false
					out <- ei

				case wde.MouseEnteredEvent:
					ctrl = false
					shift = false
					alting = false
					out <- ei

				case wde.MouseDraggedEvent:
					fixButton(&e.Which)
					scheduleMouseEvent(e)

				case wde.MouseMovedEvent:
					scheduleMouseEvent(e)

				case wde.MouseDownEvent:
					fixButton(&e.Which)
					out <- e

				case wde.MouseUpEvent:
					fixButton(&e.Which)
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
