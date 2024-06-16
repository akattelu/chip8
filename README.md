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
