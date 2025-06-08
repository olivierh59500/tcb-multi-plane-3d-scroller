# TCB Multi-Plane 3D Scroller Demo

A faithful Golang + Ebiten port of the classic "Multi Plane 3D Scroller" from TCB (The CareBears) in the Union Demo (in 1989).

![Demo Screenshot](screenshot.png)

## Overview

This demo recreates the iconic TCB "Super-Multi-Plane-3D-Scroller-And-A-Whole-Lot-More Screen" featuring:
- 3D bending scrolltext with multiple wave forms
- 32-layer parallax scrolling mountains
- Distorted TCB logo with sine wave effects
- Rotating TCB text
- YM chiptune music (Thundercats theme by Mad Max)
- Authentic raster color effects

## Features

### 3D Scrolling Text
- 8 different wave forms controlled by `^0` through `^7` control codes in the text
- Real-time 3D transformation with perspective projection
- Depth-based character sorting for proper overlap
- Smooth transitions between wave forms
- Raster gradient colors applied to text

### Visual Effects
- **Parallax Mountains**: 32 independent scrolling layers creating a depth illusion
- **Logo Distortion**: Line-by-line sine wave distortion of the TCB logo
- **Rotating Text**: The "TCB" text rotates around a horizontal axis
- **Color Rasters**: Authentic Atari ST-style color gradients

### Technical Implementation
- Pure Go implementation using Ebiten v2 game engine
- YM music playback via custom YM player
- 60 FPS performance on modern hardware
- Faithful recreation of original demo effects

## Requirements

- Go 1.19 or higher
- Ebiten v2.6.3 or higher

## Installation

```bash
# Clone the repository
git clone https://github.com/olivierh59500/tcb-multi-plane-3d-scroller
cd tcb-multi-plane-3d-scroller

# Install dependencies
go mod download

# Build and run
go run main.go
```

## Project Structure

```
tcb-multi-plane-3d-scroller/
├── main.go             # Main demo implementation
├── go.mod              # Go module definition
├── go.sum              # Dependency checksums
├── README.md           # This file
└── assets/             # Demo assets
    ├── rast.png        # Raster gradient colors (320x200)
    ├── mountains.png   # Parallax mountain layers (1024x320)
    ├── logo.png        # TCB logo graphics (320x48)
    ├── bgfont.png      # Bitmap font (320x198, 32x33 per character)
    └── Thundercats.ym  # YM music file
```

## Asset Details

### Font Layout
The bitmap font (`bgfont.png`) contains characters arranged in a 10x6 grid:
- Each character is 32x33 pixels
- Characters include: A-Z, space, and punctuation (! ( ) , . : ;)
- Font uses white pixels on transparent background

### Mountain Layers
The `mountains.png` file contains 32 horizontal strips:
- Each strip is 1024x10 pixels
- Different shades create depth perception
- Strips scroll at different speeds for parallax effect

### Logo Structure
The `logo.png` contains:
- Full logo graphic (303x48 pixels)
- TCB text portion at position (114,0) with size 79x15
- Used for both distortion effect and rotating text

## Technical Notes

### Optimization Strategies
- Pre-calculated character positions
- Efficient depth sorting algorithm
- Reused DrawImageOptions to minimize allocations
- Proper canvas clearing to avoid overdraw

### Wave Forms
The demo includes 8 different scroll wave effects:
1. **Form 0**: No wave (flat scrolling)
2. **Form 1**: Slow sine wave
3. **Form 2**: Medium sine wave
4. **Form 3**: Fast sine wave
5. **Form 4**: Slow distortion
6. **Form 5**: Medium distortion
7. **Form 6**: Fast distortion
8. **Form 7**: Split wave effect

### Coordinate System
- Screen resolution: 768x536
- ST canvas: 320x200 (scaled 2x)
- Character spacing: 32 pixels
- Base scroll speed: 4 pixels per frame

## Original Credits

- **Original Demo**: The CareBears (TCB)
- **Music**: Mad Max (Thundercats theme)
- **Golang Port**: Olivier Houte aka Bilizir from DMA

## Building from Source

### Standard Build
```bash
go build -o tcb-demo main.go
./tcb-demo
```

### Optimized Build
```bash
go build -ldflags="-s -w" -o tcb-demo main.go
```

### Cross-Platform Building
```bash
# Windows
GOOS=windows GOARCH=amd64 go build -o tcb-demo.exe main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o tcb-demo-mac main.go

# Linux
GOOS=linux GOARCH=amd64 go build -o tcb-demo-linux main.go
```

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs and feature requests.

## License

This port is provided for educational and historical preservation purposes. The original demo content and music remain the property of their respective creators (The CareBears and Mad Max).

## See Also

- [CODEF Framework](http://codef.namwollem.co.uk/) - The original web framework
- [Ebiten Game Engine](https://ebiten.org/) - The Go game engine used for this port
- [Demozoo Entry](https://demozoo.org/) - Original demo information

## Acknowledgments

Special thanks to:
- The CareBears for creating this iconic demo
- The Atari ST demoscene community
- The Ebiten development team
- Everyone working to preserve demoscene history