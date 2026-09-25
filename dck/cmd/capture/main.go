// Command capture writes deterministic native frames of the DCK production.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	capture "github.com/olivierh59500/democonstructionkit/fidelity/ebiten"
	tcbscroller "tcb-multi-plane-3d-scroller/dck"
)

func main() {
	framesFlag := flag.String("frames", "0,60,240,600", "comma-separated capture ticks")
	out := flag.String("out", "captures/native", "capture directory")
	flag.Parse()
	var frames []int
	for _, part := range strings.Split(*framesFlag, ",") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || len(frames) > 0 && n <= frames[len(frames)-1] {
			fmt.Fprintln(os.Stderr, "frames must be increasing nonnegative integers")
			os.Exit(1)
		}
		frames = append(frames, n)
	}
	var game *tcbscroller.Game
	err := capture.Run(capture.Config{Directory: *out, Frames: frames, Width: tcbscroller.ScreenWidth, Height: tcbscroller.ScreenHeight}, func() (ebiten.Game, error) {
		game = tcbscroller.NewSilentGame()
		return game, nil
	})
	if game != nil {
		game.Cleanup()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
