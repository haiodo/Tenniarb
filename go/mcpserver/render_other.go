//go:build !darwin

package mcpserver

import "errors"

func newRasterizer() (rasterizer, error) {
	return nil, errors.New("render needs the CoreGraphics backend of gowt, which exists on macOS only")
}
