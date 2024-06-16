package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/veandco/go-sdl2/sdl"
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

	cpu := NewCPU()
	cpu.LoadROM(romBytes)

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
		for i, pixel := range cpu.Graphics {
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

		instruction := cpu.ReadInstruction()
		if cpu.waitingForKey == nil {
			cpu.Run(instruction)
		}
		if cpu.DelayTimer > 0 || cpu.SoundTimer > 0 {
			if cpu.DelayTimer > 0 {
				cpu.DelayTimer -= 1

			}
			if cpu.SoundTimer > 0 {
				cpu.PlaySound(deviceID)
				cpu.SoundTimer -= 1
			}
			time.Sleep(time.Second / 60)
		}

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				println("Quit")
				running = false
				break
			case *sdl.KeyboardEvent:
				if t.Type == sdl.KEYDOWN {
					cpu.PressKey(int(t.Keysym.Sym))
				}
				if t.Type == sdl.KEYUP {
					cpu.ReleaseKey(int(t.Keysym.Sym))
					if cpu.waitingForKey != nil {
						cpu.FinishWaitingForKey(int(t.Keysym.Sym))
					}
				}
			}
		}
	}
}

type CPU struct {
	Reg            [16]byte
	SoundTimer     byte
	DelayTimer     byte
	IndexRegister  uint16
	ProgramCounter uint16 // only really needs 12 bits (programs are 4096 bytes)
	Memory         Memory
	Graphics       [64 * 32]byte
	Stack          [16]uint16
	sp             byte
	KeyState       [16]bool
	waitingForKey  *byte
}

type Memory [4096]byte

func NewCPU() CPU {
	var registers [16]byte
	return CPU{
		Reg:            registers,
		SoundTimer:     0,
		DelayTimer:     0,
		ProgramCounter: 0x200, // start of most programs
		Memory:         NewMemory(),
	}
}

