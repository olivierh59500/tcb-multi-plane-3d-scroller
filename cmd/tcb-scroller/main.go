package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	tcbscroller "tcb-multi-plane-3d-scroller"
)

func main() {
	ebiten.SetWindowSize(tcbscroller.ScreenWidth, tcbscroller.ScreenHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("TCB SUPER-MULTI-PLANE-3D-SCROLLER")
	ebiten.SetVsyncEnabled(true)
	ebiten.SetScreenClearedEveryFrame(false)

	game := tcbscroller.NewGame()
	defer game.Cleanup()
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
