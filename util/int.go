package util

import (
	"github.com/skelterjohn/go.wde"
	"image"
	"math"
	"strings"
	"time"
)

type AltingEntry struct {
	Seq   string
	Glyph string
}

type WheelEvent struct {
	Where image.Point
	Count int
}

type MouseDownEvent struct {
	Where     image.Point
	Which     wde.Button
	Modifiers string
	Count     int
}

type eventType int

const (
	ET_OTHER = iota
	ET_MOUSE_MOVE
	ET_MOUSE_DRAG
	ET_RESIZE
	ET_WHEEL
)

type eventWrapper struct {
	et eventType
	ei interface{}
}

type eventMachine struct {
	dblclickp   image.Point
	dblclickc   int
	dblclickbtn wde.Button
	dblclickt   time.Time

	alting    bool
	altingSeq string

	downButtons wde.Button

	events []eventWrapper
}

func (em *eventMachine) fixButton(which *wde.Button, modifiers string, down bool, up bool) {
	orig := *which
	switch *which {
	case wde.LeftButton:
		switch modifiers {
		case "alt+":
			*which = wde.RightButton
		case "control+":
			*which = wde.MiddleButton
		case "control+shift+":
			*which = wde.MiddleButton | wde.LeftButton
		case "super+":
			*which = wde.MiddleButton | wde.LeftButton
		}
	case wde.MiddleButton:
		if modifiers == "shift+" {
			*which = wde.MiddleButton | wde.LeftButton
		}
	case wde.RightButton:
		if modifiers == "control+" {
			*which = wde.MiddleButton | wde.LeftButton
		}
	}

	if down {
		em.downButtons |= *which
	}
	if up {
		if (em.downButtons & *which) == 0 {
			*which = orig
		}
		em.downButtons &= ^(*which)
	}
}

func (em *eventMachine) processEvent(ei interface{}, altingList []AltingEntry, keyConversion map[string]string) {
	switch e := ei.(type) {
	case wde.KeyTypedEvent:
		if em.alting && (e.Glyph != "") {
			em.altingSeq += "+" + e.Glyph
			//println("altingSeq:", altingSeq)
			keepAlting := false
			for _, altingEntry := range altingList {
				if altingEntry.Seq == em.altingSeq {
					//println("Emitting:", altingEntry.Glyph)
					em.appendEventOther(ET_OTHER, wde.KeyTypedEvent{Glyph: altingEntry.Glyph, Chord: e.Chord})
					em.alting = false
					break
				}
				if strings.HasPrefix(altingEntry.Seq, em.altingSeq) {
					keepAlting = true
				}
			}
			if !keepAlting {
				//println("Alting end")
				em.alting = false
			}
		} else if conv, ok := keyConversion[e.Chord]; ok {
			e.Chord = conv
			e.Key = conv
			em.appendEventOther(ET_OTHER, e)
		} else {
			if e.Chord == "" {
				e.Chord = e.Key
			}
			em.appendEventOther(ET_OTHER, e)
		}
		//println("Typed:", e.Glyph, e.Chord, "alting:", alting)

	case wde.KeyUpEvent:
		if e.Key == "Multi_key" || e.Key == wde.KeyRightAlt {
			em.alting = true
			em.altingSeq = ""
		}
		em.appendEventOther(ET_OTHER, ei)

	case wde.MouseDraggedEvent:
		em.fixButton(&e.Which, e.Modifiers, false, false)
		em.appendMouseDraggedEvent(e)

	case wde.MouseMovedEvent:
		em.appendMouseMovedEvent(e)

	case wde.MouseDownEvent:
		if e.Which == 0 {
			break
		}

		em.fixButton(&e.Which, e.Modifiers, true, false)
		switch e.Which {
		case wde.WheelUpButton:
			em.appendWheelEvent(e.Where, -1)
		case wde.WheelDownButton:
			em.appendWheelEvent(e.Where, +1)
		default:
			now := time.Now()
			dist := math.Sqrt(float64(em.dblclickp.X-e.Where.X)*float64(em.dblclickp.X-e.Where.X) + float64(em.dblclickp.Y-e.Where.Y)*float64(em.dblclickp.Y-e.Where.Y))

			if (e.Which == em.dblclickbtn) && (dist < 5) && (now.Sub(em.dblclickt) < time.Duration(200*time.Millisecond)) {
				em.dblclickt = now
				em.dblclickc++
			} else {
				em.dblclickbtn = e.Which
				em.dblclickp = e.Where
				em.dblclickt = now
				em.dblclickc = 1
			}

			if em.dblclickc > 3 {
				em.dblclickc = 1
			}

			em.appendEventOther(ET_OTHER, e)
			em.appendEventOther(ET_OTHER, MouseDownEvent{
				Where:     e.Where,
				Which:     e.Which,
				Count:     em.dblclickc,
				Modifiers: e.Modifiers,
			})
		}

	case wde.MouseUpEvent:
		if e.Which == 0 {
			break
		}
		em.fixButton(&e.Which, e.Modifiers, false, true)
		em.appendEventOther(ET_OTHER, e)

	case wde.ResizeEvent:
		em.appendResizeEvent(e)

	default:
		em.appendEventOther(ET_OTHER, ei)
	}
}

