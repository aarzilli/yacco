package main

/*
#include <termios.h>
#include <fcntl.h>
*/
import "C"

import (
	"os"
	"errors"
)

const (
	O_NOCTTY int = C.O_NOCTTY
	O_NDELAY int = C.O_NDELAY
)

type Termios struct {
	ios C.struct_termios
}

type IFlag C.tcflag_t
const (
	IFLAG_NONE IFlag = 0
	IGNBRK IFlag = C.IGNBRK
	BRKINT IFlag = C.BRKINT
	IGNPAR IFlag = C.IGNPAR
	PARMRK IFlag = C.PARMRK
	INPCK IFlag = C.INPCK
	ISTRIP IFlag = C.ISTRIP
	INLCR IFlag = C.INLCR
	IGNCR IFlag = C.IGNCR
	ICRNL IFlag = C.ICRNL
	IUCLC IFlag = C.IUCLC
	IXON IFlag = C.IXON
	IXANY IFlag = C.IXANY
	IXOFF IFlag = C.IXOFF
	IMAXBEL IFlag = C.IMAXBEL
	IUTF8 IFlag = C.IUTF8
)

type OFlag C.tcflag_t
const (
	OFLAG_NONE OFlag = 0
	OPOST OFlag = C.OPOST
	OLCUC OFlag = C.OLCUC
	ONLCR OFlag = C.ONLCR
	OCRNL OFlag = C.OCRNL
	ONOCR OFlag = C.ONOCR
	ONLRET OFlag = C.ONLRET
	OFILL OFlag = C.OFILL
	OFDEL OFlag = C.OFDEL
	NLDLY OFlag = C.NLDLY
	CRDLY OFlag = C.CRDLY
	TABDLY OFlag = C.TABDLY
	BSDLY OFlag = C.BSDLY
	VTDLY OFlag = C.VTDLY
	FFDLY OFlag = C.FFDLY
)

type CFlag C.tcflag_t
const (
	CFLAG_NONE CFlag = 0
	CBAUD CFlag = C.CBAUD
	CBAUDEX CFlag = C.CBAUDEX
	CS5 CFlag = C.CS5
	CS6 CFlag = C.CS6
	CS7 CFlag = C.CS7
	CS8 CFlag = C.CS8
	CSTOPB CFlag = C.CSTOPB
	CREAD CFlag = C.CREAD
	PARENB CFlag = C.PARENB
	PARODD CFlag = C.PARODD
	HUPCL CFlag = C.HUPCL
	CLOCAL CFlag = C.CLOCAL
	CIBAUD CFlag = C.CIBAUD
	CMSPAR CFlag = C.CMSPAR
)

type LFlag C.tcflag_t
const (
	LFLAG_NONE LFlag = 0
	ISIG LFlag = C.ISIG
	ICANON LFlag = C.ICANON
	XCASE LFlag = C.XCASE
	ECHO LFlag = C.ECHO
	ECHOE LFlag = C.ECHOE
	ECHOK LFlag = C.ECHOK
	ECHONL LFlag = C.ECHONL
	ECHOCTL LFlag = C.ECHOCTL
	ECHOPRT LFlag = C.ECHOPRT
	ECHOKE LFlag = C.ECHOKE
	FLUSHO LFlag = C.FLUSHO
	NOFLSH LFlag = C.NOFLSH
	TOSTOP LFlag = C.TOSTOP
	PENDIN LFlag = C.PENDIN
	IEXTEN LFlag = C.IEXTEN
)

type SpecialCharacter int
const (
	VDISCARD = C.VDISCARD
	VEOF = C.VEOF
	VEOL = C.VEOL
	VEOL2 = C.VEOL2
	VERASE = C.VERASE
	VINTR = C.VINTR
	VKILL = C.VKILL
	VLNEXT = C.VLNEXT
	VMIN = C.VMIN
	VQUIT = C.VQUIT
	VREPRINT = C.VREPRINT
	VSTART = C.VSTART
	VSTOP = C.VSTOP
	VSUSP = C.VSUSP
	VTIME = C.VTIME
	VWERASE = C.VWERASE
)

type SetWhen C.int
const (
	TCSANOW SetWhen = C.TCSANOW
	TCSADRAIN SetWhen = C.TCSADRAIN
	TCSAFLUSH SetWhen = C.TCSAFLUSH
)

var IntSpeed map[int]C.speed_t = map[int]C.speed_t{
	0: C.B0,
	50: C.B50,
	75: C.B75,
	110: C.B110,
	134: C.B134,
	150: C.B150,
	200: C.B200,
	300: C.B300,
	600: C.B600,
	1200: C.B1200,
	1800: C.B1800,
	2400: C.B2400,
	4800: C.B4800,
	9600: C.B9600,
	19200: C.B19200,
	38400: C.B38400,
	57600: C.B57600,
	115200: C.B115200,
	230400: C.B230400,
}

func TcGetAttr(file *os.File) (*Termios, error) {
	r := &Termios{}
	state, errno := C.tcgetattr(C.int(file.Fd()), &r.ios)
	if state >= 0 {
		return r, nil
	}
	return nil, errno
}

func TcSetAttr(file *os.File, when SetWhen, tios *Termios) (err error) {
	state, errno := C.tcsetattr(C.int(file.Fd()), C.int(when), &tios.ios)
	if state < 0 { err = errno }
	return
}

func (tios *Termios) SetIFlags(iflag IFlag) {
	tios.ios.c_iflag = C.tcflag_t(iflag)
}

func (tios *Termios) SetOFlags(oflag OFlag) {
	tios.ios.c_oflag = C.tcflag_t(oflag)
}

func (tios *Termios) SetCFlags(cflag CFlag) {
	tios.ios.c_cflag = C.tcflag_t(cflag)
}

func (tios *Termios) SetLFlags(lflag LFlag) {
	tios.ios.c_lflag = C.tcflag_t(lflag)
}

func (tios *Termios) SetSpecial(key SpecialCharacter, value int) {
	tios.ios.c_cc[key] = C.cc_t(value)
}

func (tios *Termios) SetSpeed(speed int) error {
	speedmask, ok := IntSpeed[speed]
	if !ok { return errors.New("Unknown speed specified") }
	state, errno := C.cfsetispeed(&tios.ios, speedmask)
	if state < 0 { return errno }
	state, errno = C.cfsetospeed(&tios.ios, speedmask)
	if state < 0 { return errno }
	return nil
}

