package ibus

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"golang.org/x/mobile/event/key"
)

var Events = make(chan Event, 10)

type Event struct {
	OldText string
	Text    string
	Select  bool
	Payload any
}

const inputContextIface = "org.freedesktop.IBus.InputContext"

var initialized = false
var inputContextObject dbus.BusObject
var ibusConn *dbus.Conn

var payloadMu sync.Mutex
var payload any
var preeditText string

// To debug the communication of this thing with ibus:
//
// 1. open a terminal with env variables GTK_IM_MODULE,QT_IM_MODULE,XMODIFIERS unset
// 2. in that terminal run:
//        dbus-monitor --address `ibus address`  > dbus-monitor-trace.txt
//
// That command must be run in a terminal that isn't going to access the bus
// otherwise it will keep spamming it with SetCursorLocation messages in a
// feedback loop.

// Start starts a connection to ibus
func Start() {
	if initialized {
		return
	}
	if runtime.GOOS != "linux" {
		return
	}
	xmodifiers := os.Getenv("XMODIFIERS")
	if !strings.Contains(xmodifiers, "@im=ibus") {
		// There doesn't seem to be any documentation on the format of the
		// XMODIFIERS environment variable, except that it can be @im=ibus or
		// @im=fcitx
		return
	}

	// There doesn't seem to be any documentation on how to properly determine
	// the location of the ibus message bus, because everyone who works on
	// freedesktop.org crap does not document anything ever for any reason.
	// This function copies the behavior of SDL.
	//
	// References:
	//  https://github.com/ProductExperts/vengi/blob/fc0c4d9e32e419253917d37bfe5bb97451f39787/contrib/libs/sdl2/src/core/linux/SDL_ibus.c#L64
	//  https://github.com/ibus/ibus/blob/b4f51b69f03b18fadf4bbd82abc7b7e92fae44f0/src/ibusengine.c#L277
	//  https://github.com/ibus/ibus/blob/b4f51b69f03b18fadf4bbd82abc7b7e92fae44f0/bus/ibusimpl.c#L214
	//  https://ibus.github.io/docs/ibus-1.5/ibus-ibustypes.html

	const ibusAddrEnvName = "IBUS_ADDRESS"

	addr := os.Getenv(ibusAddrEnvName)
	if addr != "" {
		startWithAddr(addr)
		return
	}

	var machineId string
	{
		buf, err := os.ReadFile("/etc/machine-id")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading machine-id: %v\n", err)
			return
		}
		machineId = strings.TrimSpace(string(buf))
	}

	var disp string
	{
		dispstr := os.Getenv("DISPLAY")
		hostName, screenStr, ok := strings.Cut(dispstr, ":")
		if !ok {
			fmt.Fprintf(os.Stderr, "error reading DISPLAY env: %q\n", dispstr)
			return
		}
		if hostName != "" {
			return
		}

		disp, _, ok = strings.Cut(screenStr, ".")

		_, err := strconv.Atoi(disp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading DISPLAY env: %q\n", dispstr)
			return
		}
	}

	var xdgconf string
	{
		xdgconf = os.Getenv("XDG_CONFIG_DIR")
		if xdgconf == "" {
			xdgconf = os.ExpandEnv("$HOME/.config/")
		}
	}

	path := filepath.Join(xdgconf, "ibus", "bus", fmt.Sprintf("%s-unix-%s", machineId, disp))
	buf, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading ibus bus at %q: %v", path, err)
		return
	}

	for _, line := range strings.Split(string(buf), "\n") {
		const pfx = ibusAddrEnvName + "="
		if strings.HasPrefix(line, pfx) {
			startWithAddr(line[len(pfx):])
			return
		}
	}
}

func startWithAddr(addr string) {
	conn, err := dbus.Connect(addr, dbus.WithSignalHandler(&messageHandler{}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not connect to ibus addr %q: %v", addr, err)
		return
	}
	obj := conn.Object("org.freedesktop.IBus", "/org/freedesktop/IBus")
	var inputContextPath string
	err = obj.Call("org.freedesktop.IBus.CreateInputContext", 0, "FuckFreedesktopDorOrg").Store(&inputContextPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ibus: could not create input context: %v\n", err)
		return
	}
	slash := strings.LastIndex(inputContextPath, "/")
	if slash < 0 {
		conn.Close()
		fmt.Fprintf(os.Stderr, "ibus: invalid return value for CreateInputContext: %q\n", inputContextPath)
		return
	}
	inputContextObject = conn.Object("org.freedesktop.IBus", dbus.ObjectPath(inputContextPath))

	const (
		IBUS_CAP_PREEDIT_TEXT = 1 << 0
		IBUS_CAP_FOCUS        = 1 << 3
	)
	err = inputContextObject.Call(inputContextIface+".SetCapabilities", 0, uint32(IBUS_CAP_PREEDIT_TEXT|IBUS_CAP_FOCUS)).Err
	if err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "ibus: SetCapabilities: %v", err)
		return
	}

	err = conn.AddMatchSignal(dbus.WithMatchObjectPath(dbus.ObjectPath(inputContextPath)))
	if err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "ibus: could not match signal: %v", err)
		return
	}

	ibusConn = conn
	initialized = true

	Reset()
}

