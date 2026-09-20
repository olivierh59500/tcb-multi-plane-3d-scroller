package tcbscroller

import originalassets "tcb-multi-plane-3d-scroller"

import (
	"bytes"

	"fmt"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"log"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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

const scrollShaderSource = `//kage:unit pixels

package main

func Fragment(dstPos vec4, srcPos vec2, custom vec4) vec4 {
	glyph := imageSrc0UnsafeAt(srcPos)
	rasterY := floor(custom.r) + 0.5
	// Coordinates for secondary images use image 0's texture space.
	raster := imageSrc1UnsafeAt(imageSrc0Origin() + vec2(0.5, rasterY))
	return vec4(raster.rgb * glyph.a, raster.a * glyph.a)
}
`

type scrollForm struct {
	zSize   float64
	zAmount float64
	zSpeed  float64
	zAdd    float64
	ySize   float64
	yAmount float64
	ySpeed  float64
}

type printPos struct {
	x, y, z float64
	letter  byte
}

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
	scrollRenderer *scrolling.Scrolling
	scrollBatch    *composite.QuadBatch
	rasters        *ebiten.Image
	mountains      *ebiten.Image
	logo           *ebiten.Image
	font           *ebiten.Image

	logoCenter   *ebiten.Image
	scrollShader *ebiten.Shader
	initErr      error

	fontTileRects [128]image.Rectangle
	stripVertices []ebiten.Vertex
	stripIndices  []uint16

	bgSpeed [32]float64
	bgPos   [32]float64

	scrollForms       [8]scrollForm
	form              int
	scrollX           float64
	scrollText        string
	scrollLetters     []byte
	scrollFormChanges []int8
	addi              int
	sinAdder          float64
	printPos          [30]printPos

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

	g.scrollForms = [8]scrollForm{
		{0, 0, 0, 0, 55, 0, 0},
		{0, 0, 0, 0, 55, 0, 2},
		{0, 0, 0, 0, 55, 20, 2},
		{200, 0, 0, 5, 55, 20, 2},
		{200, 0, 4, 5, 55, 20, 2},
		{200, -30, 4, 0, 55, 30, 2},
		{200, 40, -4, 5, -70, 40, -4},
		{150, 20, -3, 5, 55, 20, 2},
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

	var err error
	g.scrollShader, err = ebiten.NewShader([]byte(scrollShaderSource))
	if err != nil {
		g.initErr = fmt.Errorf("compile scroller shader: %w", err)
	}

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
	g.scrollLetters = make([]byte, len(g.scrollText))
	g.scrollFormChanges = make([]int8, len(g.scrollText))
	for i := range g.scrollFormChanges {
		g.scrollFormChanges[i] = -1
	}

	for i := range g.scrollText {
		letter := g.scrollText[i]
		if letter == '^' && i+1 < len(g.scrollText) && g.scrollText[i+1] >= '0' && g.scrollText[i+1] <= '7' {
			g.scrollFormChanges[i] = int8(g.scrollText[i+1] - '0')
			letter = g.scrollText[(i-1+len(g.scrollText))%len(g.scrollText)]
		} else if i >= 2 && g.scrollText[i-1] == '^' && letter >= '0' && letter <= '7' {
			letter = g.scrollText[i-2]
		}
		g.scrollLetters[i] = letter
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
	charMap := [6][10]rune{
		{0, '!', 0, 0, 0, 0, 0, 0, '(', ')'},
		{0, 0, ',', 0, '.', 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, ':', ';', 0, 0},
		{0, 0, 0, 'A', 'B', 'C', 'D', 'E', 'F', 'G'},
		{'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q'},
		{'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', 0},
	}

	for row := range charMap {
		for col, ch := range charMap[row] {
			if ch == 0 {
				continue
			}
			x := col * 32
			y := row * 33
			g.fontTileRects[ch] = image.Rect(x, y, x+32, y+33)
		}
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

func (g *Game) scroll3D(scrollSpeed float64) {
	g.sinAdder += 0.02
	activeForm := -1
	previousCharIdx := -2
	var zSin, zCos, ySin, yCos float64
	var zStepSin, zStepCos, yStepSin, yStepCos float64

	for i := range g.printPos {
		charIdx := g.addi + i
		if charIdx >= len(g.scrollText) {
			charIdx -= len(g.scrollText)
		}

		letter := g.scrollLetters[charIdx]
		if form := g.scrollFormChanges[charIdx]; form >= 0 {
			g.form = int(form)
		}
		sf := g.scrollForms[g.form]

		if activeForm != g.form || charIdx != previousCharIdx+1 {
			if sf.zSize != 0 {
				zSin, zCos = math.Sincos(sf.zAdd + float64(charIdx)*sf.zAmount*0.01 + g.sinAdder*sf.zSpeed)
				zStepSin, zStepCos = math.Sincos(sf.zAmount * 0.01)
			}
			ySin, yCos = math.Sincos(1.5 + float64(charIdx)*sf.yAmount*0.01 + g.sinAdder*sf.ySpeed)
			yStepSin, yStepCos = math.Sincos(sf.yAmount * 0.01)
			activeForm = g.form
		} else {
			if sf.zSize != 0 {
				zSin, zCos = stepSinCosForward(zSin, zCos, zStepSin, zStepCos)
			}
			ySin, yCos = stepSinCosForward(ySin, yCos, yStepSin, yStepCos)
		}
		previousCharIdx = charIdx

		letterZ := sf.zSize*zSin + 150
		letterY := sf.ySize*yCos - 4
		scale := 250.0 / (250.0 + letterZ)
		letterX := -450.0 + float64(i)*32 - g.scrollX
		g.printPos[i] = printPos{
			x:      ((letterX - 16) * scale) + canvasWidth/2.0,
			y:      ((letterY - 14) * scale) + canvasHeight/2.0,
			z:      scale,
			letter: letter,
		}
	}

	// This fixed, tiny list is faster and allocation-free with insertion sort.
	for i := 1; i < len(g.printPos); i++ {
		item := g.printPos[i]
		j := i
		for j > 0 && g.printPos[j-1].z > item.z {
			g.printPos[j] = g.printPos[j-1]
			j--
		}
		g.printPos[j] = item
	}

	g.scrollX += scrollSpeed
	if g.scrollX >= 32 {
		g.scrollX -= 32
		g.addi++
		if g.addi >= len(g.scrollText) {
			g.addi = 0
		}
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
	if g.scrollShader == nil {
		return
	}
	if g.scrollRenderer == nil {
		var err error
		g.scrollRenderer, err = scrolling.FromImages(make([]*ebiten.Image, len(g.printPos)), 1)
		if err != nil {
			panic(err)
		}
		g.scrollBatch = composite.NewQuadBatch(len(g.printPos))
		g.scrollBatch.AlternateDiagonal = true
	}
	g.scrollBatch.Shader = g.scrollShader
	g.scrollBatch.ShaderOptions.Images[0] = g.font
	g.scrollBatch.ShaderOptions.Images[1] = g.rasters
	g.scrollBatch.Begin(stage, g.font)
	state := scrolling.IdentityState()
	state.Paint = func(dst *ebiten.Image, s scrolling.Sample, op ebiten.DrawImageOptions) {
		p := g.printPos[s.Index]
		if p.letter == 0 || p.z <= 0 {
			return
		}
		ch := rune(p.letter)
		var r image.Rectangle
		if ch >= 0 && ch < rune(len(g.fontTileRects)) {
			r = g.fontTileRects[ch]
		}
		if r.Empty() && ch >= 'a' && ch <= 'z' {
			r = g.fontTileRects[ch-'a'+'A']
		}
		if r.Empty() {
			return
		}
		scale := float32(p.z)
		localY := float32(p.y) - 16.5*scale
		x, y, w, h := float32(stageX)+2*(float32(p.x)-16*scale), float32(stageY)+2*localY, 64*scale, 66*scale
		sx, sy := float32(r.Min.X), float32(r.Min.Y)
		vertices := [4]ebiten.Vertex{
			{DstX: x, DstY: y, SrcX: sx, SrcY: sy, ColorR: localY, ColorG: 1, ColorB: 1, ColorA: 1},
			{DstX: x + w, DstY: y, SrcX: sx + 32, SrcY: sy, ColorR: localY, ColorG: 1, ColorB: 1, ColorA: 1},
			{DstX: x, DstY: y + h, SrcX: sx, SrcY: sy + 33, ColorR: localY + 33*scale, ColorG: 1, ColorB: 1, ColorA: 1},
			{DstX: x + w, DstY: y + h, SrcX: sx + 32, SrcY: sy + 33, ColorR: localY + 33*scale, ColorG: 1, ColorB: 1, ColorA: 1},
		}
		g.scrollBatch.Quad(vertices)
	}
	g.scrollRenderer.DrawAt(stage, state)
	g.scrollBatch.Flush()
}

func (g *Game) Layout(_, _ int) (int, int) {
	return ScreenWidth, ScreenHeight
}

// Cleanup releases the audio resources owned by the game.
func (g *Game) Cleanup() {
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
