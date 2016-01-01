package util

import (
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/size"
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
	Which     mouse.Button
	Modifiers key.Modifiers
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
	dblclickbtn mouse.Button
	dblclickt   time.Time

	alting    bool
	altingSeq string

	events []eventWrapper
}

func (em *eventMachine) fixButton(which *mouse.Button, modifiers key.Modifiers, down bool, up bool) {
	alt := (modifiers & key.ModAlt) != 0
	ctrl := modifiers&key.ModControl != 0
	shift := modifiers&key.ModShift != 0
	meta := modifiers&key.ModMeta != 0
	_ = shift

	switch *which {
	case mouse.ButtonLeft:
		switch {
		case alt:
			*which = mouse.ButtonRight
		case ctrl:
			*which = mouse.ButtonMiddle
		case meta:
			*which = mouse.ButtonMiddle | mouse.ButtonLeft
		}
	case mouse.ButtonMiddle:
		switch {
		case ctrl:
			*which = mouse.ButtonMiddle | mouse.ButtonLeft
		}
	case mouse.ButtonRight:
		if ctrl {
			*which = mouse.ButtonMiddle | mouse.ButtonLeft
		}
	}
}

func (em *eventMachine) processEvent(ei interface{}, altingList []AltingEntry, keyConversion map[string]key.Event) {
	switch e := ei.(type) {
	case key.Event:
		switch e.Direction {
		case key.DirPress, key.DirNone:
			if em.alting && (e.Rune >= 0) {
				em.altingSeq += "+" + string(e.Rune)
				//println("altingSeq:", altingSeq)
				keepAlting := false
				for _, altingEntry := range altingList {
					if altingEntry.Seq == em.altingSeq {
						//println("Emitting:", altingEntry.Glyph)
						//TODO: inefficient, just have Glyph be a rune
						em.appendEventOther(ET_OTHER, key.Event{Rune: ([]rune(altingEntry.Glyph))[0]})
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
			} else if conv, ok := keyConversion[e.String()]; ok {
				e.Rune = conv.Rune
				e.Code = conv.Code
				e.Modifiers = conv.Modifiers
				em.appendEventOther(ET_OTHER, e)
			} else {
				em.appendEventOther(ET_OTHER, e)
			}
			//println("Typed:", e.Glyph, e.Chord, "alting:", alting)

		case key.DirRelease:
			if e.Code == key.CodeRightAlt {
				em.alting = true
				em.altingSeq = ""
			}
			em.appendEventOther(ET_OTHER, ei)
		}

	case mouse.Event:
		switch e.Direction {
		case mouse.DirNone:
			if e.Button == mouse.ButtonNone {
				em.appendMouseMovedEvent(e)
			} else {
				em.fixButton(&e.Button, e.Modifiers, false, false)
				em.appendMouseDraggedEvent(e)
			}

		case mouse.DirPress:
			if e.Button == mouse.ButtonNone {
				break
			}

			where := image.Point{int(e.X), int(e.Y)}
			em.fixButton(&e.Button, e.Modifiers, true, false)
			switch e.Button {
			case mouse.ButtonWheelUp:
				em.appendWheelEvent(where, -1)
			case mouse.ButtonWheelDown:
				em.appendWheelEvent(where, +1)
			default:
				now := time.Now()
				dist := math.Sqrt(float64(em.dblclickp.X-where.X)*float64(em.dblclickp.X-where.X) + float64(em.dblclickp.Y-where.Y)*float64(em.dblclickp.Y-where.Y))

				if (e.Button == em.dblclickbtn) && (dist < 5) && (now.Sub(em.dblclickt) < time.Duration(200*time.Millisecond)) {
					em.dblclickt = now
					em.dblclickc++
				} else {
					em.dblclickbtn = e.Button
					em.dblclickp = where
					em.dblclickt = now
					em.dblclickc = 1
				}

				if em.dblclickc > 3 {
					em.dblclickc = 1
				}

				em.appendEventOther(ET_OTHER, e)
				em.appendEventOther(ET_OTHER, MouseDownEvent{
					Where:     where,
					Which:     e.Button,
					Count:     em.dblclickc,
					Modifiers: e.Modifiers,
				})
			}

		case mouse.DirRelease:
			if e.Button == mouse.ButtonNone {
				break
			}
			em.fixButton(&e.Button, e.Modifiers, false, true)
			em.appendEventOther(ET_OTHER, e)
		}

	case size.Event:
		em.appendResizeEvent(e)

	default:
		em.appendEventOther(ET_OTHER, ei)
	}
}

func FilterEvents(in <-chan interface{}, altingList []AltingEntry, keyConversion map[string]key.Event) chan interface{} {
	var em eventMachine

	em.dblclickp = image.Point{0, 0}
	em.dblclickc = 0
	em.dblclickbtn = mouse.ButtonLeft
	em.dblclickt = time.Now()

	em.alting = false
	em.altingSeq = ""

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

func (em *eventMachine) appendMouseDraggedEvent(e mouse.Event) {
	for i := range em.events {
		if em.events[i].et == ET_MOUSE_DRAG {
			em.removeAndReaddEvent(i, e, ET_MOUSE_DRAG)
			return
		}
	}
	em.appendEventOther(ET_MOUSE_DRAG, e)
}

func (em *eventMachine) appendMouseMovedEvent(e mouse.Event) {
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

func (em *eventMachine) appendResizeEvent(e size.Event) {
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
