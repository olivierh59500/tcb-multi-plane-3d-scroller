//go:build android || ios

// Package tcbscrollermobile exposes the game to Ebitengine's native mobile view.
package tcbscrollermobile

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/mobile"
	tcbscroller "tcb-multi-plane-3d-scroller/dck"
)

func init() {
	ebiten.SetScreenClearedEveryFrame(false)
	mobile.SetGame(&game{})
}

// game delays image and audio creation until Android has initialized its
// application context and rendering surface.
type game struct {
	delegate *tcbscroller.Game
}

func (g *game) Update() error {
	if g.delegate == nil {
		g.delegate = tcbscroller.NewGame()
	}
	return g.delegate.Update()
}

func (g *game) Draw(screen *ebiten.Image) {
	if g.delegate != nil {
		g.delegate.Draw(screen)
	}
}

func (g *game) Layout(_, _ int) (int, int) {
	return tcbscroller.ScreenWidth, tcbscroller.ScreenHeight
}

// Dummy ensures gomobile emits bindings for this package.
func Dummy() {}
