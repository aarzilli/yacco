package clipboard

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
	"os"
	"time"
)

const debugClipboardRequests = false

var X *xgbutil.XUtil
var win *xwindow.Window
var clipboardText string
var selnotify chan bool

var clipboardAtom, primaryAtom, textAtom, targetsAtom, atomAtom xproto.Atom
var targetAtoms []xproto.Atom

func Start() {
	var err error
	X, err = xgbutil.NewConn()
	if err != nil {
		panic(err)
	}

	selnotify = make(chan bool, 1)

	win, err = xwindow.Generate(X)
	if err != nil {
		panic(err)
	}

	err = win.CreateChecked(X.Screen().Root, 100, 100, 1, 1, 0)
	if err != nil {
		panic(err)
	}

	clipboardAtom = internAtom(X.Conn(), "CLIPBOARD")
	primaryAtom = internAtom(X.Conn(), "PRIMARY")
	textAtom = internAtom(X.Conn(), "UTF8_STRING")
	targetsAtom = internAtom(X.Conn(), "TARGETS")
	atomAtom = internAtom(X.Conn(), "ATOM")

	targetAtoms = []xproto.Atom{targetsAtom, textAtom}

	go eventLoop()
}

func Set(text string) {
	clipboardText = text
	ssoc := xproto.SetSelectionOwnerChecked(X.Conn(), win.Id, clipboardAtom, xproto.TimeCurrentTime)
	if err := ssoc.Check(); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting clipboard: %v", err)
	}
	ssoc = xproto.SetSelectionOwnerChecked(X.Conn(), win.Id, primaryAtom, xproto.TimeCurrentTime)
	if err := ssoc.Check(); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting primary selection: %v", err)
	}
}

func Get() string {
	return getSelection(clipboardAtom)
}

func GetPrimary() string {
	return getSelection(primaryAtom)
}

func getSelection(selAtom xproto.Atom) string {
	csc := xproto.ConvertSelectionChecked(X.Conn(), win.Id, selAtom, textAtom, selAtom, xproto.TimeCurrentTime)
	err := csc.Check()
	if err != nil {
		fmt.Println(err)
		return ""
	}

	select {
	case r := <-selnotify:
		if !r {
			return ""
		}
		gpc := xproto.GetProperty(X.Conn(), true, win.Id, selAtom, textAtom, 0, 5*1024*1024)
		gpr, err := gpc.Reply()
		if err != nil {
			fmt.Println(err)
			return ""
		}
		/*typename, _ := xprop.AtomName(w.xu, gpr.Type)
		println("Type", typename)*/
		if gpr.BytesAfter != 0 {
			fmt.Println("Clipboard too large")
			return ""
		}
		return string(gpr.Value[:gpr.ValueLen])
	case <-time.After(1 * time.Second):
		fmt.Println("Clipboard retrieval failed, timeout")
		return ""
	}
}

func eventLoop() {
	conn := X.Conn()
	for {
		e, err := conn.WaitForEvent()
		if err != nil {
			continue
		}

		switch e := e.(type) {
		case xproto.SelectionRequestEvent:
			if debugClipboardRequests {
				tgtname, _ := xprop.AtomName(X, e.Target)
				fmt.Println("SelectionRequest", e, textAtom, tgtname, "isPrimary:", e.Selection == primaryAtom, "isClipboard:", e.Selection == clipboardAtom)
			}
			t := clipboardText

			switch e.Target {
			case textAtom:
				if debugClipboardRequests {
					fmt.Println("Sending as text")
				}
				cpc := xproto.ChangePropertyChecked(X.Conn(), xproto.PropModeReplace, e.Requestor, e.Property, textAtom, 8, uint32(len(t)), []byte(t))
				err := cpc.Check()
				if err == nil {
					sendSelectionNotify(e)
				} else {
					fmt.Println(err)
				}

			case targetsAtom:
				if debugClipboardRequests {
					fmt.Println("Sending targets")
				}
				propName, _ := xprop.AtomName(X, e.Property)
				err := xprop.ChangeProp32(X, e.Requestor, propName, "ATOM", xprop.AtomToUint(targetAtoms)...)
				if err == nil {
					sendSelectionNotify(e)
				} else {
					fmt.Println(err)
				}

			default:
				if debugClipboardRequests {
					fmt.Println("Skipping")
				}
				//println("Skipping")
				e.Property = 0
				sendSelectionNotify(e)
			}

		case xproto.SelectionNotifyEvent:
			selnotify <- (e.Property == clipboardAtom) || (e.Property == primaryAtom)
		}
	}
}

func sendSelectionNotify(e xproto.SelectionRequestEvent) {
	sn := xproto.SelectionNotifyEvent{
		Time:      xproto.TimeCurrentTime,
		Requestor: e.Requestor,
		Selection: e.Selection,
		Target:    e.Target,
		Property:  e.Property}
	sec := xproto.SendEventChecked(X.Conn(), false, e.Requestor, 0, string(sn.Bytes()))
	err := sec.Check()
	if err != nil {
		fmt.Println(err)
	}
}

func internAtom(conn *xgb.Conn, n string) xproto.Atom {
	iac := xproto.InternAtom(conn, true, uint16(len(n)), n)
	iar, err := iac.Reply()
	if err != nil {
		panic(err)
	}
	return iar.Atom
}
