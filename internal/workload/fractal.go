package workload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
)

// Fractal renders a Mandelbrot image. Each unit is one square tile.
type Fractal struct{}

// FractalParams are the job parameters. Zero values take the defaults.
type FractalParams struct {
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	TileSize int     `json:"tile_size"`
	MaxIter  int     `json:"max_iter"`
	XMin     float64 `json:"x_min"`
	XMax     float64 `json:"x_max"`
	YMin     float64 `json:"y_min"`
	YMax     float64 `json:"y_max"`
}

type fractalTile struct {
	FractalParams
	X0 int `json:"x0"`
	Y0 int `json:"y0"`
	W  int `json:"w"`
	H  int `json:"h"`
}

// Name returns "fractal".
func (Fractal) Name() string { return "fractal" }

func parseFractalParams(raw []byte) (FractalParams, error) {
	var p FractalParams
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return p, fmt.Errorf("fractal params: %w", err)
		}
	}
	if p.Width == 0 {
		p.Width = 1024
	}
	if p.Height == 0 {
		p.Height = 1024
	}
	if p.TileSize == 0 {
		p.TileSize = 256
	}
	if p.MaxIter == 0 {
		p.MaxIter = 1000
	}
	if p.XMin == 0 && p.XMax == 0 {
		p.XMin, p.XMax = -2.0, 1.0
	}
	if p.YMin == 0 && p.YMax == 0 {
		p.YMin, p.YMax = -1.5, 1.5
	}
	if p.Width < 1 || p.Height < 1 || p.TileSize < 1 || p.MaxIter < 1 {
		return p, fmt.Errorf("fractal params: width, height, tile_size and max_iter must be positive")
	}
	if p.Width > 16384 || p.Height > 16384 {
		return p, fmt.Errorf("fractal params: width and height must be 16384 or less")
	}
	if p.XMin >= p.XMax || p.YMin >= p.YMax {
		return p, fmt.Errorf("fractal params: x_min must be less than x_max, and y_min less than y_max")
	}
	return p, nil
}

// Split returns one payload per tile, row by row.
func (Fractal) Split(params []byte) ([][]byte, error) {
	p, err := parseFractalParams(params)
	if err != nil {
		return nil, err
	}
	var units [][]byte
	for y := 0; y < p.Height; y += p.TileSize {
		for x := 0; x < p.Width; x += p.TileSize {
			t := fractalTile{
				FractalParams: p, X0: x, Y0: y,
				W: min(p.TileSize, p.Width-x), H: min(p.TileSize, p.Height-y),
			}
			b, err := json.Marshal(t)
			if err != nil {
				return nil, err
			}
			units = append(units, b)
		}
	}
	return units, nil
}

// Run returns one gray byte per pixel of the tile, row by row.
func (Fractal) Run(ctx context.Context, payload []byte) ([]byte, error) {
	var t fractalTile
	if err := json.Unmarshal(payload, &t); err != nil {
		return nil, fmt.Errorf("fractal tile: %w", err)
	}
	if t.W < 1 || t.H < 1 || t.MaxIter < 1 || t.Width < 1 || t.Height < 1 {
		return nil, fmt.Errorf("fractal tile: invalid size")
	}
	out := make([]byte, t.W*t.H)
	dx := (t.XMax - t.XMin) / float64(t.Width)
	dy := (t.YMax - t.YMin) / float64(t.Height)
	for py := 0; py < t.H; py++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ci := t.YMin + float64(float64(t.Y0+py)*dy)
		for px := 0; px < t.W; px++ {
			cr := t.XMin + float64(float64(t.X0+px)*dx)
			n := mandelbrot(cr, ci, t.MaxIter)
			out[py*t.W+px] = byte(255 * n / t.MaxIter)
		}
	}
	return out, nil
}

// mandelbrot returns the number of iterations before the point escapes.
//
// Each product is wrapped in an explicit float64 conversion. The Go spec then
// forbids fused multiply-add, so arm64 and amd64 volunteers give the same bytes.
func mandelbrot(cr, ci float64, maxIter int) int {
	zr, zi := 0.0, 0.0
	for n := 0; n < maxIter; n++ {
		zr2 := float64(zr * zr)
		zi2 := float64(zi * zi)
		if zr2+zi2 > 4 {
			return n
		}
		zi = float64(2*float64(zr*zi)) + ci
		zr = zr2 - zi2 + cr
	}
	return maxIter
}

// Merge places the tiles in one image and returns it as PNG.
func (Fractal) Merge(params []byte, outputs [][]byte) ([]byte, error) {
	p, err := parseFractalParams(params)
	if err != nil {
		return nil, err
	}
	img := image.NewGray(image.Rect(0, 0, p.Width, p.Height))
	i := 0
	for y := 0; y < p.Height; y += p.TileSize {
		for x := 0; x < p.Width; x += p.TileSize {
			if i >= len(outputs) {
				return nil, fmt.Errorf("fractal merge: %d outputs, need more", len(outputs))
			}
			w, h := min(p.TileSize, p.Width-x), min(p.TileSize, p.Height-y)
			tile := outputs[i]
			if len(tile) != w*h {
				return nil, fmt.Errorf("fractal merge: tile %d has %d bytes, want %d", i, len(tile), w*h)
			}
			for row := 0; row < h; row++ {
				copy(img.Pix[(y+row)*img.Stride+x:], tile[row*w:(row+1)*w])
			}
			i++
		}
	}
	if i != len(outputs) {
		return nil, fmt.Errorf("fractal merge: %d outputs, want %d", len(outputs), i)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
