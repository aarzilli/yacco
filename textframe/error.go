package textframe

import (
	"fmt"
)

type FrameError struct {
	fetype int
	msg    string
	n      int
}

const (
	FE_NOT_ENOUGH_COLOR_LINES = iota
	FE_NOT_ENOUGH_COLORS
	FE_NOT_WIDE_ENOUGH
)

var FrameErrorNotEnoughColorLines = &FrameError{FE_NOT_ENOUGH_COLOR_LINES, "Not enough color lines", -1}
var ScrollFrameErrorNotWideEnough = &FrameError{FE_NOT_WIDE_ENOUGH, "Scrollbar not wide enough", -1 }

func FrameErrorNotEnoughColors(n int) *FrameError {
	return &FrameError{FE_NOT_ENOUGH_COLORS, fmt.Sprintf("Not enough colors in row %d", n), n}
}

func (fe *FrameError) Error() string {
	return fe.msg
}
