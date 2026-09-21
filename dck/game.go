package tcbscroller

import originalassets "tcb-multi-plane-3d-scroller"

import (
	"bytes"
	"github.com/olivierh59500/democonstructionkit/presets"

	"fmt"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"log"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	audio "github.com/olivierh59500/democonstructionkit/sound/output"
	"github.com/olivierh59500/ym-player/pkg/stsound"
)

const (
	ScreenWidth  = 768
	ScreenHeight = 536

	canvasWidth  = 320
	canvasHeight = 200
	sampleRate   = 48000
	stageX       = 64
	stageY       = 60
)

var (
	rastersData = originalassets.
			DCKAssetRastersData()

	mountainsData = originalassets.
			DCKAssetMountainsData()

	logoData = originalassets.
			DCKAssetLogoData()

	fontData = originalassets.
			DCKAssetFontData()

	musicData = originalassets.DCKAssetMusicData()
)

const scrollShaderSource = scrolling.PlaneShaderSource

// YMPlayer adapts the mono YM synthesizer to Ebitengine's stereo PCM stream.
type YMPlayer struct {
	player *stsound.StSound
	buffer []int16
	mutex  sync.Mutex
	loop   bool
}

func NewYMPlayer(data []byte, rate int, loop bool) (*YMPlayer, error) {
	player := stsound.CreateWithRate(rate)
	if err := player.LoadMemory(data); err != nil {
		player.Destroy()
		return nil, fmt.Errorf("load YM data: %w", err)
	}
	player.SetLoopMode(loop)

	return &YMPlayer{
		player: player,
		buffer: make([]int16, 4096),
		loop:   loop,
	}, nil
}

// Read writes signed 16-bit little-endian stereo PCM directly into p.
func (y *YMPlayer) Read(p []byte) (n int, err error) {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	pcmBytes := len(p) - len(p)%4
	if pcmBytes == 0 {
		return 0, nil
	}
	if y.player == nil {
		clear(p[:pcmBytes])
		return 0, io.EOF
	}

	samplesNeeded := pcmBytes / 4
	processed := 0
	for processed < samplesNeeded {
		chunkSize := samplesNeeded - processed
		if chunkSize > len(y.buffer) {
			chunkSize = len(y.buffer)
		}

		if !y.player.Compute(y.buffer[:chunkSize], chunkSize) && !y.loop {
			clear(p[processed*4 : pcmBytes])
			err = io.EOF
			break
		}

		for i := 0; i < chunkSize; i++ {
			// The old reader applied 0.7 here and another 0.7 on the player.
			// Halving is allocation-free and preserves essentially the same level.
			sample := y.buffer[i] / 2
			offset := (processed + i) * 4
			p[offset] = byte(sample)
			p[offset+1] = byte(sample >> 8)
			p[offset+2] = byte(sample)
			p[offset+3] = byte(sample >> 8)
		}
		processed += chunkSize
	}

	return pcmBytes, err
}

func (y *YMPlayer) Close() error {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	if y.player != nil {
		y.player.Destroy()
		y.player = nil
	}
	return nil
}

// Game contains the complete standalone TCB screen.
type Game struct {
	planes        *scrolling.Planes
	planeRenderer *scrolling.PlaneRenderer
	rasters       *ebiten.Image
	mountains     *ebiten.Image
	logo          *ebiten.Image
	font          *ebiten.Image

	logoCenter *ebiten.Image
	initErr    error

	stripVertices []ebiten.Vertex
	stripIndices  []uint16

	bgSpeed [32]float64
	bgPos   [32]float64

	scrollText string

	logoSin  []float64
	dcounter int
	rotPos   float64
	rotAdd   float64
	next     int

	audioReady   bool
	audioContext *audio.Context
	audioPlayer  *audio.Player
	ymPlayer     *YMPlayer
	needsRedraw  bool
}

func NewGame() *Game {
	g := &Game{
		stripVertices: make([]ebiten.Vertex, 0, 64*4),
		stripIndices:  make([]uint16, 0, 64*6),
		rotAdd:        1,
		needsRedraw:   true,
	}

	speeds := [...]float64{8, 7.5, 7, 6.5, 6, 5.5, 5, 4.5, 4, 3.5, 3, 2.5, 2, 1.5, 1, 0.5}
	for i, speed := range speeds {
		g.bgSpeed[i] = speed
		g.bgSpeed[i+16] = speed
	}

	g.initLogoSin()
	g.initScrollText()
	g.preprocessScrollText()
	g.loadAssets()

	return g
}

