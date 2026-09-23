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
