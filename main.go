package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"log"
	"math"
	"sort"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/ym-player/pkg/stsound"
)

const (
	screenWidth  = 768
	screenHeight = 536
	canvasWidth  = 320
	canvasHeight = 200
	fov          = 250
)

// Embedded assets
var (
	//go:embed assets/rast.png
	rastersData []byte
	//go:embed assets/mountains.png
	mountainsData []byte
	//go:embed assets/logo.png
	logoData []byte
	//go:embed assets/bgfont.png
	fontData []byte
	//go:embed assets/Thundercats.ym
	musicData []byte
)

// ScrollForm defines parameters for scroll wave forms
type ScrollForm struct {
	zSize   float64
	zAmount float64
	zSpeed  float64
	zAdd    float64
	ySize   float64
	yAmount float64
	ySpeed  float64
}

// PrintPos represents a character position for 3D rendering
type PrintPos struct {
	x, y, z float64
	letter  string
}

// YMPlayer wraps the YM player for Ebiten audio
type YMPlayer struct {
	player       *stsound.StSound
	sampleRate   int
	buffer       []int16
	mutex        sync.Mutex
	position     int64
	totalSamples int64
	loop         bool
	volume       float64
}

// NewYMPlayer creates a new YM player instance
func NewYMPlayer(data []byte, sampleRate int, loop bool) (*YMPlayer, error) {
	player := stsound.CreateWithRate(sampleRate)

	if err := player.LoadMemory(data); err != nil {
		player.Destroy()
		return nil, fmt.Errorf("failed to load YM data: %w", err)
	}

	player.SetLoopMode(loop)

	info := player.GetInfo()
	totalSamples := int64(info.MusicTimeInMs) * int64(sampleRate) / 1000

	return &YMPlayer{
		player:       player,
		sampleRate:   sampleRate,
		buffer:       make([]int16, 4096),
		totalSamples: totalSamples,
		loop:         loop,
		volume:       0.7,
	}, nil
}

// Read implements io.Reader for audio streaming
func (y *YMPlayer) Read(p []byte) (n int, err error) {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	samplesNeeded := len(p) / 4
	outBuffer := make([]int16, samplesNeeded*2)

	processed := 0
	for processed < samplesNeeded {
		chunkSize := samplesNeeded - processed
		if chunkSize > len(y.buffer) {
			chunkSize = len(y.buffer)
		}

		if !y.player.Compute(y.buffer[:chunkSize], chunkSize) {
			if !y.loop {
				for i := processed * 2; i < len(outBuffer); i++ {
					outBuffer[i] = 0
				}
				err = io.EOF
				break
			}
		}

		for i := 0; i < chunkSize; i++ {
			sample := int16(float64(y.buffer[i]) * y.volume)
			outBuffer[(processed+i)*2] = sample
			outBuffer[(processed+i)*2+1] = sample
		}

		processed += chunkSize
		y.position += int64(chunkSize)
	}

	buf := make([]byte, 0, len(outBuffer)*2)
	for _, sample := range outBuffer {
		buf = append(buf, byte(sample), byte(sample>>8))
	}

	copy(p, buf)
	n = len(buf)
	if n > len(p) {
		n = len(p)
	}

	return n, err
}

// Seek implements io.Seeker
func (y *YMPlayer) Seek(offset int64, whence int) (int64, error) {
	return y.position, nil
}

// Close releases resources
func (y *YMPlayer) Close() error {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	if y.player != nil {
		y.player.Destroy()
		y.player = nil
	}
	return nil
}