func (g *Game) initLogoSin() {
	g.logoSin = make([]float64, 0, 40+(160*5+4)+(160*5+10)+160)
	for i := 0; i < 40; i++ {
		g.logoSin = append(g.logoSin, 0)
	}
	for i := 0; i < 160*5+4; i++ {
		g.logoSin = append(g.logoSin, 8*math.Sin(float64(i)*0.05-2))
	}
	for i := 0; i < 160*5+10; i++ {
		g.logoSin = append(g.logoSin, 8*math.Sin(float64(i)*0.15))
	}
	for i := 0; i < 160; i++ {
		g.logoSin = append(g.logoSin, 0)
	}
}

func (g *Game) initScrollText() {
	spc := "                             "
	g.scrollText = " ^0" + spc +
		"WOW, THIS DEMO SURE DOES LOOK GREAT..  BUT PERHAPS THE SCROLLINE LOOKS A BIT   TOO ORDINARY. " +
		"WELL, OKEY, LET US SWING IT UP AND DOWN. " +
		"^1 THIS IS THE LITTLE BIT OF EVERYTHING DEMO BY THE CAREBEARS. THERE ARE STAR RAY TYPE OF " +
		"BACKGROUND SCROLLERS, A DISTORTED TCB LOGO, " +
		"SOME GREAT MAD MAX MUSIC AND A SWINGING SCROLLINE OR..... PERHAPS EVEN MORE.............." +
		"^2...........  THIS IS BEGINNING TO LOOK " +
		"LIKE THE XXX INTERNATIONAL BALL DEMO SCREEN.                       " +
		"^3    BUT THEIR SCROLLINE WAS NOT THIS BIG. WE HOPE YOU DO NOT " +
		"THINK THAT WE HAVE TWO DIFFERENTLY SIZED FONTS. WE HAVE MANY MORE... ^4  " +
		"YEAH...  DO NOT LEAVE YET, THERE IS STILL MORE TO COME, JUST " +
		"WAIT AND SEE.  IF YOU THINK THIS IS HARD TO READ, WAIT TILL YOU HAVE " +
		"SEEN WHAT YOU ARE GOING TO SEE IN ABOUT THREE SECONDS.     " +
		"^5 THAT WAS NOT THREE SECONDS, BUT NOW YOU HAVE SEEN OUR THREE DIMENSIONAL " +
		"BENDING.. YOU MIGHT WONDER WHY WE HAVE NO PUNCTUATION EXCEPT " +
		"FOR THESE TWO ., . WE DO NOT EVEN HAVE THE LITTLE BLACK DOT BETWEEN HAVEN AND T, " +
		"HAVEN T, SEE... WELL, NOW THAT WE ARE OUT OF IDEAS WHAT " +
		"TO WRITE, WE CAN AS WELL EXPLAIN WHY. THE PROBLEM IS THAT ALL THE PART DEMOS " +
		"MUST WORK ON HALF A MEG AND EVERY CHARACTER TAKES ABOUT TEN " +
		"KILOBYTES. WE ARE GOING TO GREET SOME FOLKS NOW, SO LET US CHANGE WAVEFORM... " +
		"                        ^6             " +
		"MEGAGREETINGS GO TO ALL THE OTHER MEMBERS OF THE UNION. WE DO NOT FEEL " +
		"LIKE GREETING TO MUCH COZ WE DO NOT HAVE THOSE LITTLE BENT LINES, SO " +
		"WE CAN NOT MAKE COMMENTS. BUT JUST ONCE YOU WILL HAVE TO PRETEND YOU SAW " +
		"ONE OF THOSE, IT SHOULD HAVE COME INSTEAD OF THE SPACE BETWEEN " +
		"THE WORDS COOL AND YOUR. HERE WE GO... HELLO, AN COOL  YOUR NEW INTRO IS " +
		"REALLY SOMETHING .                    ^7 YOU WILL HAVE " +
		"TO READ IN THE MAIN SCROLLTEXT FOR MORE GREETINGS....  BYE.............. " +
		"                                             "
}

