package main

import (
	"crypto/rand"
	"encoding/binary"
)

type CHIP8 struct {
	registers      [16]byte
	soundTimer     byte
	delayTimer     byte
	indexRegister  uint16
	programCounter uint16 // only really needs 12 bits (programs are 4096 bytes)
	memory         [4096]byte
	stack          [16]uint16
	sp             byte

	Graphics      [64 * 32]byte
	KeyState      [16]bool
	WaitingForKey *byte
}

func NewCHIP8() CHIP8 {
	var registers [16]byte
	return CHIP8{
		registers:      registers,
		programCounter: 0x200, // start of most programs
		memory:         NewMemory(),
	}
}

func (c *CHIP8) FinishWaitingForKey(sdlKey int) {
	if c.WaitingForKey != nil {
		loc, ok := KEY_STATE_MAP[sdlKey]
		if ok {
			c.registers[*c.WaitingForKey] = loc
			c.WaitingForKey = nil
		}
	}
}

func (c *CHIP8) PressKey(sdlKey int) {
	loc, ok := KEY_STATE_MAP[sdlKey]
	if ok {
		c.KeyState[loc] = true
	}
}

func (c *CHIP8) ReleaseKey(sdlKey int) {
	loc, ok := KEY_STATE_MAP[sdlKey]
	if ok {
		c.KeyState[loc] = false
	}
}

func (c *CHIP8) LoadROM(romBytes []byte) {
	for i, b := range romBytes {
		c.memory[0x200+i] = b
	}
}

func (c *CHIP8) ReadInstruction() uint16 {
	first := c.memory[c.programCounter]
	second := c.memory[c.programCounter+1]
	return binary.BigEndian.Uint16([]byte{first, second})
}

func (c *CHIP8) Run(instr uint16) {
	// just exact codes
	switch instr {
	case 0x00E0:
		c.clearDisplay()
	case 0x00EE:
		c.programCounter = c.stack[c.sp]
		c.sp -= 1
	}
	opcode := instr & 0xF000
	switch opcode {
	case 0x1000: // jump
		c.programCounter = instr & 0x0FFF
		return
	case 0x2000: // call
		c.sp += 1
		c.stack[c.sp] = c.programCounter
		c.programCounter = instr & 0x0FFF
		return
	case 0x3000: // skip if equal
		reg := (instr & 0x0F00) >> 8
		left := c.registers[reg]
		right := byte(instr & 0x00FF)
		if left == right {
			c.advance()
		}
	case 0x4000: // skip if not equal
		reg := (instr & 0x0F00) >> 8
		left := c.registers[reg]
		right := byte(instr & 0x00FF)
		if left != right {
			c.advance()
		}
	case 0x5000: // skip if registers equal
		left := c.registers[(instr&0x0F00)>>8]
		right := c.registers[(instr&0x00F0)>>4]
		if left == right {
			c.advance()
		}
	case 0x6000: // load into register
		reg := (instr & 0x0F00) >> 8
		c.registers[reg] = byte(instr & 0x00FF)
	case 0x7000: // add
		reg := (instr & 0x0F00) >> 8
		c.registers[reg] = c.registers[reg] + (byte(instr & 0x00FF))
	case 0x8000:
		switch instr & 0x000F {
		case 0x0000: // set left to right
			left := (instr & 0x0F00) >> 8
			right := (instr & 0x00F0) >> 4
			c.registers[left] = c.registers[right]
		case 0x0001: // bitwise or
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.registers[leftReg] = c.registers[leftReg] | c.registers[rightReg]
			c.registers[0xF] = 0
		case 0x0002: // bitwise and
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.registers[leftReg] = c.registers[leftReg] & c.registers[rightReg]
			c.registers[0xF] = 0
		case 0x0003: // bitwise xor
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.registers[leftReg] = c.registers[leftReg] ^ c.registers[rightReg]
			c.registers[0xF] = 0
		case 0x0004: // add with carry
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			result16 := uint16(c.registers[leftReg]) + uint16(c.registers[rightReg])
			c.registers[leftReg] = byte(result16 & 0x00FF)
			if result16&0xFF00 > 0 {
				c.registers[0xF] = 1
			} else {
				c.registers[0xF] = 0
			}
		case 0x0005: // sub with borrow
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			left := c.registers[leftReg]
			right := c.registers[rightReg]
			var overflow byte
			if left >= right {
				overflow = 1
			} else {
				overflow = 0
			}
			c.registers[leftReg] = (left - right)
			c.registers[0xF] = overflow
		case 0x0006: // shift right
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.registers[leftReg] = c.registers[rightReg]
			var overflow byte
			if c.registers[leftReg]&0x01 == 0x01 {
				overflow = 1
			} else {
				overflow = 0
			}
			c.registers[leftReg] >>= 1
			c.registers[0xF] = overflow
		case 0x0007: // sub with borrow other order
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			left := uint16(c.registers[leftReg])
			right := uint16(c.registers[rightReg])
			var overflow byte
			if right >= left {
				overflow = 1
			} else {
				overflow = 0
			}
			c.registers[leftReg] = byte((right - left) & 0x00FF)
			c.registers[0xF] = overflow
		case 0x000E:
			leftReg := instr & 0x0F00 >> 8
			rightReg := instr & 0x00F0 >> 4
			c.registers[leftReg] = c.registers[rightReg]
			var overflow byte
			if c.registers[leftReg]&0x80 == 0x80 {
				overflow = 1
			} else {
				overflow = 0
			}
			c.registers[leftReg] *= 2
			c.registers[0xF] = overflow

		}
	case 0x9000: // skip if registers not equal
		left := c.registers[(instr&0x0F00)>>8]
		right := c.registers[(instr&0x00F0)>>4]
		if left != right {
			c.advance()
		}
	case 0xA000: // set index
		c.indexRegister = instr & 0x0FFF
	case 0xB000: // increment pc by v0
		c.programCounter = uint16(c.registers[0]) + instr&0x0FFF
		return
	case 0xC000: // random mask
		randBytes := [2]byte{}
		_, err := rand.Read(randBytes[:])
		if err != nil {
			panic(err)
		}
		rand16 := binary.BigEndian.Uint16(randBytes[:]) & 0x00FF
		c.registers[(instr&0x0F00)>>8] = byte(rand16 & (instr & 0x00FF))
	case 0xD000: // draw
		left := c.registers[(instr&0x0F00)>>8]
		right := c.registers[(instr&0x00F0)>>4]
		numBytes := instr & 0x000F
		memStart := c.indexRegister
		sprite := c.memory[memStart : memStart+numBytes]
		if overflow := c.writeGraphics(left, right, sprite); overflow {
			c.registers[0xF] = 1
		} else {
			c.registers[0xF] = 0
		}
	case 0xE000:
		switch byte(instr & 0x00FF) {
		case 0x009E: // Skip if Vx value key press
			reg := (instr & 0x0F00) >> 8
			if c.KeyState[c.registers[reg]] {
				c.advance()
			}
		case 0x00A1: // Skip if Vx value key not pressed
			reg := (instr & 0x0F00) >> 8
			if !c.KeyState[c.registers[reg]] {
				c.advance()
			}
		}
	case 0xF000:
		switch byte(instr & 0x00FF) {
		case 0x0007: // read delay timer
			reg := byte((instr & 0x0F00) >> 8)
			c.registers[reg] = c.delayTimer
		case 0x000A: // wait for key press
			reg := byte((instr & 0x0F00) >> 8)
			c.WaitingForKey = &reg
		case 0x0015: // load delay timer
			reg := byte((instr & 0x0F00) >> 8)
			c.delayTimer = c.registers[reg]
		case 0x0018: // set sound timer
			reg := byte((instr & 0x0F00) >> 8)
			c.soundTimer = c.registers[reg]
		case 0x001E: // I += Vx
			reg := (instr & 0x0F00) >> 8
			c.indexRegister += uint16(c.registers[reg]) & 0xFFF
		case 0x0029: // load sprite
			font := instr & 0x0F00
			c.indexRegister = c.fontLocation(font)
		case 0x0033:
			reg := (instr & 0x0F00) >> 8
			val := c.registers[reg]
			c.memory[c.indexRegister] = val / 100
			c.memory[c.indexRegister+1] = (val % 100) / 10
			c.memory[c.indexRegister+2] = (val % 100) % 10
		case 0x0055: // dump registers
			untilRegister := (instr & 0x0F00) >> 8
			for i, val := range c.registers[0 : untilRegister+1] {
				c.memory[i+int(c.indexRegister)] = val
			}
			c.indexRegister = c.indexRegister + untilRegister + 1
		case 0x0065: // read registers
			untilRegister := (instr & 0x0F00) >> 8
			for i, memVal := range c.memory[int(c.indexRegister) : int(c.indexRegister)+int(untilRegister)+1] {
				c.registers[i] = memVal
			}
			c.indexRegister = c.indexRegister + untilRegister + 1
		}

	}
	c.advance()
}

