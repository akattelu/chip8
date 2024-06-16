package main

import (
	"github.com/veandco/go-sdl2/sdl"
	"math"
	"os"
	"time"
)

const (
	PIXEL_SIZE        = 20
	AUDIO_SAMPLE_RATE = 48000
)

var KEY_STATE_MAP = map[int]byte{
	sdl.K_1: 0x1,
	sdl.K_2: 0x2,
	sdl.K_3: 0x3,
	sdl.K_4: 0xC,
	sdl.K_q: 0x4,
	sdl.K_w: 0x5,
	sdl.K_e: 0x6,
	sdl.K_r: 0xD,
	sdl.K_a: 0x7,
	sdl.K_s: 0x8,
	sdl.K_d: 0x9,
	sdl.K_f: 0xE,
	sdl.K_z: 0xA,
	sdl.K_x: 0x0,
	sdl.K_c: 0xB,
	sdl.K_v: 0xF,
}

func main() {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		panic(err)
	}
	defer sdl.Quit()

	romFile := os.Args[1]

	romBytes, err := os.ReadFile(romFile)
	if err != nil {
		panic(err)
	}

	chip8 := NewCHIP8()
	chip8.LoadROM(romBytes)

	window, err := sdl.CreateWindow("chip8-go", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		64*PIXEL_SIZE, 32*PIXEL_SIZE, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()

	spec := &sdl.AudioSpec{
		Freq:     AUDIO_SAMPLE_RATE,
		Format:   sdl.AUDIO_F32,
		Channels: 1,
		Samples:  4096,
	}
	deviceID, err := sdl.OpenAudioDevice("", false, spec, nil, 0)
	if err != nil {
		panic(err)
	}
	sdl.PauseAudioDevice(deviceID, false)
	defer sdl.CloseAudioDevice(deviceID)

	surface, err := window.GetSurface()
	if err != nil {
		panic(err)
	}
	surface.FillRect(nil, 0)
	fgColor := sdl.Color{R: 0x88, G: 0xC0, B: 0xD0, A: 255}
	bgColor := sdl.Color{R: 0x4c, G: 0x56, B: 0x6a, A: 255}
	running := true
	for running {
		for i, pixel := range chip8.Graphics {
			x, y := fromGraphicsPos(i)

			var colorVal uint32
			if pixel > 0 {
				colorVal = sdl.MapRGBA(surface.Format, fgColor.R, fgColor.G, fgColor.B, fgColor.A)
			} else {
				colorVal = sdl.MapRGBA(surface.Format, bgColor.R, bgColor.G, bgColor.B, bgColor.A)
			}
			rect := sdl.Rect{X: int32(x) * PIXEL_SIZE, Y: int32(y) * PIXEL_SIZE, W: PIXEL_SIZE, H: PIXEL_SIZE}
			surface.FillRect(&rect, colorVal)
		}
		window.UpdateSurface()

		instruction := chip8.ReadInstruction()
		if chip8.WaitingForKey == nil {
			chip8.Run(instruction)
		}
		if chip8.delayTimer > 0 || chip8.soundTimer > 0 {
			if chip8.delayTimer > 0 {
				chip8.delayTimer -= 1

			}
			if chip8.soundTimer > 0 {
				playSound(deviceID)
				chip8.soundTimer -= 1
			}
			time.Sleep(time.Second / 60)
		}

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				running = false
				break
			case *sdl.KeyboardEvent:
				if t.Type == sdl.KEYDOWN {
					chip8.PressKey(int(t.Keysym.Sym))
				}
				if t.Type == sdl.KEYUP {
					chip8.ReleaseKey(int(t.Keysym.Sym))
					if chip8.WaitingForKey != nil {
						chip8.FinishWaitingForKey(int(t.Keysym.Sym))
					}
				}
			}
		}
	}
}

func playSound(deviceID sdl.AudioDeviceID) {
	durationSeconds := 1.0 / 60.0
	samples := int(float64(AUDIO_SAMPLE_RATE) * durationSeconds)
	stream := make([]float32, samples)

	for i := range stream {
		t := float64(i) / float64(AUDIO_SAMPLE_RATE)
		// Generate a sine wave for the beep sound
		stream[i] = float32(math.Sin(2.0 * math.Pi * 440 * t))
	}

	var bytes []byte
	for _, v := range stream {
		data := math.Float32bits(v)
		bytes = append(bytes, byte(data), byte(data>>8), byte(data>>16), byte(data>>24))
	}

	sdl.QueueAudio(deviceID, bytes)
}