// Game represents the TCB demo state
type Game struct {
	// Images
	rasters   *ebiten.Image
	mountains *ebiten.Image
	logo      *ebiten.Image
	font      *ebiten.Image

	// Canvases - following the original structure
	mycanvas     *ebiten.Image
	papercanvas  *ebiten.Image
	papercanvas2 *ebiten.Image
	scrollcanvas *ebiten.Image
	lettercanvas *ebiten.Image
	thecanvas    *ebiten.Image
	thecanvas2   *ebiten.Image

	// Font tiles
	fontTiles map[rune]*ebiten.Image

	// Background parallax
	bgSpeed []float64
	bgPos   []float64

	// Scroll parameters
	scrollForms []ScrollForm
	form        int
	scrollX     float64
	scrollText  string
	addi        int
	sinAdder    float64
	printPos    []PrintPos

	// Logo animation
	logoSin     []float64
	dcounter    int
	rotPos      float64
	rotAdd      float64
	next        int

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player
	ymPlayer     *YMPlayer
}

// NewGame creates and initializes the demo
func NewGame() *Game {
	g := &Game{
		mycanvas:     ebiten.NewImage(screenWidth, screenHeight),
		papercanvas:  ebiten.NewImage(canvasWidth, canvasHeight),
		papercanvas2: ebiten.NewImage(canvasWidth*2, canvasHeight*2),
		scrollcanvas: ebiten.NewImage(canvasWidth, canvasHeight),
		lettercanvas: ebiten.NewImage(32, 32),

		fontTiles: make(map[rune]*ebiten.Image),
		printPos:  make([]PrintPos, 30),

		form:    0,
		addi:    0,
		rotAdd:  1,
		scrollX: 0,
	}

	// Initialize scroll forms (exactly as in JS)
	g.scrollForms = []ScrollForm{
		{0, 0, 0, 0, 55, 0, 0},
		{0, 0, 0, 0, 55, 0, 2},
		{0, 0, 0, 0, 55, 20, 2},
		{200, 0, 0, 5, 55, 20, 2},
		{200, 0, 4, 5, 55, 20, 2},
		{200, -30, 4, 0, 55, 30, 2},
		{200, 40, -4, 5, -70, 40, -4},
		{150, 20, -3, 5, 55, 20, 2},
	}

	// Initialize background speeds (exactly as in JS)
	speeds := []float64{8, 7.5, 7, 6.5, 6, 5.5, 5, 4.5, 4, 3.5, 3, 2.5, 2, 1.5, 1, 0.5}
	g.bgSpeed = make([]float64, 32)
	g.bgPos = make([]float64, 32)

	// Copy speeds forward and backward
	copy(g.bgSpeed[:16], speeds)
	copy(g.bgSpeed[16:], speeds)

	// Initialize logo sine table
	g.initLogoSin()

	// Load assets
	g.loadAssets()

	// Initialize scroll text
	g.initScrollText()

	// Extract logo parts
	if g.logo != nil {
		g.thecanvas = ebiten.NewImage(80, 16)
		g.thecanvas2 = ebiten.NewImage(80, 16)

		// Extract TCB text from logo (79x15 at position 114,0)
		tcbPart := g.logo.SubImage(image.Rect(114, 0, 193, 15)).(*ebiten.Image)

		// Draw to thecanvas (normal)
		op := &ebiten.DrawImageOptions{}
		g.thecanvas.DrawImage(tcbPart, op)

		// Draw to thecanvas2 (flipped vertically)
		op2 := &ebiten.DrawImageOptions{}
		op2.GeoM.Scale(1, -1)
		op2.GeoM.Translate(0, 16)
		g.thecanvas2.DrawImage(tcbPart, op2)
	}

	// Initialize audio
	g.initAudio()

	return g
}

