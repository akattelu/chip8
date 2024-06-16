# CHIP8

A CHIP8 emulator written in go

# Installation

This package depends on sdl, to install follow instructions in https://github.com/veandco/go-sdl2

On MacOS:
```sh
brew install sdl2{,_image,_mixer,_ttf,_gfx} pkg-config
```

```sh
git clone git@github.com:akattelu/chip8.git
cd chip8/
go mod tidy
```

This repository uses https://github.com/Timendus/chip8-test-suite/ as a submodule for test ROMs. Update with:
```sh
git submodule update --init --recursive
```

# Usage

Build and start the emulator
```
go build
./chip8 test-suite-roms/bin/1-chip8-logo.ch8
```

# Screenshots

<img width="1274" alt="image" src="https://github.com/akattelu/chip8/assets/12012201/249a54c1-99f5-43a8-8f07-eb39e5396706">
<img width="1278" alt="image" src="https://github.com/akattelu/chip8/assets/12012201/fbf46b04-586f-4c18-89b5-d96c74d22dc1">
<img width="1267" alt="image" src="https://github.com/akattelu/chip8/assets/12012201/3f820a1f-4ed1-4bb8-a1b7-84cb88e28821">
<img width="1281" alt="image" src="https://github.com/akattelu/chip8/assets/12012201/55cf86c4-b172-4764-951c-b33069c01414">



