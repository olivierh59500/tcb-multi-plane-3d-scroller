# DCK version

This directory contains the construction-kit version of tcb-multi-plane-3d-scroller. The original Go sources are preserved at their original paths (revision `2fa1b3f6636c7aaa61d44ffa96f0d4366096fa21`), with small asset accessors so both versions use the same embedded resources.

Run the original with `go run ./cmd/tcb-scroller` and this version with `go run ./dck/cmd/tcb-scroller` from the repository root.

The choreography and assets remain in this repository. Reusable rendering and
effects come from the published `github.com/olivierh59500/democonstructionkit`
module pinned in `go.mod`. Music is opened with `sound.Open`; DCK selects the decoder from the asset and
provides the configured stereo PCM format. The demo keeps its playback level and loop settings.

## Shared scrolling effects

The demo now delegates plane motion and rendering to `scrolling.Planes` and `PlaneRenderer`, with the eight original forms from `presets.TCBScrollForms`. The original text, visible-slot controls, phase recurrence and raster colors are preserved.

Each form is independently available as `PlaneForm.Mode` in `scrolling.New`, alongside `Normal`, `Bounce`, `Sine`, `Zoom`, `Perspective` and DNA. Supply any configured font, then choose effects with `{shape:name}` or `ModeSequence`. The standalone example `examples/scrollmodes` in the DCK module demonstrates mixed fonts and both scheduling mechanisms.

See the [DCK effect configuration guide](../../../lib/democonstructionkit/docs/EFFECT_OPTIONS.md) for the shared API and examples.

For silent, deterministic native frames of this version, run:

```sh
go run ./dck/cmd/capture -frames 0,12,13,38,39,240 -out captures/tcb
```

The command captures only the demo canvas and never opens an audio device.
The central logo now uses `sprites.AxisFlip` with an authored saw cycle, source
anchor and mirrored back face. Eleven frames around both face changes and later
motion remain pixel-identical to the preceding DCK renderer.
The mountains now use `composite.Bands` with `TruncatePhaseX` and the shared
`presets.TCBMountainBands` recipe. Thirteen captures through fractional strip
speeds and wrap cycles remain pixel-identical.
The large logo's 32-row warp now uses `composite.ProfileImage`, including its
strict phase reset and 2× viewport mapping. Ten captures at the wave-section
joins and wrap remain pixel-identical; the local quad builder is gone.