func (g *Game) initLogoSin() {
	g.logoSin = make([]float64, 0)

	// First 40 zeros
	for i := 0; i < 40; i++ {
		g.logoSin = append(g.logoSin, 0)
	}

	// First sine wave
	for i := 0; i < 160*5+4; i++ {
		g.logoSin = append(g.logoSin, 8*math.Sin(float64(i)*0.05-2))
	}

	// Second sine wave
	for i := 0; i < 160*5+10; i++ {
		g.logoSin = append(g.logoSin, 8*math.Sin(float64(i)*0.15))
	}

	// Final 160 zeros
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

func (g *Game) loadAssets() {
	var err error

	// Load rasters
	img, _, err := image.Decode(bytes.NewReader(rastersData))
	if err != nil {
		log.Printf("Error loading rasters: %v", err)
		g.rasters = ebiten.NewImage(320, 200)
		g.rasters.Fill(color.RGBA{255, 0, 255, 255})
	} else {
		g.rasters = ebiten.NewImageFromImage(img)
	}

	// Load mountains
	img, _, err = image.Decode(bytes.NewReader(mountainsData))
	if err != nil {
		log.Printf("Error loading mountains: %v", err)
		g.mountains = ebiten.NewImage(1024, 320)
	} else {
		g.mountains = ebiten.NewImageFromImage(img)
		// Mountains are 1024x320 with 32 layers of 10 pixels height each
		// Set up mountain tiles properly
		g.mountains.SubImage(image.Rect(0, 0, 1024, 320))
	}

	// Load logo
	img, _, err = image.Decode(bytes.NewReader(logoData))
	if err != nil {
		log.Printf("Error loading logo: %v", err)
		g.logo = ebiten.NewImage(320, 48)
	} else {
		g.logo = ebiten.NewImageFromImage(img)
	}

	// Load font
	img, _, err = image.Decode(bytes.NewReader(fontData))
	if err != nil {
		log.Printf("Error loading font: %v", err)
		g.font = ebiten.NewImage(320, 198)
	} else {
		g.font = ebiten.NewImageFromImage(img)
		g.cacheFontTiles()
	}
}

func (g *Game) cacheFontTiles() {
	// Font layout based on your description
	charMap := [][]rune{
		{0, '!', 0, 0, 0, 0, 0, 0, '(', ')'},
		{0, 0, ',', 0, '.', 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, ':', ';', 0, 0},
		{0, 0, 0, 'A', 'B', 'C', 'D', 'E', 'F', 'G'},
		{'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q'},
		{'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', 0},
	}

	// Create font tiles for each character
	for row := 0; row < 6; row++ {
		for col := 0; col < 10; col++ {
			ch := charMap[row][col]
			if ch != 0 {
				x := col * 32
				y := row * 33
				g.fontTiles[ch] = g.font.SubImage(
					image.Rect(x, y, x+32, y+33),
				).(*ebiten.Image)
			}
		}
	}

	// Space is a blank tile
	g.fontTiles[' '] = ebiten.NewImage(32, 33)
}

func (g *Game) initAudio() {
	g.audioContext = audio.NewContext(44100)

	var err error
	g.ymPlayer, err = NewYMPlayer(musicData, 44100, true)
	if err != nil {
		log.Printf("Failed to create YM player: %v", err)
		return
	}

	g.audioPlayer, err = g.audioContext.NewPlayer(g.ymPlayer)
	if err != nil {
		log.Printf("Failed to create audio player: %v", err)
		g.ymPlayer.Close()
		g.ymPlayer = nil
		return
	}

	g.audioPlayer.SetVolume(0.7)
	g.audioPlayer.Play()
}

func (g *Game) Update() error {
	// Handle fullscreen toggle
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

	// Update background parallax (exactly as in JS)
	for i := 0; i < 32; i++ {
		g.bgPos[i] = math.Mod(g.bgPos[i]-g.bgSpeed[i], 256)
	}

	// Update logo distortion counter
	g.dcounter++
	if g.dcounter > len(g.logoSin)-80 {
		g.dcounter = 0
	}

	// Update logo rotation
	g.rotPos += g.rotAdd * 0.08
	if g.rotPos > 1 {
		g.rotPos = -1
		g.next++
		if g.next > 1 {
			g.next = 0
		}
	}

	// Update 3D scroll
	g.scroll3D(4)

	return nil
}

func (g *Game) scroll3D(scrollspeed float64) {
	// Update sine adder for animation
	g.sinAdder += 0.02

	// Clear printPos array
	for i := range g.printPos {
		g.printPos[i] = PrintPos{}
	}

	// Process characters
	printIdx := 0
	for i := 0; i < 30; i++ {
		charIdx := g.addi + i
		// Handle wrapping
		for charIdx >= len(g.scrollText) {
			charIdx -= len(g.scrollText)
		}

		letter := string(g.scrollText[charIdx])

		// Handle control codes
		if letter == "^" && charIdx+1 < len(g.scrollText) {
			nextChar := g.scrollText[(charIdx+1)%len(g.scrollText)]
			if nextChar >= '0' && nextChar <= '7' {
				g.form = int(nextChar - '0')
				letter = string(g.scrollText[(charIdx-1+len(g.scrollText))%len(g.scrollText)])
			}
		}

		// Skip numbers after control codes
		if charIdx > 0 && g.scrollText[(charIdx-1+len(g.scrollText))%len(g.scrollText)] == '^' {
			if g.scrollText[charIdx] >= '0' && g.scrollText[charIdx] <= '7' {
				if charIdx >= 2 {
					letter = string(g.scrollText[(charIdx-2+len(g.scrollText))%len(g.scrollText)])
				}
			}
		}

		// Calculate 3D position using current form
		sf := g.scrollForms[g.form]

		// IMPORTANT: Use charIdx (not i) for the wave calculation to keep it stable
		// This ensures each character keeps its wave position as it scrolls
		letterZ := sf.zSize*math.Sin(sf.zAdd+float64(charIdx)*sf.zAmount*0.01+g.sinAdder*sf.zSpeed) + 150
		letterY := sf.ySize*math.Cos(1.5+float64(charIdx)*sf.yAmount*0.01+g.sinAdder*sf.ySpeed) - 4

		scale := fov / (fov + letterZ)

		// Position calculation with smooth scrolling
		letterX := -450.0 + float64(i)*32 - g.scrollX
		x2d := ((letterX-16)*scale) + float64(g.papercanvas.Bounds().Dx())/2
		y2d := ((letterY-14)*scale) + float64(g.papercanvas.Bounds().Dy())/2

		g.printPos[printIdx].x = x2d
		g.printPos[printIdx].y = y2d
		g.printPos[printIdx].z = scale
		g.printPos[printIdx].letter = letter
		printIdx++
	}

	// Sort by depth (back to front)
	sort.Slice(g.printPos, func(i, j int) bool {
		return g.printPos[i].z < g.printPos[j].z
	})

	// Update scroll position
	g.scrollX += scrollspeed

	// When we've scrolled one character width, advance index
	if g.scrollX >= 32 {
		g.scrollX -= 32
		g.addi++
		if g.addi >= len(g.scrollText) {
			g.addi = 0
		}
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Clear main canvas
	g.mycanvas.Fill(color.Black)
	g.papercanvas.Clear()
	g.papercanvas2.Clear()
	g.scrollcanvas.Clear()

	// Draw parallax mountains
	// In the JS version: mountains.drawTile(papercanvas2,i,(bgpos[i])*2,i*10);
	// The mountains image is 1024 wide, and we draw tiles that are the full width
	for i := 0; i < 16; i++ {
		xPos := int(g.bgPos[i]) * 2
		yPos := i * 10

		// Draw the full width mountain strip for this layer
		srcY := i * 10
		mountainStrip := g.mountains.SubImage(image.Rect(0, srcY, 1024, srcY+10)).(*ebiten.Image)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(xPos), float64(yPos))
		g.papercanvas2.DrawImage(mountainStrip, op)

		// Draw wrapped tile to ensure continuous scrolling
		op.GeoM.Translate(640, 0)
		g.papercanvas2.DrawImage(mountainStrip, op)
	}

	// Draw bottom mountain layers
	for i := 16; i < 32; i++ {
		xPos := int(g.bgPos[i]) * 2
		yPos := i*10 + 84

		srcY := i * 10
		mountainStrip := g.mountains.SubImage(image.Rect(0, srcY, 1024, srcY+10)).(*ebiten.Image)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(xPos), float64(yPos))
		g.papercanvas2.DrawImage(mountainStrip, op)

		op.GeoM.Translate(640, 0)
		g.papercanvas2.DrawImage(mountainStrip, op)
	}

	// Draw papercanvas2 to main canvas
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(64, 60)
	g.mycanvas.DrawImage(g.papercanvas2, op)

	// Draw distorted logo
	for i := 0; i < 32; i++ {
		xOffset := g.logoSin[g.dcounter+i]

		src := g.logo.SubImage(image.Rect(0, 16+i, 303, 17+i)).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(8+xOffset, float64(96+i))
		g.papercanvas.DrawImage(src, op)
	}

	// Draw rotating TCB text
	if g.thecanvas != nil && g.thecanvas2 != nil {
		op = &ebiten.DrawImageOptions{}
		// Center the rotation on the text
		op.GeoM.Translate(-40, -8)
		op.GeoM.Scale(1, g.rotPos)
		op.GeoM.Translate(160, 88)

		if g.next == 0 {
			g.papercanvas.DrawImage(g.thecanvas, op)
		} else {
			g.papercanvas.DrawImage(g.thecanvas2, op)
		}
	}

	// Draw 3D scroll
	g.drawScroll3D()

	// Composite scroll onto paper canvas
	op = &ebiten.DrawImageOptions{}
	g.papercanvas.DrawImage(g.scrollcanvas, op)

	// Draw paper canvas to main canvas (scaled 2x)
	op = &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	op.GeoM.Translate(64, 60)
	g.mycanvas.DrawImage(g.papercanvas, op)

	// Draw to screen
	screen.DrawImage(g.mycanvas, nil)
}

