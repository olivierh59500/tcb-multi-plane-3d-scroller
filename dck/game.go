package tcbscroller

import (
	"bytes"
	"image"
	"image/color"
	originalassets "tcb-multi-plane-3d-scroller"

	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sound"
	"github.com/olivierh59500/democonstructionkit/sprites"

	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	audio "github.com/olivierh59500/democonstructionkit/sound/output"
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

// Game contains the complete standalone TCB screen.
type Game struct {
	scroll    *scrolling.Scrolling
	rasters   *ebiten.Image
	mountains *ebiten.Image
	logo      *ebiten.Image
	font      *ebiten.Image

	logoCenter *ebiten.Image
	initErr    error

	mountainBands *composite.Bands
	logoRows      *composite.ProfileImage

	scrollText string

	centerFlip *sprites.AxisFlip

	audioReady   bool
	audioContext *audio.Context
	audioPlayer  *audio.Player
	musicStream  *sound.Stream
	needsRedraw  bool
}

func NewGame() *Game {
	g := &Game{needsRedraw: true}

	var err error
	g.mountainBands, err = composite.NewBands(presets.TCBMountainBands())
	if err != nil {
		g.initErr = err
		return g
	}

	g.initScrollText()
	g.loadAssets()
	if g.initErr != nil {
		return g
	}
	profile, err := motion.CompileWaveTable(presets.TCBLogoWaveSections()...)
	if err != nil {
		g.initErr = err
		return g
	}
	logoConfig := presets.TCBLogoRowProfile(profile, 303)
	logoConfig.ScaleX, logoConfig.ScaleY = 2, 2
	logoConfig.OutputX, logoConfig.OutputY = stageX, stageY
	g.logoRows, err = composite.NewProfileImage(g.logo.SubImage(image.Rect(0, 16, 303, 48)).(*ebiten.Image), logoConfig)
	if err != nil {
		g.initErr = err
		return g
	}
	g.centerFlip, err = sprites.NewAxisFlip(sprites.AxisFlipConfig{
		Front: g.logoCenter, Saw: &motion.SawToggleConfig{Start: 0, Velocity: .08, Boundary: 1, Restart: -1},
		UseAnchor: true, AnchorX: 40, AnchorY: 8, BackMirrorY: true, BackMirrorShift: 16,
		Filter: ebiten.FilterNearest, Blend: ebiten.BlendSourceOver,
	})
	if err != nil {
		g.initErr = err
	}

	return g
}

// NewSilentGame creates the same visual scene without starting a device audio
// stream, so deterministic frame capture can run without affecting playback.
func NewSilentGame() *Game {
	game := NewGame()
	game.audioReady = true
	return game
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
	config := presets.TCBProjectedScroll(g.scrollText, 32, scrolling.Face{Atlas: g.font, Metrics: metrics}, g.rasters)
	config.Projected.Draw = scrolling.PlaneDraw{OriginX: stageX, OriginY: stageY, ScaleX: 2, ScaleY: 2}
	g.scroll, err = scrolling.New(config)
	if err != nil {
		g.initErr = err
	}
}

func (g *Game) initAudio() {
	g.audioContext = audio.NewContext(sampleRate)

	var err error
	g.musicStream, err = sound.Open("music.ym", musicData, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		log.Printf("open music: %v", err)
		return
	}

	g.audioPlayer, err = g.audioContext.NewPlayer(g.musicStream)
	if err != nil {
		log.Printf("create audio player: %v", err)
		_ = g.musicStream.Close()
		g.musicStream = nil
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

	g.mountainBands.Step()

	g.logoRows.Advance()

	g.centerFlip.Step()

	if g.scroll != nil {
		g.initErr = g.scroll.Update(kit.Frame{})
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if !g.needsRedraw || g.initErr != nil {
		return
	}
	g.needsRedraw = false

	screen.Fill(color.Black)
	stage := screen.SubImage(image.Rect(stageX, stageY, stageX+640, stageY+400)).(*ebiten.Image)

	g.mountainBands.DrawAt(stage, g.mountains, stageX, stageY)

	g.logoRows.Draw(screen)

	parent := ebiten.GeoM{}
	parent.Scale(2, 2)
	parent.Translate(stageX, stageY)
	g.centerFlip.DrawAtWith(screen, 160, 88, parent)

	g.drawScroll3D(stage)
}

func (g *Game) drawScroll3D(stage *ebiten.Image) {
	if g.scroll != nil {
		g.scroll.Draw(stage)
	}
}

func (g *Game) Layout(_, _ int) (int, int) {
	return ScreenWidth, ScreenHeight
}

// Cleanup releases the audio resources owned by the game.
func (g *Game) Cleanup() {
	if g.scroll != nil {
		g.scroll.Close()
	}
	if g.logoRows != nil {
		_ = g.logoRows.Close()
	}
	if g.audioPlayer != nil {
		_ = g.audioPlayer.Close()
		g.audioPlayer = nil
	}
	if g.musicStream != nil {
		_ = g.musicStream.Close()
		g.musicStream = nil
	}
}

func stepSinCosForward(sinValue, cosValue, sinStep, cosStep float64) (float64, float64) {
	return sinValue*cosStep + cosValue*sinStep, cosValue*cosStep - sinValue*sinStep
}