func (c *CHIP8) advance() {
	c.programCounter += 2
}

func (c *CHIP8) fontLocation(font uint16) uint16 {
	return 5 * font
}

func (c *CHIP8) clearDisplay() {
	c.Graphics = [64 * 32]byte{}
}

func (c *CHIP8) writeGraphics(x, y byte, sprite []byte) bool {
	startX := int(x) % 64
	startY := int(y) % 32

	for _, spriteByte := range sprite {
		pos := toGraphicsPos(startX, startY)
		c.Graphics[getWrappedPos(pos, 0)] ^= (spriteByte & 0x80) >> 7
		c.Graphics[getWrappedPos(pos, 1)] ^= (spriteByte & 0x40) >> 6
		c.Graphics[getWrappedPos(pos, 2)] ^= (spriteByte & 0x20) >> 5
		c.Graphics[getWrappedPos(pos, 3)] ^= (spriteByte & 0x10) >> 4
		c.Graphics[getWrappedPos(pos, 4)] ^= (spriteByte & 0x08) >> 3
		c.Graphics[getWrappedPos(pos, 5)] ^= (spriteByte & 0x04) >> 2
		c.Graphics[getWrappedPos(pos, 6)] ^= (spriteByte & 0x02) >> 1
		c.Graphics[getWrappedPos(pos, 7)] ^= (spriteByte & 0x01) >> 0
		startY = (startY + 1) % 32
	}
	return false
}

func toGraphicsPos(x, y int) int {
	return (int(y) * 64) + int(x)
}

func fromGraphicsPos(pos int) (int, int) {
	x_pos := pos % 64
	y_pos := pos / 64
	return x_pos, y_pos
}

func getWrappedPos(initial, offset int) int {
	x, y := fromGraphicsPos(initial)
	if x+offset > 63 {
		return toGraphicsPos(x+offset%64, y)
	}
	return toGraphicsPos(x+offset, y)
}

func NewMemory() [4096]byte {
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