func (g *Game) drawScroll3D() {
	// Don't clear the canvas, it's already cleared in Draw()

	// Draw each character
	for i := 0; i < 30; i++ {
		if g.printPos[i].letter == "" || g.printPos[i].z <= 0 {
			continue
		}

		ch := rune(g.printPos[i].letter[0])
		tile, ok := g.fontTiles[ch]
		if !ok {
			// Try uppercase
			if ch >= 'a' && ch <= 'z' {
				ch = ch - 'a' + 'A'
				tile, ok = g.fontTiles[ch]
			}
			if !ok {
				tile = g.fontTiles[' ']
			}
		}

		if tile != nil {
			op := &ebiten.DrawImageOptions{}
			// Center the character sprite
			op.GeoM.Translate(-16, -16.5)
			op.GeoM.Scale(g.printPos[i].z, g.printPos[i].z)
			op.GeoM.Translate(g.printPos[i].x, g.printPos[i].y)

			// Use nearest neighbor filter for pixel-perfect rendering
			op.Filter = ebiten.FilterNearest

			g.scrollcanvas.DrawImage(tile, op)
		}
	}

	// Apply raster colors
	// The raster image needs to be stretched to cover the full canvas width
	// Then source-atop will apply it only inside the already drawn letters
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(g.scrollcanvas.Bounds().Dx())/float64(g.rasters.Bounds().Dx()), 1)
	op.CompositeMode = ebiten.CompositeModeSourceAtop
	g.scrollcanvas.DrawImage(g.rasters, op)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

// Cleanup releases resources
func (g *Game) Cleanup() {
	if g.audioPlayer != nil {
		g.audioPlayer.Close()
	}
	if g.ymPlayer != nil {
		g.ymPlayer.Close()
	}
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("TCB SUPER-MULTI-PLANE-3D-SCROLLER")

	game := NewGame()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}

	game.Cleanup()
}