func FilterEvents(in <-chan interface{}, altingList []AltingEntry, keyConversion map[string]string) chan interface{} {
	var em eventMachine

	em.dblclickp = image.Point{0, 0}
	em.dblclickc = 0
	em.dblclickbtn = wde.LeftButton
	em.dblclickt = time.Now()

	em.alting = false
	em.altingSeq = ""

	em.downButtons = wde.Button(0)

	em.events = make([]eventWrapper, 0, 10)
	rout := make(chan interface{})

	lastResize := time.Unix(0, 0)
	lastMouse := time.Unix(0, 0)
	lastWheel := time.Unix(0, 0)

	var ticker *time.Ticker = nil

	go func() {
		for {
			if ticker != nil {
				select {
				case ei := <-in:
					em.processEvent(ei, altingList, keyConversion)
				case <-ticker.C:
					ticker.Stop()
					ticker = nil
				}
			} else if len(em.events) > 0 {
				d := int64(-1)

				switch em.events[0].et {
				case ET_MOUSE_DRAG, ET_MOUSE_MOVE:
					d = int64(30*time.Millisecond - time.Now().Sub(lastMouse))
				case ET_RESIZE:
					d = int64(30*time.Millisecond - time.Now().Sub(lastResize))
				case ET_WHEEL:
					d = int64(60*time.Millisecond - time.Now().Sub(lastWheel))
				}

				if d > 0 {
					ticker = time.NewTicker(time.Duration(d))
				} else {
					select {
					case rout <- em.events[0].ei:
						switch em.events[0].et {
						case ET_MOUSE_DRAG, ET_MOUSE_MOVE:
							lastMouse = time.Now()
						case ET_RESIZE:
							lastResize = time.Now()
						case ET_WHEEL:
							lastWheel = time.Now()
						}
						copy(em.events, em.events[1:])
						em.events = em.events[:len(em.events)-1]
					case ei := <-in:
						em.processEvent(ei, altingList, keyConversion)
					}
				}
			} else {
				ei := <-in
				em.processEvent(ei, altingList, keyConversion)
			}
		}
	}()

	return rout
}

func (em *eventMachine) appendEventOther(et eventType, ei interface{}) {
	em.events = append(em.events, eventWrapper{et, ei})
}

func (em *eventMachine) appendMouseDraggedEvent(e wde.MouseDraggedEvent) {
	for i := range em.events {
		if em.events[i].et == ET_MOUSE_DRAG {
			em.removeAndReaddEvent(i, e, ET_MOUSE_DRAG)
			return
		}
	}
	em.appendEventOther(ET_MOUSE_DRAG, e)
}

func (em *eventMachine) appendMouseMovedEvent(e wde.MouseMovedEvent) {
	for i := range em.events {
		if em.events[i].et == ET_MOUSE_MOVE {
			em.removeAndReaddEvent(i, e, ET_MOUSE_MOVE)
			return
		}
	}
	em.appendEventOther(ET_MOUSE_MOVE, e)
}

func (em *eventMachine) appendWheelEvent(w image.Point, d int) {
	for i := range em.events {
		if em.events[i].et == ET_WHEEL {
			e := em.events[i].ei.(WheelEvent)
			e.Count += d
			em.removeAndReaddEvent(i, e, ET_WHEEL)
			return
		}
	}
	em.appendEventOther(ET_WHEEL, WheelEvent{Count: d, Where: w})
}

func (em *eventMachine) appendResizeEvent(e wde.ResizeEvent) {
	for i := range em.events {
		if em.events[i].et == ET_RESIZE {
			em.removeAndReaddEvent(i, e, ET_RESIZE)
			return
		}
	}
	em.appendEventOther(ET_RESIZE, e)
}

func (em *eventMachine) removeAndReaddEvent(i int, e interface{}, et eventType) {
	copy(em.events[i:], em.events[i+1:])
	em.events[len(em.events)-1] = eventWrapper{et, e}
}
