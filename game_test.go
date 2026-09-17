package tcbscroller

import (
	"bytes"
	"image/png"
	"math"
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

func TestScrollCalculationIsAllocationFree(t *testing.T) {
	game := &Game{
		scrollForms: [8]scrollForm{{ySize: 55}},
		scrollText:  "ABCDEFGHIJKLMNOPQRSTUVWXYZ    ",
	}
	game.preprocessScrollText()

	allocs := testing.AllocsPerRun(1000, func() {
		game.scroll3D(4)
	})
	if allocs != 0 {
		t.Fatalf("scroll3D allocated %.2f objects per call, want 0", allocs)
	}
}

func TestScrollRecurrenceMatchesDirectTrigonometry(t *testing.T) {
	game := &Game{
		scrollForms: [8]scrollForm{
			{0, 0, 0, 0, 55, 0, 0},
			{0, 0, 0, 0, 55, 0, 2},
			{0, 0, 0, 0, 55, 20, 2},
			{200, 0, 0, 5, 55, 20, 2},
			{200, 0, 4, 5, 55, 20, 2},
			{200, -30, 4, 0, 55, 30, 2},
			{200, 40, -4, 5, -70, 40, -4},
			{150, 20, -3, 5, 55, 20, 2},
		},
		scrollText: "^0ABCDE^3FGHIJ^5KLMNO^6PQRST^7UVWXYZ          ",
		scrollX:    13,
		sinAdder:   2.75,
	}
	game.preprocessScrollText()
	want := *game
	referenceScroll3D(&want, 4)
	game.scroll3D(4)

	if game.form != want.form || game.addi != want.addi || game.scrollX != want.scrollX || game.sinAdder != want.sinAdder {
		t.Fatalf("scroll state = (%d, %d, %g, %g), want (%d, %d, %g, %g)",
			game.form, game.addi, game.scrollX, game.sinAdder,
			want.form, want.addi, want.scrollX, want.sinAdder)
	}
	for i := range game.printPos {
		got, expected := game.printPos[i], want.printPos[i]
		if got.letter != expected.letter || math.Abs(got.x-expected.x) > 1e-10 ||
			math.Abs(got.y-expected.y) > 1e-10 || math.Abs(got.z-expected.z) > 1e-12 {
			t.Fatalf("print position %d = %+v, want %+v", i, got, expected)
		}
	}
}

func referenceScroll3D(game *Game, scrollSpeed float64) {
	game.sinAdder += 0.02
	for i := range game.printPos {
		charIdx := game.addi + i
		if charIdx >= len(game.scrollText) {
			charIdx -= len(game.scrollText)
		}

		letter := game.scrollLetters[charIdx]
		if form := game.scrollFormChanges[charIdx]; form >= 0 {
			game.form = int(form)
		}
		sf := game.scrollForms[game.form]
		letterZ := sf.zSize*math.Sin(sf.zAdd+float64(charIdx)*sf.zAmount*0.01+game.sinAdder*sf.zSpeed) + 150
		letterY := sf.ySize*math.Cos(1.5+float64(charIdx)*sf.yAmount*0.01+game.sinAdder*sf.ySpeed) - 4
		scale := 250.0 / (250.0 + letterZ)
		letterX := -450.0 + float64(i)*32 - game.scrollX
		game.printPos[i] = printPos{
			x:      ((letterX - 16) * scale) + canvasWidth/2.0,
			y:      ((letterY - 14) * scale) + canvasHeight/2.0,
			z:      scale,
			letter: letter,
		}
	}

	for i := 1; i < len(game.printPos); i++ {
		item := game.printPos[i]
		j := i
		for j > 0 && game.printPos[j-1].z > item.z {
			game.printPos[j] = game.printPos[j-1]
			j--
		}
		game.printPos[j] = item
	}

	game.scrollX += scrollSpeed
	if game.scrollX >= 32 {
		game.scrollX -= 32
		game.addi++
		if game.addi >= len(game.scrollText) {
			game.addi = 0
		}
	}
}

func BenchmarkScrollCalculation(b *testing.B) {
	game := &Game{
		scrollForms: [8]scrollForm{{ySize: 55}},
		scrollText:  "ABCDEFGHIJKLMNOPQRSTUVWXYZ    ",
	}
	game.preprocessScrollText()
	b.ReportAllocs()
	for b.Loop() {
		game.scroll3D(4)
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