func (g *Game) preprocessScrollText() {
	var err error
	g.planes, err = scrolling.NewPlanes(scrolling.PlanesConfig{
		Slots: presets.TCBPlaneSlots(g.scrollText, 32), Forms: presets.TCBScrollForms(),
		Visible: 30, PhaseStep: .02,
		Projection: scrolling.PlaneProjection{Focal: 250, Depth: 150, OriginX: -450, CenterX: 160, CenterY: 100, XBias: -16, YBias: -14, VerticalOffset: -4},
	})
	if err != nil {
		g.initErr = err
	}
}

func (g *Game) loadAssets() {
	img, _, err := image.Decode(bytes.NewReader(rastersData))
	if err != nil {
		log.Printf("load rasters: %v", err)
		g.rasters = ebiten.NewImage(canvasWidth, canvasHeight)
		g.rasters.Fill(color.RGBA{R: 255, B: 255, A: 255})
	} else {
		g.rasters = ebiten.NewImageFromImage(img)
	}

	img, _, err = image.Decode(bytes.NewReader(mountainsData))
	if err != nil {
		log.Printf("load mountains: %v", err)
		g.mountains = ebiten.NewImage(1024, 320)
	} else {
		g.mountains = ebiten.NewImageFromImage(img)
	}

	img, _, err = image.Decode(bytes.NewReader(logoData))
	if err != nil {
		log.Printf("load logo: %v", err)
		g.logo = ebiten.NewImage(320, 48)
	} else {
		g.logo = ebiten.NewImageFromImage(img)
	}
	g.logoCenter = g.logo.SubImage(image.Rect(114, 0, 193, 15)).(*ebiten.Image)

	img, _, err = image.Decode(bytes.NewReader(fontData))
	if err != nil {
		log.Printf("load font: %v", err)
		g.font = ebiten.NewImage(320, 198)
	} else {
		g.font = ebiten.NewImageFromImage(img)
	}
	g.cacheFontTileRects()
}

func (g *Game) cacheFontTileRects() {
	spec, _ := presets.FindFont("tcb-multi-plane-3d-scroller")
	metrics, err := spec.Build(g.font.Bounds())
	if err != nil {
		g.initErr = err
		return
	}
	g.planeRenderer, err = scrolling.NewPlaneRenderer(scrolling.Face{Atlas: g.font, Metrics: metrics}, g.rasters)
	if err != nil {
		g.initErr = err
	}
}

func (g *Game) initAudio() {
	g.audioContext = audio.NewContext(sampleRate)

	var err error
	g.ymPlayer, err = NewYMPlayer(musicData, sampleRate, true)
	if err != nil {
		log.Printf("create YM player: %v", err)
		return
	}

	g.audioPlayer, err = g.audioContext.NewPlayer(g.ymPlayer)
	if err != nil {
		log.Printf("create audio player: %v", err)
		_ = g.ymPlayer.Close()
		g.ymPlayer = nil
		return
	}
	g.audioPlayer.Play()
}

func (g *Game) Update() error {
	if g.initErr != nil {
		return g.initErr
	}
	if !g.audioReady {
		g.audioReady = true
		g.initAudio()
	}
	g.needsRedraw = true

	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

	for i := range g.bgPos {
		g.bgPos[i] -= g.bgSpeed[i]
		if g.bgPos[i] <= -256 {
			g.bgPos[i] += 256
		}
	}

	g.dcounter++
	if g.dcounter > len(g.logoSin)-80 {
		g.dcounter = 0
	}

	g.rotPos += g.rotAdd * 0.08
	if g.rotPos > 1 {
		g.rotPos = -1
		g.next++
		if g.next > 1 {
			g.next = 0
		}
	}

	g.scroll3D(4)
	return nil
}

