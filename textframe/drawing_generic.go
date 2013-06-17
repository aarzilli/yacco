// +build !linux

package textframe

func GetOptimizedDrawing(img draw.Image) DrawingFuncs {
	return defaultDrawingFuncs
}
