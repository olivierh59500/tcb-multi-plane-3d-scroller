package tcbscroller

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestYMPlayerReadProducesStereoWithoutAllocating(t *testing.T) {
	player, err := NewYMPlayer(musicData, sampleRate, true)
	if err != nil {
		t.Fatalf("NewYMPlayer: %v", err)
	}
	t.Cleanup(func() {
		if err := player.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	buffer := make([]byte, 4096*4)
	n, err := player.Read(buffer)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != len(buffer) {
		t.Fatalf("Read returned %d bytes, want %d", n, len(buffer))
	}

	nonZero := false
	for i := 0; i < n; i += 4 {
		if buffer[i] != buffer[i+2] || buffer[i+1] != buffer[i+3] {
			t.Fatalf("PCM frame %d differs between left and right channels", i/4)
		}
		nonZero = nonZero || buffer[i] != 0 || buffer[i+1] != 0
	}
	if !nonZero {
		t.Fatal("PCM stream unexpectedly contains only silence")
	}

	var readN int
	var readErr error
	allocs := testing.AllocsPerRun(100, func() {
		readN, readErr = player.Read(buffer)
	})
	if readErr != nil {
		t.Fatalf("allocation-check Read: %v", readErr)
	}
	if readN != len(buffer) {
		t.Fatalf("allocation-check Read returned %d bytes, want %d", readN, len(buffer))
	}
	if allocs != 0 {
		t.Fatalf("YMPlayer.Read allocated %.2f objects per call, want 0", allocs)
	}
}

func TestYMPlayerCloseIsIdempotent(t *testing.T) {
	player, err := NewYMPlayer(musicData, sampleRate, true)
	if err != nil {
		t.Fatalf("NewYMPlayer: %v", err)
	}
	if err := player.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := player.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestScrollShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader([]byte(scrollShaderSource))
	if err != nil {
		t.Fatalf("compile scroller shader: %v", err)
	}
	shader.Deallocate()
}

func TestRasterMatchesShaderAssumptions(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(rastersData))
	if err != nil {
		t.Fatalf("decode raster: %v", err)
	}
	if got, want := img.Bounds().Dx(), 1; got != want {
		t.Fatalf("raster width = %d, want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), canvasHeight; got != want {
		t.Fatalf("raster height = %d, want %d", got, want)
	}
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		_, _, _, alpha := img.At(img.Bounds().Min.X, y).RGBA()
		if alpha != 0xffff {
			t.Fatalf("raster alpha at row %d = %#04x, want opaque", y, alpha)
		}
	}
}

func BenchmarkYMPlayerRead(b *testing.B) {
	player, err := NewYMPlayer(musicData, sampleRate, true)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := player.Close(); err != nil {
			b.Errorf("Close: %v", err)
		}
	})

	buffer := make([]byte, 4096*4)
	b.ReportAllocs()
	b.SetBytes(int64(len(buffer)))
	for b.Loop() {
		if _, err := player.Read(buffer); err != nil {
			b.Fatal(err)
		}
	}
}
