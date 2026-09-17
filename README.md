# TCB Multi-Plane 3D Scroller

A Go/Ebitengine port of The CareBears' “Super-Multi-Plane-3D-Scroller-And-A-Whole-Lot-More Screen” from the 1989 Union Demo.

The demo includes the original 3D bending scrolltext, 32 parallax mountain strips, line-distorted TCB logo, rotating emblem, raster colors, and Mad Max's *Thundercats* YM music.

## Requirements

- Go 1.25 or newer
- macOS, Linux, or Windows for the desktop build
- JDK 17 and Android SDK 36 for the Android build

The project pins Ebitengine 2.9.11 and `ym-player` revision `3f73bdca82e5`.

## Desktop

Run directly from the repository:

```sh
go run ./cmd/tcb-scroller
```

Build an executable:

```sh
go build -o tcb-scroller ./cmd/tcb-scroller
./tcb-scroller
```

Press `F` to toggle fullscreen mode.

## Android / Pixel

The Android project targets API 36, requires API 23 or newer, and currently builds the `arm64-v8a` ABI used by the Pixel 10a. Connect one authorized Android device, then run:

```sh
./scripts/run-android.sh
```

The script:

1. locates the Android SDK and JDK 17;
2. generates `android/app/libs/tcbscroller.aar` with the same Ebitengine 2.9.11 version as the game;
3. builds a debug APK with the checked-in Gradle wrapper;
4. requires exactly one authorized device;
5. installs and launches `com.olivierh59500.tcbscroller/.MainActivity`.

Generated artifacts are:

```text
android/app/libs/tcbscroller.aar
android/app/libs/tcbscroller-sources.jar
android/app/build/outputs/apk/debug/app-debug.apk
```

The activity uses immersive `sensorLandscape` mode and handles pause/resume through Ebitengine's mobile view. The original 768×536 logical canvas is kept intact, so wide displays receive undistorted, centered output.

If the SDK or Java cannot be detected automatically, set:

```sh
export ANDROID_HOME=/path/to/android-sdk
export JAVA_HOME=/path/to/jdk-17
```

## Validation

Run the Go checks:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Run the allocation benchmarks:

```sh
go test -run '^$' \
  -bench 'Benchmark(ScrollCalculation|YMPlayerRead)$' \
  -benchmem
```

Run Android lint after generating the AAR:

```sh
JAVA_HOME=/path/to/jdk-17 \
ANDROID_HOME=/path/to/android-sdk \
./android/gradlew -p android lintDebug
```

### Measured micro-benchmarks

Apple M4 Max results for the original revision and this revision, using the same 4096-sample PCM block and scroll state:

| Operation | Original | Optimized |
|---|---:|---:|
| `YMPlayer.Read` | ~36.2 µs, 40,960 B, 3 allocs | ~11.2 µs, 0 B, 0 allocs |
| Scroll position calculation | ~454 ns, 240 B, 33 allocs | ~104 ns, 0 B, 0 allocs |

These are focused CPU micro-benchmarks, not whole-frame or device battery measurements.

## Implementation notes

- Music synthesis and Ebitengine output both use 48 kHz, matching the Pixel audio path.
- `YMPlayer.Read` writes interleaved 16-bit stereo PCM directly into Ebitengine's buffer and performs no per-read allocation.
- Audio initialization is deferred until the first `Update`, after Android has created its application context.
- Scroll control codes are preprocessed once; the animation loop uses fixed arrays, trigonometric recurrences, and allocation-free insertion sort.
- Mountain strips, logo scanlines, and glyphs are batched with `DrawTriangles` instead of creating sub-images and draw options every frame.
- A small Kage shader applies the raster colors while the glyphs are rendered, removing the old intermediate scroll canvas and compositing pass.
- Unchanged frames are not rebuilt when a display refreshes faster than the game's 60 updates per second.

## Project structure

```text
.
├── game.go                         # shared game and renderer
├── game_test.go                    # audio, shader, and performance tests
├── cmd/tcb-scroller/main.go        # desktop entry point
├── mobile/mobile.go                # Ebitengine mobile bridge
├── android/                        # Android activity and Gradle wrapper
├── scripts/run-android.sh          # AAR → APK → device workflow
└── assets/                         # embedded graphics and YM music
```

All runtime assets use `go:embed`; the application does not depend on its working directory.

## Asset layout

- `rast.png`: 1×200 raster color table
- `mountains.png`: 32 strips of 1024×10 pixels
- `logo.png`: logo and rotating TCB emblem
- `bgfont.png`: 32×33 bitmap glyph grid
- `Thundercats.ym`: embedded YM music

## Credits

- Original demo: The CareBears (TCB)
- Music: Mad Max
- Go port: Olivier Houte / Bilizir, DMA

This port is provided for educational and historical-preservation purposes. The original demo content and music remain the property of their respective creators.
