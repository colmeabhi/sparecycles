package volunteer

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestCorruptAlwaysChangesTheOutput(t *testing.T) {
	for _, output := range [][]byte{nil, {}, {0}, {1, 2, 3}, bytes.Repeat([]byte{7}, 4096)} {
		for i := 0; i < 100; i++ {
			got := corrupt(output)
			if bytes.Equal(got, output) {
				t.Fatalf("corrupt(%v) returned the same bytes", output)
			}
			if len(output) > 0 && len(got) != len(output) {
				t.Fatalf("corrupt changed the length from %d to %d", len(output), len(got))
			}
		}
	}
}

func TestCorruptDoesNotChangeItsInput(t *testing.T) {
	output := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	corrupt(output)
	if !bytes.Equal(output, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("input changed to %v", output)
	}
}

// Two faulty volunteers must not agree by chance, or they could outvote an honest one.
func TestTwoCorruptionsDiffer(t *testing.T) {
	output := bytes.Repeat([]byte{42}, 1024)
	same := 0
	for i := 0; i < 1000; i++ {
		if bytes.Equal(corrupt(output), corrupt(output)) {
			same++
		}
	}
	if same > 0 {
		t.Fatalf("%d of 1000 pairs of wrong outputs were equal", same)
	}
}

func TestFaultyRate(t *testing.T) {
	output := []byte("real output")
	honest := &Volunteer{cfg: Config{FaultyRate: 0}, log: quiet}
	faulty := &Volunteer{cfg: Config{FaultyRate: 1}, log: quiet}
	for i := 0; i < 100; i++ {
		if got := honest.maybeCorrupt("u", output); !bytes.Equal(got, output) {
			t.Fatal("rate 0 changed an output")
		}
		if got := faulty.maybeCorrupt("u", output); bytes.Equal(got, output) {
			t.Fatal("rate 1 sent a real output")
		}
	}
}
