// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"
	"testing"

	"github.com/xiaoshengduan/gio-fly/op"
)

func TestStack(t *testing.T) {
	gtx := Context{
		Ops: new(op.Ops),
		Constraints: Constraints{
			Max: image.Pt(100, 100),
		},
	}
	exp := image.Point{X: 60, Y: 70}
	dims := Stack{Alignment: Center}.Layout(gtx,
		Expanded(func(gtx Context) Dimensions {
			return Dimensions{Size: exp}
		}),
		Stacked(func(gtx Context) Dimensions {
			return Dimensions{Size: image.Point{X: 50, Y: 50}}
		}),
	)
	if got := dims.Size; got != exp {
		t.Errorf("Stack ignored Expanded size, got %v expected %v", got, exp)
	}
}

func TestFlex(t *testing.T) {
	gtx := Context{
		Ops: new(op.Ops),
		Constraints: Constraints{
			Min: image.Pt(100, 100),
			Max: image.Pt(100, 100),
		},
	}
	dims := Flex{}.Layout(gtx)
	if got := dims.Size; got != gtx.Constraints.Min {
		t.Errorf("Flex ignored minimum constraints, got %v expected %v", got, gtx.Constraints.Min)
	}
}

func TestDirection(t *testing.T) {
	max := image.Pt(100, 100)
	for _, tc := range []struct {
		dir Direction
		exp image.Point
	}{
		{N, image.Pt(max.X, 0)},
		{S, image.Pt(max.X, 0)},
		{E, image.Pt(0, max.Y)},
		{W, image.Pt(0, max.Y)},
		{NW, image.Pt(0, 0)},
		{NE, image.Pt(0, 0)},
		{SE, image.Pt(0, 0)},
		{SW, image.Pt(0, 0)},
		{Center, image.Pt(0, 0)},
	} {
		t.Run(tc.dir.String(), func(t *testing.T) {
			gtx := Context{
				Ops:         new(op.Ops),
				Constraints: Exact(max),
			}
			var min image.Point
			tc.dir.Layout(gtx, func(gtx Context) Dimensions {
				min = gtx.Constraints.Min
				return Dimensions{}
			})
			if got, exp := min, tc.exp; got != exp {
				t.Errorf("got %v; expected %v", got, exp)
			}
		})
	}
}
