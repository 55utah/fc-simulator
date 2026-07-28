# FC-simulator
> An NES/FC emulator implemented in Go.

### Support Status
Supports mapper 0/1/2/3/4 games, including most common titles such as Adventure Island, Salamander, Contra, Super Mario, etc.

### Audio
Audio support is enabled.

### GUI
Implemented using fyne.io.

### Desktop Usage
> Note: portaudio must be installed first. For macOS, install via: `brew install portaudio`

From source:
`go run main.go /User/xxx/xxx.nes`

Binary:
`./main /User/xxx/xxx.nes`

### Web Version
**In addition to the desktop version, a web version is available for immediate experience:**

- Online Experience: https://55utah.github.io/wasm-nes/index.html
- Open Source Project: https://github.com/55utah/wasm-nes-web

### Desktop Controls
```
System Keys:
Q   Reset Game
-   Zoom Out
=   Zoom In

Controller 1:
W/S/A/D  Up/Down/Left/Right
F/G      Game A/B buttons
R/T      Select/Start

Controller 2:
Arrow Keys Up/Down/Left/Right
J/K       Game A/B buttons
U/I       Select/Start
```

### Demo
<img src="https://user-images.githubusercontent.com/17704150/147229324-08580103-be82-4d53-8538-a989b95bb7df.gif" width="200">
<img src="https://user-images.githubusercontent.com/17704150/147230553-55e57fbc-c0c5-4eb5-9fa1-7bc15af480d8.gif" width="200">

## Star History
[![Star History Chart](https://api.star-history.com/svg?repos=55utah/fc-simulator&type=Date)](https://star-history.com/#55utah/fc-simulator&Date)