type messageHandler struct {
}

func getTextArg(signal *dbus.Signal) (string, bool) {
	if len(signal.Body) > 0 {
		body, ok := signal.Body[0].(dbus.Variant)
		if ok {
			args, ok := body.Value().([]interface{})
			if ok && len(args) > 2 {
				text, ok := args[2].(string)
				return text, ok
			}
		}
	}
	return "", false
}

func (*messageHandler) DeliverSignal(iface, name string, signal *dbus.Signal) {
	if iface != inputContextIface {
		return
	}

	payloadMu.Lock()
	p := payload
	payloadMu.Unlock()

	switch name {
	case "UpdatePreeditText":
		if text, ok := getTextArg(signal); ok {
			if preeditText == text {
				return
			}
			select {
			case Events <- Event{preeditText, text, true, p}:
				preeditText = text
			default:
			}

		}
	case "HidePreeditText":
		select {
		case Events <- Event{preeditText, "", true, p}:
			preeditText = ""
		default:
		}
	case "CommitText":
		if text, ok := getTextArg(signal); ok {
			select {
			case Events <- Event{preeditText, text, false, p}:
				preeditText = text
			default:
			}
		}
	}
}

func SetCursorLocation(pos image.Rectangle) {
	inputContextObject.Call(inputContextIface+".SetCursorLocation", 0, int(pos.Min.X), int(pos.Min.Y), pos.Dx(), pos.Dy())
}

// Stop closes the connection to ibus
func Stop() {
	if !initialized {
		return
	}
	ibusConn.Close()
	initialized = false
}

// ProcessKey sends a key event to ibus, returns true if the event was handled
func ProcessKey(e key.Event, cursor func() image.Rectangle, newPayload any) bool {
	if !initialized {
		return false
	}

	payloadMu.Lock()
	payload = newPayload
	payloadMu.Unlock()

	r, c := uint32(e.Rune), e.Code
	if e.Rune == -1 {
		if r2, ok := deshiny[c]; ok {
			r = r2
		}
	}

	const (
		_IBUS_SHIFT_MASK   = 1 << 0
		_IBUS_CONTROL_MASK = 1 << 2
		_IBUS_MOD1_MASK    = 1 << 3
		_IBUS_MOD4_MASK    = 1 << 6
	)

	state := uint32(0)
	if e.Modifiers&key.ModShift != 0 {
		state |= _IBUS_SHIFT_MASK
	}
	if e.Modifiers&key.ModControl != 0 {
		state |= _IBUS_CONTROL_MASK
	}
	if e.Modifiers&key.ModAlt != 0 {
		state |= _IBUS_MOD1_MASK
	}
	if e.Modifiers&key.ModMeta != 0 {
		state |= _IBUS_MOD4_MASK
	}

	var out bool
	inputContextObject.Call(inputContextIface+".ProcessKeyEvent", 0, r, c, state).Store(&out)
	if !out {
		return false
	}
	SetCursorLocation(cursor())
	return true
}

// Focused copies the focused status to ibus
func Focused(focused bool) {
	if !initialized {
		return
	}
	if focused {
		payloadMu.Lock()
		payload = nil
		payloadMu.Unlock()
		inputContextObject.Call(inputContextIface+".FocusIn", 0)
	} else {
		payloadMu.Lock()
		payload = nil
		payloadMu.Unlock()
		inputContextObject.Call(inputContextIface+".FocusOut", 0)
	}
}

// Reset sends the reset command to ibus
func Reset() {
	if !initialized {
		return
	}
	payloadMu.Lock()
	payload = nil
	payloadMu.Unlock()
	inputContextObject.Call(inputContextIface+".Reset", 0)
}

func Enabled() bool {
	return initialized
}
