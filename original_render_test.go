//go:build tcb_original_rendercheck

package tcbscroller

import (
	"fmt"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	capture "github.com/olivierh59500/democonstructionkit/fidelity/ebiten"
)

// Capture the preserved Go implementation with its audio device disabled.
// The DCK capture command can render the same ticks for a full-frame comparison.
func TestMain(m *testing.M) {
	if code := m.Run(); code != 0 {
		os.Exit(code)
	}
	directory := os.Getenv("TCB_ORIGINAL_CAPTURES")
	if directory == "" {
		var err error
		directory, err = os.MkdirTemp("", "tcb-original-captures-")
		if err != nil {
			panic(err)
		}
	}
	frames := []int{0, 1, 39, 40, 41, 60, 240, 600, 843, 844, 845,
		1200, 1653, 1654, 1655, 1734, 1735, 1736, 2400, 4800, 9600}
	var game *Game
	err := capture.Run(capture.Config{Directory: directory, Frames: frames,
		Width: ScreenWidth, Height: ScreenHeight}, func() (ebiten.Game, error) {
		game = NewGame()
		game.audioReady = true
		return game, nil
	})
	if game != nil {
		game.Cleanup()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("TCB original captures:", directory)
}