func (g *Game) scroll3D(speed float64) {
	if g.planes != nil {
		g.initErr = g.planes.Step(speed)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	if !g.needsRedraw || g.initErr != nil {
		return
	}
	g.needsRedraw = false

	screen.Fill(color.Black)
	stage := screen.SubImage(image.Rect(stageX, stageY, stageX+640, stageY+400)).(*ebiten.Image)

	g.stripVertices = g.stripVertices[:0]
	g.stripIndices = g.stripIndices[:0]
	for i := 0; i < 16; i++ {
		xPos := int(g.bgPos[i]) * 2
		yPos := i * 10
		g.appendMountainStrip(i, xPos, yPos)
	}
	for i := 16; i < 32; i++ {
		xPos := int(g.bgPos[i]) * 2
		yPos := i*10 + 84
		g.appendMountainStrip(i, xPos, yPos)
	}
	stage.DrawTriangles(g.stripVertices, g.stripIndices, g.mountains, nil)

	g.stripVertices = g.stripVertices[:0]
	g.stripIndices = g.stripIndices[:0]
	for i := 0; i < 32; i++ {
		xOffset := g.logoSin[g.dcounter+i]
		g.stripVertices, g.stripIndices = appendTexturedQuad(
			g.stripVertices, g.stripIndices,
			float32(stageX+2*(8+xOffset)), float32(stageY+2*(96+i)), 606, 2,
			0, float32(16+i), 303, 1,
		)
	}
	screen.DrawTriangles(g.stripVertices, g.stripIndices, g.logo, nil)

	if g.logoCenter != nil {
		op := &ebiten.DrawImageOptions{}
		if g.next != 0 {
			op.GeoM.Scale(1, -1)
			op.GeoM.Translate(0, 16)
		}
		op.GeoM.Translate(-40, -8)
		op.GeoM.Scale(1, g.rotPos)
		op.GeoM.Translate(160, 88)
		op.GeoM.Scale(2, 2)
		op.GeoM.Translate(stageX, stageY)
		screen.DrawImage(g.logoCenter, op)
	}

	g.drawScroll3D(stage)
}

func (g *Game) appendMountainStrip(layer, xPos, yPos int) {
	for _, offset := range [...]int{0, 640} {
		g.stripVertices, g.stripIndices = appendTexturedQuad(
			g.stripVertices, g.stripIndices,
			float32(stageX+xPos+offset), float32(stageY+yPos), 1024, 10,
			0, float32(layer*10), 1024, 10,
		)
	}
}

func (g *Game) drawScroll3D(stage *ebiten.Image) {
	if g.planeRenderer != nil {
		g.planeRenderer.Draw(stage, g.planes.Points(), scrolling.PlaneDraw{OriginX: stageX, OriginY: stageY, ScaleX: 2, ScaleY: 2})
	}
}

func (g *Game) Layout(_, _ int) (int, int) {
	return ScreenWidth, ScreenHeight
}

// Cleanup releases the audio resources owned by the game.
func (g *Game) Cleanup() {
	if g.planeRenderer != nil {
		g.planeRenderer.Close()
	}
	if g.audioPlayer != nil {
		_ = g.audioPlayer.Close()
		g.audioPlayer = nil
	}
	if g.ymPlayer != nil {
		_ = g.ymPlayer.Close()
		g.ymPlayer = nil
	}
}

func appendTexturedQuad(vertices []ebiten.Vertex, indices []uint16, dstX, dstY, dstWidth, dstHeight, srcX, srcY, srcWidth, srcHeight float32) ([]ebiten.Vertex, []uint16) {
	base := uint16(len(vertices))
	vertices = append(vertices,
		ebiten.Vertex{DstX: dstX, DstY: dstY, SrcX: srcX, SrcY: srcY, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		ebiten.Vertex{DstX: dstX + dstWidth, DstY: dstY, SrcX: srcX + srcWidth, SrcY: srcY, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		ebiten.Vertex{DstX: dstX, DstY: dstY + dstHeight, SrcX: srcX, SrcY: srcY + srcHeight, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		ebiten.Vertex{DstX: dstX + dstWidth, DstY: dstY + dstHeight, SrcX: srcX + srcWidth, SrcY: srcY + srcHeight, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
	)
	indices = append(indices, base, base+1, base+2, base+1, base+2, base+3)
	return vertices, indices
}

func stepSinCosForward(sinValue, cosValue, sinStep, cosStep float64) (float64, float64) {
	return sinValue*cosStep + cosValue*sinStep, cosValue*cosStep - sinValue*sinStep
}
