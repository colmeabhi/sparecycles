package workload

import (
	"bytes"
	"context"
	"image/png"
	"testing"
)

func TestFractalSplitRunMerge(t *testing.T) {
	f := Fractal{}
	params := []byte(`{"width":100,"height":60,"tile_size":32,"max_iter":50}`)

	units, err := f.Split(params)
	if err != nil {
		t.Fatal(err)
	}
	// 100 / 32 -> 4 columns, 60 / 32 -> 2 rows.
	if len(units) != 8 {
		t.Fatalf("units = %d, want 8", len(units))
	}

	outputs := make([][]byte, len(units))
	for i, u := range units {
		if outputs[i], err = f.Run(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}

	result, err := f.Merge(params, outputs)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 100 || b.Dy() != 60 {
		t.Fatalf("image size = %v, want 100x60", b)
	}
}

// Voting compares bytes, so Run must give the same output each time.
func TestFractalRunIsDeterministic(t *testing.T) {
	f := Fractal{}
	units, err := f.Split([]byte(`{"width":64,"height":64,"tile_size":64,"max_iter":200}`))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := f.Run(context.Background(), units[0])
	b, _ := f.Run(context.Background(), units[0])
	if !bytes.Equal(a, b) {
		t.Fatal("two runs of the same unit differ")
	}
}

func TestFractalBadParams(t *testing.T) {
	for _, p := range []string{`{"width":-1}`, `{"x_min":2,"x_max":1}`, `not json`} {
		if _, err := (Fractal{}).Split([]byte(p)); err == nil {
			t.Fatalf("params %s: expected an error", p)
		}
	}
}
