// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"

	"github.com/xiaoshengduan/gio-fly/f32"
	"github.com/xiaoshengduan/gio-fly/layout"
	"github.com/xiaoshengduan/gio-fly/op"
	"github.com/xiaoshengduan/gio-fly/op/clip"
	"github.com/xiaoshengduan/gio-fly/op/paint"
	"github.com/xiaoshengduan/gio-fly/unit"
)

// Image is a widget that displays an image.
type Image struct {
	// Src is the image to display.
	Src paint.ImageOp
	// Fit specifies how to scale the image to the constraints.
	// By default it does not do any scaling.
	Fit Fit
	// Position specifies where to position the image within
	// the constraints.
	Position layout.Direction
	// Scale is the ratio of image pixels to
	// dps. If Scale is zero Image falls back to
	// a scale that match a standard 72 DPI.
	Scale float32
}

const defaultScale = float32(160.0 / 72.0)

func (im Image) Layout(gtx layout.Context) layout.Dimensions {
	scale := im.Scale
	if scale == 0 {
		scale = defaultScale
	}

	size := im.Src.Size()
	wf, hf := float32(size.X), float32(size.Y)
	w, h := gtx.Dp(unit.Dp(wf*scale)), gtx.Dp(unit.Dp(hf*scale))

	dims, trans := im.Fit.scale(gtx.Constraints, im.Position, layout.Dimensions{Size: image.Pt(w, h)})
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()

	pixelScale := scale * gtx.Metric.PxPerDp
	trans = trans.Mul(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(pixelScale, pixelScale)))
	defer op.Affine(trans).Push(gtx.Ops).Pop()

	im.Src.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	return dims
}