func (c *CPU) PlaySound(deviceID sdl.AudioDeviceID) {
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

func (c *CPU) FinishWaitingForKey(sdlKey int) {
	if c.waitingForKey != nil {
		loc, ok := KEY_STATE_MAP[sdlKey]
		if ok {
			c.Reg[*c.waitingForKey] = loc
			c.waitingForKey = nil
		}
	}
}

func (c *CPU) PressKey(sdlKey int) {
	loc, ok := KEY_STATE_MAP[sdlKey]
	if ok {
		fmt.Printf("Key Pressed: %x\n", loc)
		c.KeyState[loc] = true
	}
}

func (c *CPU) ReleaseKey(sdlKey int) {
	loc, ok := KEY_STATE_MAP[sdlKey]
	if ok {
		fmt.Printf("Key released: %x\n", loc)
		c.KeyState[loc] = false
	}
}

func (c *CPU) FontLocation(font uint16) uint16 {
	return 5 * font
}

func (c *CPU) LoadROM(romBytes []byte) {
	for i, b := range romBytes {
		c.Memory[0x200+i] = b
	}
}

func (c *CPU) ReadInstruction() uint16 {
	first := c.Memory[c.ProgramCounter]
	second := c.Memory[c.ProgramCounter+1]
	return binary.BigEndian.Uint16([]byte{first, second})
}

func (c *CPU) ClearDisplay() {
	c.Graphics = [64 * 32]byte{}
}

func ToGraphicsPos(x, y int) int {
	return (int(y) * 64) + int(x)
}

func fromGraphicsPos(pos int) (int, int) {
	x_pos := pos % 64
	y_pos := pos / 64
	return x_pos, y_pos
}

func GetWrappedPos(initial, offset int) int {
	x, y := fromGraphicsPos(initial)
	if x+offset > 63 {
		return ToGraphicsPos(x+offset%64, y)
	}
	return ToGraphicsPos(x + offset, y)
}

func (c *CPU) WriteGraphics(x, y byte, sprite []byte) bool {
	startX := int(x) % 64
	startY := int(y) % 32

	for _, spriteByte := range sprite {
		fmt.Printf("Plotting 8 bits starting at (%d, %d)\n", startX, startY)
		pos := ToGraphicsPos(startX, startY)
		c.Graphics[GetWrappedPos(pos, 0)] ^= (spriteByte & 0x80) >> 7
		c.Graphics[GetWrappedPos(pos, 1)] ^= (spriteByte & 0x40) >> 6
		c.Graphics[GetWrappedPos(pos, 2)] ^= (spriteByte & 0x20) >> 5
		c.Graphics[GetWrappedPos(pos, 3)] ^= (spriteByte & 0x10) >> 4
		c.Graphics[GetWrappedPos(pos, 4)] ^= (spriteByte & 0x08) >> 3
		c.Graphics[GetWrappedPos(pos, 5)] ^= (spriteByte & 0x04) >> 2
		c.Graphics[GetWrappedPos(pos, 6)] ^= (spriteByte & 0x02) >> 1
		c.Graphics[GetWrappedPos(pos, 7)] ^= (spriteByte & 0x01) >> 0
		startY = (startY + 1) % 32
	}
	return false
}

func (c *CPU) Run(instr uint16) {
	// just exact codes
	switch instr {
	case 0x00E0:
		c.ClearDisplay()
	case 0x00EE:
		c.ProgramCounter = c.Stack[c.sp]
		c.sp -= 1
	}
	opcode := instr & 0xF000
	fmt.Printf("Running opcode %x (instr %x)\n", opcode, instr)
	switch opcode {
	case 0x1000: // jump
		c.ProgramCounter = instr & 0x0FFF
		return
	case 0x2000: // call
		c.sp += 1
		c.Stack[c.sp] = c.ProgramCounter
		c.ProgramCounter = instr & 0x0FFF
		return
	case 0x3000: // skip if equal
		reg := (instr & 0x0F00) >> 8
		left := c.Reg[reg]
		right := byte(instr & 0x00FF)
		if left == right {
			c.Advance()
		}
	case 0x4000: // skip if not equal
		reg := (instr & 0x0F00) >> 8
		left := c.Reg[reg]
		right := byte(instr & 0x00FF)
		if left != right {
			c.Advance()
		}
	case 0x5000: // skip if registers equal
		left := c.Reg[(instr&0x0F00)>>8]
		right := c.Reg[(instr&0x00F0)>>4]
		if left == right {
			c.Advance()
		}
	case 0x6000: // load into register
		reg := (instr & 0x0F00) >> 8
		c.Reg[reg] = byte(instr & 0x00FF)
	case 0x7000: // add
		reg := (instr & 0x0F00) >> 8
		c.Reg[reg] = c.Reg[reg] + (byte(instr & 0x00FF))
	case 0x8000:
		switch instr & 0x000F {
		case 0x0000: // set left to right
			left := (instr & 0x0F00) >> 8
			right := (instr & 0x00F0) >> 4
			c.Reg[left] = c.Reg[right]
		case 0x0001: // bitwise or
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.Reg[leftReg] = c.Reg[leftReg] | c.Reg[rightReg]
			c.Reg[0xF] = 0
		case 0x0002: // bitwise and
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.Reg[leftReg] = c.Reg[leftReg] & c.Reg[rightReg]
			c.Reg[0xF] = 0
		case 0x0003: // bitwise xor
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.Reg[leftReg] = c.Reg[leftReg] ^ c.Reg[rightReg]
			c.Reg[0xF] = 0
		case 0x0004: // add with carry
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			result16 := uint16(c.Reg[leftReg]) + uint16(c.Reg[rightReg])
			c.Reg[leftReg] = byte(result16 & 0x00FF)
			if result16&0xFF00 > 0 {
				c.Reg[0xF] = 1
			} else {
				c.Reg[0xF] = 0
			}
		case 0x0005: // sub with borrow
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			left := c.Reg[leftReg]
			right := c.Reg[rightReg]
			var overflow byte
			if left >= right {
				overflow = 1
			} else {
				overflow = 0
			}
			c.Reg[leftReg] = (left - right)
			c.Reg[0xF] = overflow
		case 0x0006: // shift right
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.Reg[leftReg] = c.Reg[rightReg]
			var overflow byte
			if c.Reg[leftReg]&0x01 == 0x01 {
				overflow = 1
			} else {
				overflow = 0
			}
			c.Reg[leftReg] >>= 1
			c.Reg[0xF] = overflow
		case 0x0007: // sub with borrow other order
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			left := uint16(c.Reg[leftReg])
			right := uint16(c.Reg[rightReg])
			var overflow byte
			if right >= left {
				overflow = 1
			} else {
				overflow = 0
			}
			c.Reg[leftReg] = byte((right - left) & 0x00FF)
			c.Reg[0xF] = overflow
		case 0x000E:
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.Reg[leftReg] = c.Reg[rightReg]
			var overflow byte
			if c.Reg[leftReg]&0x80 == 0x80 {
				overflow = 1
			} else {
				overflow = 0
			}
			c.Reg[leftReg] *= 2
			c.Reg[0xF] = overflow

		}
	case 0x9000: // skip if registers not equal
		left := c.Reg[(instr&0x0F00)>>8]
		right := c.Reg[(instr&0x00F0)>>4]
		if left != right {
			c.Advance()
		}
	case 0xA000: // set index
		c.IndexRegister = instr & 0x0FFF
	case 0xB000: // increment pc by v0
		c.ProgramCounter = uint16(c.Reg[0]) + instr&0x0FFF
		return
	case 0xC000: // random mask
		randBytes := [2]byte{}
		_, err := rand.Read(randBytes[:])
		if err != nil {
			panic(err)
		}
		rand16 := binary.BigEndian.Uint16(randBytes[:]) & 0x00FF
		c.Reg[(instr&0x0F00)>>8] = byte(rand16 & (instr & 0x00FF))
	case 0xD000: // draw
		left := c.Reg[(instr&0x0F00)>>8]
		right := c.Reg[(instr&0x00F0)>>4]
		numBytes := instr & 0x000F
		memStart := c.IndexRegister
		sprite := c.Memory[memStart : memStart+numBytes]
		if overflow := c.WriteGraphics(left, right, sprite); overflow {
			c.Reg[0xF] = 1
		} else {
			c.Reg[0xF] = 0
		}
	case 0xE000:
		switch byte(instr & 0x00FF) {
		case 0x009E: // Skip if Vx value key press
			reg := (instr & 0x0F00) >> 8
			if c.KeyState[c.Reg[reg]] {
				c.Advance()
			}
		case 0x00A1: // Skip if Vx value key not pressed
			reg := (instr & 0x0F00) >> 8
			if !c.KeyState[c.Reg[reg]] {
				c.Advance()
			}
		}
	case 0xF000:
		switch byte(instr & 0x00FF) {
		case 0x0007: // read delay timer
			reg := byte((instr & 0x0F00) >> 8)
			c.Reg[reg] = c.DelayTimer
		case 0x000A: // wait for key press
			reg := byte((instr & 0x0F00) >> 8)
			c.waitingForKey = &reg
		case 0x0015: // load delay timer
			reg := byte((instr & 0x0F00) >> 8)
			c.DelayTimer = c.Reg[reg]
		case 0x0018: // set sound timer
			reg := byte((instr & 0x0F00) >> 8)
			c.SoundTimer = c.Reg[reg]
		case 0x001E: // I += Vx
			reg := (instr & 0x0F00) >> 8
			c.IndexRegister += uint16(c.Reg[reg]) & 0xFFF
		case 0x0029: // load sprite
			font := instr & 0x0F00
			c.IndexRegister = c.FontLocation(font)
		case 0x0033:
			reg := (instr & 0x0F00) >> 8
			val := c.Reg[reg]
			c.Memory[c.IndexRegister] = val / 100
			c.Memory[c.IndexRegister+1] = (val % 100) / 10
			c.Memory[c.IndexRegister+2] = (val % 100) % 10
		case 0x0055: // dump registers
			untilRegister := (instr & 0x0F00) >> 8
			for i, val := range c.Reg[0 : untilRegister+1] {
				c.Memory[i+int(c.IndexRegister)] = val
			}
			c.IndexRegister = c.IndexRegister + untilRegister + 1
		case 0x0065: // read registers
			untilRegister := (instr & 0x0F00) >> 8
			for i, memVal := range c.Memory[int(c.IndexRegister) : int(c.IndexRegister)+int(untilRegister)+1] {
				c.Reg[i] = memVal
			}
			c.IndexRegister = c.IndexRegister + untilRegister + 1
		}

	}
	c.Advance()
}

func (c *CPU) Advance() {
	c.ProgramCounter += 2
}

func NewMemory() Memory {
	var arr [4096]byte
	var sprites [][]byte = [][]byte{
		// 0
		{0xF0, 0x90, 0x90, 0x90, 0xF0},
		// 1
		{0x20, 0x60, 0x20, 0x20, 0x70},
		// 2
		{0xF0, 0x10, 0xF0, 0x80, 0xF0},
		// 3
		{0xF0, 0x10, 0xF0, 0x10, 0xF0},
		// 4
		{0x90, 0x90, 0xF0, 0x10, 0x10},
		// 5
		{0xF0, 0x80, 0xF0, 0x10, 0xF0},
		// 6
		{0xF0, 0x80, 0xF0, 0x90, 0xF0},
		// 7
		{0xF0, 0x10, 0x20, 0x40, 0x40},
		// 8
		{0xF0, 0x90, 0xF0, 0x90, 0xF0},
		// 9
		{0xF0, 0x90, 0xF0, 0x10, 0xF0},
		// A
		{0xF0, 0x90, 0xF0, 0x90, 0x90},
		// B
		{0xE0, 0x90, 0xE0, 0x90, 0xE0},
		// C
		{0xF0, 0x80, 0x80, 0x80, 0xF0},
		// D
		{0xE0, 0x90, 0x90, 0x90, 0xE0},
		// E
		{0xF0, 0x80, 0xF0, 0x80, 0xF0},
		// F
		{0xF0, 0x80, 0xF0, 0x80, 0x80},
	}
	addr := 0
	for _, sprite := range sprites {
		for _, sprite_byte := range sprite {
			arr[addr] = sprite_byte
			addr++
		}
	}
	return arr
}
