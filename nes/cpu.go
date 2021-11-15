package nes

import (
	"fmt"
)

/*
CPU模块，对外需要以下接口：
clock
reset
irq
nmi
还需要一个NewCPU方法
*/

// 各中断的地址信息，2byte
const (
	// NMI中断
	NMI = 0xfffa
	// 每次启动触发
	RESET = 0xfffc
	// IRQ/BRK共用中断地址
	// 硬件/apu触发
	IRQ = 0xfffe
	// 软件触发
	BRK = 0xfffe
)

const CPUFrequency = 1789773

const (
	_ = iota
	interruptNone
	interruptNMI
	interruptIRQ
)

// 寻址方式
const (
	_ = iota
	modeAbsolute
	modeAbsoluteX
	modeAbsoluteY
	modeAccumulator
	modeImmediate
	modeImplied
	modeIndexedIndirect
	modeIndirect
	modeIndirectIndexed
	modeRelative
	modeZeroPage
	modeZeroPageX
	modeZeroPageY
)

// 寻址方式
var instructionModes = [256]byte{
	6, 7, 6, 7, 11, 11, 11, 11, 6, 5, 4, 5, 1, 1, 1, 1,
	10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
	1, 7, 6, 7, 11, 11, 11, 11, 6, 5, 4, 5, 1, 1, 1, 1,
	10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
	6, 7, 6, 7, 11, 11, 11, 11, 6, 5, 4, 5, 1, 1, 1, 1,
	10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
	6, 7, 6, 7, 11, 11, 11, 11, 6, 5, 4, 5, 8, 1, 1, 1,
	10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
	5, 7, 5, 7, 11, 11, 11, 11, 6, 5, 6, 5, 1, 1, 1, 1,
	10, 9, 6, 9, 12, 12, 13, 13, 6, 3, 6, 3, 2, 2, 3, 3,
	5, 7, 5, 7, 11, 11, 11, 11, 6, 5, 6, 5, 1, 1, 1, 1,
	10, 9, 6, 9, 12, 12, 13, 13, 6, 3, 6, 3, 2, 2, 3, 3,
	5, 7, 5, 7, 11, 11, 11, 11, 6, 5, 6, 5, 1, 1, 1, 1,
	10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
	5, 7, 5, 7, 11, 11, 11, 11, 6, 5, 6, 5, 1, 1, 1, 1,
	10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
}

// 每个指令字节大小
var instructionSizes = [256]byte{
	2, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
	3, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
	1, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
	1, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 0, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 0, 3, 0, 0,
	2, 2, 2, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
	2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
}

// 指令占用基础周期数，不包括额外的周期
var instructionCycles = [256]byte{
	7, 6, 2, 8, 3, 3, 5, 5, 3, 2, 2, 2, 4, 4, 6, 6,
	2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
	6, 6, 2, 8, 3, 3, 5, 5, 4, 2, 2, 2, 4, 4, 6, 6,
	2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
	6, 6, 2, 8, 3, 3, 5, 5, 3, 2, 2, 2, 3, 4, 6, 6,
	2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
	6, 6, 2, 8, 3, 3, 5, 5, 4, 2, 2, 2, 5, 4, 6, 6,
	2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
	2, 6, 2, 6, 3, 3, 3, 3, 2, 2, 2, 2, 4, 4, 4, 4,
	2, 6, 2, 6, 4, 4, 4, 4, 2, 5, 2, 5, 5, 5, 5, 5,
	2, 6, 2, 6, 3, 3, 3, 3, 2, 2, 2, 2, 4, 4, 4, 4,
	2, 5, 2, 5, 4, 4, 4, 4, 2, 4, 2, 4, 4, 4, 4, 4,
	2, 6, 2, 8, 3, 3, 5, 5, 2, 2, 2, 2, 4, 4, 6, 6,
	2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
	2, 6, 2, 8, 3, 3, 5, 5, 2, 2, 2, 2, 4, 4, 6, 6,
	2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
}

// 指令是否跨page
var instructionPageCycles = [256]byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 1, 1, 1, 1, 1,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
}

// 指令名
var instructionNames = [256]string{
	"BRK", "ORA", "KIL", "SLO", "NOP", "ORA", "ASL", "SLO",
	"PHP", "ORA", "ASL", "ANC", "NOP", "ORA", "ASL", "SLO",
	"BPL", "ORA", "KIL", "SLO", "NOP", "ORA", "ASL", "SLO",
	"CLC", "ORA", "NOP", "SLO", "NOP", "ORA", "ASL", "SLO",
	"JSR", "AND", "KIL", "RLA", "BIT", "AND", "ROL", "RLA",
	"PLP", "AND", "ROL", "ANC", "BIT", "AND", "ROL", "RLA",
	"BMI", "AND", "KIL", "RLA", "NOP", "AND", "ROL", "RLA",
	"SEC", "AND", "NOP", "RLA", "NOP", "AND", "ROL", "RLA",
	"RTI", "EOR", "KIL", "SRE", "NOP", "EOR", "LSR", "SRE",
	"PHA", "EOR", "LSR", "ALR", "JMP", "EOR", "LSR", "SRE",
	"BVC", "EOR", "KIL", "SRE", "NOP", "EOR", "LSR", "SRE",
	"CLI", "EOR", "NOP", "SRE", "NOP", "EOR", "LSR", "SRE",
	"RTS", "ADC", "KIL", "RRA", "NOP", "ADC", "ROR", "RRA",
	"PLA", "ADC", "ROR", "ARR", "JMP", "ADC", "ROR", "RRA",
	"BVS", "ADC", "KIL", "RRA", "NOP", "ADC", "ROR", "RRA",
	"SEI", "ADC", "NOP", "RRA", "NOP", "ADC", "ROR", "RRA",
	"NOP", "STA", "NOP", "SAX", "STY", "STA", "STX", "SAX",
	"DEY", "NOP", "TXA", "XAA", "STY", "STA", "STX", "SAX",
	"BCC", "STA", "KIL", "AHX", "STY", "STA", "STX", "SAX",
	"TYA", "STA", "TXS", "TAS", "SHY", "STA", "SHX", "AHX",
	"LDY", "LDA", "LDX", "LAX", "LDY", "LDA", "LDX", "LAX",
	"TAY", "LDA", "TAX", "LAX", "LDY", "LDA", "LDX", "LAX",
	"BCS", "LDA", "KIL", "LAX", "LDY", "LDA", "LDX", "LAX",
	"CLV", "LDA", "TSX", "LAS", "LDY", "LDA", "LDX", "LAX",
	"CPY", "CMP", "NOP", "DCP", "CPY", "CMP", "DEC", "DCP",
	"INY", "CMP", "DEX", "AXS", "CPY", "CMP", "DEC", "DCP",
	"BNE", "CMP", "KIL", "DCP", "NOP", "CMP", "DEC", "DCP",
	"CLD", "CMP", "NOP", "DCP", "NOP", "CMP", "DEC", "DCP",
	"CPX", "SBC", "NOP", "ISC", "CPX", "SBC", "INC", "ISC",
	"INX", "SBC", "NOP", "SBC", "CPX", "SBC", "INC", "ISC",
	"BEQ", "SBC", "KIL", "ISC", "NOP", "SBC", "INC", "ISC",
	"SED", "SBC", "NOP", "ISC", "NOP", "SBC", "INC", "ISC",
}

/*
	256个待实现的指令
*/
func (c *CPU) createTable() {
	c.table = [256]func(*stepInfo){
		c.brk, c.ora, c.kil, c.slo, c.nop, c.ora, c.asl, c.slo,
		c.php, c.ora, c.asl, c.anc, c.nop, c.ora, c.asl, c.slo,
		c.bpl, c.ora, c.kil, c.slo, c.nop, c.ora, c.asl, c.slo,
		c.clc, c.ora, c.nop, c.slo, c.nop, c.ora, c.asl, c.slo,
		c.jsr, c.and, c.kil, c.rla, c.bit, c.and, c.rol, c.rla,
		c.plp, c.and, c.rol, c.anc, c.bit, c.and, c.rol, c.rla,
		c.bmi, c.and, c.kil, c.rla, c.nop, c.and, c.rol, c.rla,
		c.sec, c.and, c.nop, c.rla, c.nop, c.and, c.rol, c.rla,
		c.rti, c.eor, c.kil, c.sre, c.nop, c.eor, c.lsr, c.sre,
		c.pha, c.eor, c.lsr, c.alr, c.jmp, c.eor, c.lsr, c.sre,
		c.bvc, c.eor, c.kil, c.sre, c.nop, c.eor, c.lsr, c.sre,
		c.cli, c.eor, c.nop, c.sre, c.nop, c.eor, c.lsr, c.sre,
		c.rts, c.adc, c.kil, c.rra, c.nop, c.adc, c.ror, c.rra,
		c.pla, c.adc, c.ror, c.arr, c.jmp, c.adc, c.ror, c.rra,
		c.bvs, c.adc, c.kil, c.rra, c.nop, c.adc, c.ror, c.rra,
		c.sei, c.adc, c.nop, c.rra, c.nop, c.adc, c.ror, c.rra,
		c.nop, c.sta, c.nop, c.sax, c.sty, c.sta, c.stx, c.sax,
		c.dey, c.nop, c.txa, c.xaa, c.sty, c.sta, c.stx, c.sax,
		c.bcc, c.sta, c.kil, c.ahx, c.sty, c.sta, c.stx, c.sax,
		c.tya, c.sta, c.txs, c.tas, c.shy, c.sta, c.shx, c.ahx,
		c.ldy, c.lda, c.ldx, c.lax, c.ldy, c.lda, c.ldx, c.lax,
		c.tay, c.lda, c.tax, c.lax, c.ldy, c.lda, c.ldx, c.lax,
		c.bcs, c.lda, c.kil, c.lax, c.ldy, c.lda, c.ldx, c.lax,
		c.clv, c.lda, c.tsx, c.las, c.ldy, c.lda, c.ldx, c.lax,
		c.cpy, c.cmp, c.nop, c.dcp, c.cpy, c.cmp, c.dec, c.dcp,
		c.iny, c.cmp, c.dex, c.axs, c.cpy, c.cmp, c.dec, c.dcp,
		c.bne, c.cmp, c.kil, c.dcp, c.nop, c.cmp, c.dec, c.dcp,
		c.cld, c.cmp, c.nop, c.dcp, c.nop, c.cmp, c.dec, c.dcp,
		c.cpx, c.sbc, c.nop, c.isc, c.cpx, c.sbc, c.inc, c.isc,
		c.inx, c.sbc, c.nop, c.sbc, c.cpx, c.sbc, c.inc, c.isc,
		c.beq, c.sbc, c.kil, c.isc, c.nop, c.sbc, c.inc, c.isc,
		c.sed, c.sbc, c.nop, c.isc, c.nop, c.sbc, c.inc, c.isc,
	}
}

func NewCPU(console *Console) *CPU {
	cpu := CPU{Memory: NewCPUMemory(console)}
	cpu.createTable()
	cpu.Reset()
	return &cpu
}

type CPU struct {
	Memory
	Cycles    uint64
	PC        uint16
	SP        byte // 堆栈寄存器
	A         byte
	X         byte
	Y         byte
	C         byte // 8个状态FLAG C - 进位标志
	Z         byte // Z - 结果为零标志
	I         byte // I - 中断屏蔽
	D         byte // D - 未使用
	B         byte // BRK
	U         byte // 未使用
	V         byte // 溢出标志，计算结果产生溢出
	N         byte // 负标志，结果为负
	interrupt byte // 中断type
	table     [256]func(*stepInfo)
	stall     int // 剩余等待时钟数
}

// 指令执行需要的信息
type stepInfo struct {
	address uint16
	pc      uint16
	mode    byte
}

func (cpu *CPU) Read16(addr uint16) uint16 {
	low := cpu.Read(addr)
	high := cpu.Read(addr + 1)
	return (uint16(high) << 8) | uint16(low)
}

// func (cpu *CPU) write16(addr uint16, value uint16) {
// 	cpu.Write(addr, byte(value&0xff))
// 	cpu.Write(addr+1, byte(value>>8)&0xff)
// }

// 这里模拟cpu的bug，读取16位数据
// 例如JMP ($10FF), 理论上讲是读取$10FF和$1100这两个字节的数据, 但是实际上是读取的$10FF和$1000这两个字节的数据.
func (cpu *CPU) read16bug(address uint16) uint16 {
	a := address
	b := (a & 0xFF00) | uint16(byte(a)+1)
	lo := cpu.Read(a)
	hi := cpu.Read(b)
	return (uint16(hi) << 8) | uint16(lo)
}

// 栈操作：push/push16/pull/pull16
// 压栈 SP指针向0x00靠近
func (cpu *CPU) push(value byte) {
	// SP  0x00=0xff 对应真实地址的 0x100-0x1ff
	cpu.Write(0x100|uint16(cpu.SP), value)
	cpu.SP--
}

func (cpu *CPU) push16(value uint16) {
	hi := value >> 8
	lo := value & 0xff
	cpu.push(byte(hi))
	cpu.push(byte(lo))
}

// pop a byte from stack
func (cpu *CPU) pull() byte {
	cpu.SP++
	return cpu.Read(0x100 | uint16(cpu.SP))
}

func (cpu *CPU) pull16() uint16 {
	lo := uint16(cpu.pull())
	hi := uint16(cpu.pull())
	return (hi << 8) | lo
}

// 标志寄存器相关
// 零标志
func (cpu *CPU) setZ(value byte) {
	if value == 0 {
		cpu.Z = 1
	} else {
		cpu.Z = 0
	}
}

// 负标志
func (cpu *CPU) setN(value byte) {
	if value&0x80 != 0 {
		cpu.N = 1
	} else {
		cpu.N = 0
	}
}

func (cpu *CPU) setZN(value byte) {
	cpu.setN(value)
	cpu.setZ(value)
}

func (cpu *CPU) getFlags() byte {
	var flags byte
	flags |= cpu.C << 0
	flags |= cpu.Z << 1
	flags |= cpu.I << 2
	flags |= cpu.D << 3
	flags |= cpu.B << 4
	flags |= cpu.U << 5
	flags |= cpu.V << 6
	flags |= cpu.N << 7
	return flags
}

func (cpu *CPU) setFlags(p byte) {
	cpu.C = (p >> 0) & 1
	cpu.Z = (p >> 1) & 1
	cpu.I = (p >> 2) & 1
	cpu.D = (p >> 3) & 1
	cpu.B = (p >> 4) & 1
	cpu.U = (p >> 5) & 1
	cpu.V = (p >> 6) & 1
	cpu.N = (p >> 7) & 1
}

// 中断触发相关, 处理irq、nmi中断
func (cpu *CPU) TriggerIRQ() {
	if cpu.I == 0 {
		cpu.interrupt = interruptIRQ
	}
}

func (cpu *CPU) TriggerNMI() {
	cpu.interrupt = interruptNMI
}

// irq/nmi/brk实现类似
func (cpu *CPU) irq() {
	cpu.push16(cpu.PC)
	cpu.push(cpu.getFlags())
	cpu.PC = cpu.Read16(IRQ)
	cpu.I = 1
	cpu.Cycles += 7
}

func (cpu *CPU) nmi() {
	cpu.push16(cpu.PC)
	cpu.push(cpu.getFlags())
	cpu.PC = cpu.Read16(NMI)
	cpu.I = 1
	cpu.Cycles += 7
}

// 特殊处理，如果是跨branch（地址跳转），cycle++，如果跨page，cycle再+1
func (cpu *CPU) addBranchCycles(info *stepInfo) {
	cpu.Cycles++
	if cpu.pageDiff(info.pc, info.address) {
		cpu.Cycles++
	}
}

// 判断地址是否跨页, 跨页则返回true
func (cpu *CPU) pageDiff(old uint16, new uint16) bool {
	return old&0xff00 != new&0xff00
}

func (cpu *CPU) Reset() {
	cpu.PC = cpu.Read16(RESET)
	cpu.Cycles = 0
	cpu.A = 0
	cpu.X = 0
	cpu.Y = 0
	// 栈指针初始化为$FD即指向$1FD
	cpu.SP = 0xfd
	cpu.setFlags(0x24)
}

func LogReg(cpu *CPU) {
	opcode := cpu.Read(cpu.PC)
	bytes := instructionSizes[opcode]
	name := instructionNames[opcode]
	w0 := fmt.Sprintf("%02X", cpu.Read(cpu.PC+0))
	w1 := fmt.Sprintf("%02X", cpu.Read(cpu.PC+1))
	w2 := fmt.Sprintf("%02X", cpu.Read(cpu.PC+2))

	if bytes < 2 {
		w1 = "  "
	}
	if bytes < 3 {
		w2 = "  "
	}
	fmt.Printf(
		"%4X  %s %s %s  %s %28s"+
			"A:%02X X:%02X Y:%02X P:%02X SP:%02X CYC:%d\n",
		cpu.PC, w0, w1, w2, name, "",
		cpu.A, cpu.X, cpu.Y, cpu.getFlags(), cpu.SP, cpu.Cycles)
}

// step执行一个指令：读指令-寻址-将数据提供给指令方法执行-计算时钟数
func (cpu *CPU) Step() int64 {
	// 日志
	// LogReg(cpu)

	if cpu.stall > 0 {
		cpu.stall--
		return 1
	}

	// 处理下中断的情况
	if cpu.interrupt != interruptNone {
		if cpu.interrupt == interruptIRQ {
			cpu.irq()
		} else if cpu.interrupt == interruptNMI {
			cpu.nmi()
		}
		cpu.interrupt = interruptNone
	}

	// 初始1byte必定是opcode
	opcode := cpu.Read(cpu.PC)
	mode := instructionModes[opcode]
	lastCycles := cpu.Cycles

	var address uint16
	var pageCrossed bool

	// 参考这里： https://github.com/dustpg/BlogFM/issues/9
	switch mode {
	case modeAbsolute:
		address = cpu.Read16(cpu.PC + 1)
	case modeAbsoluteX:
		address = cpu.Read16(cpu.PC+1) + uint16(cpu.X)
		pageCrossed = cpu.pageDiff(address-uint16(cpu.X), address)
	case modeAbsoluteY:
		address = cpu.Read16(cpu.PC+1) + uint16(cpu.Y)
		pageCrossed = cpu.pageDiff(address-uint16(cpu.Y), address)
	case modeAccumulator:
		// cpu.A = cpu.A >> 1
		// 无需地址置0
		address = 0
	case modeImmediate:
		address = cpu.PC + 1
	case modeImplied:
		// cpu.X = cpu.A
		address = 0
	// 变址间接寻址
	case modeIndexedIndirect:
		// 将指令的数据 + X 结果作为地址去获取数据作为新地址
		address = cpu.read16bug(uint16(cpu.Read(cpu.PC+1) + cpu.X))
	// 间接寻址
	case modeIndirect:
		address = cpu.read16bug(cpu.Read16(cpu.PC + 1))
	// 间接变址寻址
	case modeIndirectIndexed:
		address = cpu.read16bug(uint16(cpu.Read(cpu.PC+1))) + uint16(cpu.Y)
		pageCrossed = cpu.pageDiff(address-uint16(cpu.Y), address)
	// 相对寻址
	case modeRelative:
		offset := uint16(cpu.Read(cpu.PC + 1))
		if offset < 0x80 {
			address = cpu.PC + 2 + offset
		} else {
			address = cpu.PC + 2 + offset - 0x100
		}
	case modeZeroPage:
		address = uint16(cpu.Read(cpu.PC+1)) & 0xff
	case modeZeroPageX:
		address = uint16(cpu.Read(cpu.PC+1) + cpu.X)
		address = address & 0xff
	case modeZeroPageY:
		address = uint16(cpu.Read(cpu.PC+1) + cpu.Y)
		address = address & 0xff
	default:
		panic("unknown address mode.")
	}

	size := instructionSizes[opcode]
	cpu.PC += uint16(size)

	// name := instructionNames[opcode]

	pageCycles := instructionPageCycles[opcode]

	cpu.Cycles += uint64(instructionCycles[opcode])
	if pageCrossed {
		cpu.Cycles += uint64(pageCycles)
	}

	info := &stepInfo{address, cpu.PC, mode}

	cpu.table[opcode](info)

	return int64(cpu.Cycles - lastCycles)
}

// LDA - load "A"
func (cpu *CPU) lda(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.A = value
	cpu.setZN(value)
}

// LDX - load "X"
func (cpu *CPU) ldx(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.X = value
	cpu.setZN(value)
}

// LDY - load "Y"
func (cpu *CPU) ldy(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.Y = value
	cpu.setZN(value)
}

// STA - store "A"
func (cpu *CPU) sta(info *stepInfo) {
	cpu.Write(info.address, cpu.A)
}

// STX - store "X"
func (cpu *CPU) stx(info *stepInfo) {
	cpu.Write(info.address, cpu.X)
}

// STY - store "Y"
func (cpu *CPU) sty(info *stepInfo) {
	cpu.Write(info.address, cpu.Y)
}

// 这个自己实现错了，使用了参考项目的代码
// ADC - add with carry -- A = A + M + C
func (cpu *CPU) adc(info *stepInfo) {
	a := cpu.A
	b := cpu.Read(info.address)
	c := cpu.C
	cpu.A = a + b + c
	cpu.setZN(cpu.A)
	if int(a)+int(b)+int(c) > 0xFF {
		cpu.C = 1
	} else {
		cpu.C = 0
	}
	if (a^b)&0x80 == 0 && (a^cpu.A)&0x80 != 0 {
		cpu.V = 1
	} else {
		cpu.V = 0
	}
}

// SBC - subtract with carry -- A = A - M - (1 - C)
func (cpu *CPU) sbc(info *stepInfo) {
	a := cpu.A
	b := cpu.Read(info.address)
	c := cpu.C
	cpu.A = a - b - (1 - c)
	cpu.setZN(cpu.A)
	// 判断进位和溢出
	// 看不太懂。
	if int(a)-int(b)-int(1-c) >= 0 {
		cpu.C = 1
	} else {
		cpu.C = 0
	}
	// 看不懂 +1
	if (a^b)&0x80 != 0 && (a^cpu.A)&0x80 != 0 {
		cpu.V = 1
	} else {
		cpu.V = 0
	}
}

// INC - Increment memory
func (cpu *CPU) inc(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.Write(info.address, value+1)
	cpu.setZN(value + 1)
}

// DEC - Decrement memory
func (cpu *CPU) dec(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.Write(info.address, value-1)
	cpu.setZN(value - 1)
}

// AND - A & memory
func (cpu *CPU) and(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.A = cpu.A & value
	cpu.setZN(cpu.A)
}

// ORA - A | memory
func (cpu *CPU) ora(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.A |= value
	cpu.setZN(cpu.A)
}

// EOR "Exclusive-Or" memory with A
func (cpu *CPU) eor(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.A ^= value
	cpu.setZN(cpu.A)
}

// INX - Increment X
func (cpu *CPU) inx(info *stepInfo) {
	cpu.X++
	cpu.setZN(cpu.X)
}

// DEX - Decrement X
func (cpu *CPU) dex(info *stepInfo) {
	cpu.X--
	cpu.setZN(cpu.X)
}

// INY - Increment Y
func (cpu *CPU) iny(info *stepInfo) {
	cpu.Y++
	cpu.setZN(cpu.Y)
}

// DEY - Decrement Y
func (cpu *CPU) dey(info *stepInfo) {
	cpu.Y--
	cpu.setZN(cpu.Y)
}

// TAX - Transfer A to X
func (cpu *CPU) tax(info *stepInfo) {
	cpu.X = cpu.A
	cpu.setZN(cpu.X)
}

// TXA - Transfer X to A
func (cpu *CPU) txa(info *stepInfo) {
	cpu.A = cpu.X
	cpu.setZN(cpu.A)
}

// TAY - Transfer A to Y
func (cpu *CPU) tay(info *stepInfo) {
	cpu.Y = cpu.A
	cpu.setZN(cpu.Y)
}

// TYA - Transfer Y to A
func (cpu *CPU) tya(info *stepInfo) {
	cpu.A = cpu.Y
	cpu.setZN(cpu.A)
}

// TSX - Transfer SP to X
func (cpu *CPU) tsx(info *stepInfo) {
	cpu.X = cpu.SP
	cpu.setZN(cpu.X)
}

// TXS - Transfer X to SP
func (cpu *CPU) txs(info *stepInfo) {
	cpu.SP = cpu.X
}

// CLC - Clear Carry
func (cpu *CPU) clc(info *stepInfo) {
	cpu.C = 0
}

// SEC - Set Carry
func (cpu *CPU) sec(info *stepInfo) {
	cpu.C = 1
}

// CLD - Clear Decimal
func (cpu *CPU) cld(info *stepInfo) {
	cpu.D = 0
}

// SED - Clear Decimal
func (cpu *CPU) sed(info *stepInfo) {
	cpu.D = 1
}

// CLV - Clear Overflow
func (cpu *CPU) clv(info *stepInfo) {
	cpu.V = 0
}

// CLI - Clear Interrupt-disable
func (cpu *CPU) cli(info *stepInfo) {
	cpu.I = 0
}

// SEI - Set Interrupt-disable
func (cpu *CPU) sei(info *stepInfo) {
	cpu.I = 1
}

func (cpu *CPU) compare(a, b byte) {
	cpu.setZN(a - b)
	if a >= b {
		cpu.C = 1
	} else {
		cpu.C = 0
	}
}

// CMP - Compare memory with A
func (cpu *CPU) cmp(info *stepInfo) {
	value := cpu.Read(info.address)
	// if int(cpu.A)-int(value) < 0x100 {
	// 	cpu.C = 1
	// } else {
	// 	cpu.C = 0
	// }
	// cpu.setZN(cpu.A - value)
	cpu.compare(cpu.A, value)
}

// CPX - Compare memory with X
func (cpu *CPU) cpx(info *stepInfo) {
	value := cpu.Read(info.address)
	// if int(cpu.X)-int(value) < 0x100 {
	// 	cpu.C = 1
	// } else {
	// 	cpu.C = 0
	// }
	// cpu.setZN(cpu.X - value)
	cpu.compare(cpu.X, value)
}

// CPY - Compare memory with Y
func (cpu *CPU) cpy(info *stepInfo) {
	value := cpu.Read(info.address)
	// if int(cpu.Y)-int(value) < 0x100 {
	// 	cpu.C = 1
	// } else {
	// 	cpu.C = 0
	// }
	// cpu.setZN(cpu.Y - value)
	cpu.compare(cpu.Y, value)
}

// BIT - Bit test memory with A
func (cpu *CPU) bit(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.setZ(cpu.A & value)
	cpu.V = (value >> 6) & 1
	cpu.N = (value >> 7) & 1
}

// ASL - Arithmetic Shift Left --  C <- |7|6|5|4|3|2|1|0| <- 0
func (cpu *CPU) asl(info *stepInfo) {
	if info.mode == modeAccumulator {
		cpu.C = (cpu.A >> 7) & 1
		cpu.A <<= 1
		cpu.setZN(cpu.A)
	} else {
		value := cpu.Read(info.address)
		cpu.C = (value >> 7) & 1
		value <<= 1
		cpu.Write(info.address, value)
		cpu.setZN(value)
	}
}

// LSR - Logical Shift Right
func (cpu *CPU) lsr(info *stepInfo) {
	if info.mode == modeAccumulator {
		cpu.C = cpu.A & 1
		cpu.A >>= 1
		cpu.setZN(cpu.A)
	} else {
		value := cpu.Read(info.address)
		cpu.C = value & 1
		value >>= 1
		cpu.Write(info.address, value)
		cpu.setZN(value)
	}
}

// ROL - Rotate Left
func (cpu *CPU) rol(info *stepInfo) {
	if info.mode == modeAccumulator {
		c := cpu.C
		cpu.C = (cpu.A >> 7) & 1
		cpu.A = (cpu.A << 1) | c
		cpu.setZN(cpu.A)
	} else {
		c := cpu.C
		value := cpu.Read(info.address)
		cpu.C = (value >> 7) & 1
		value = (value << 1) | c
		cpu.setZN(value)
		cpu.Write(info.address, value)
	}
}

// ROR - Rotate Right
func (cpu *CPU) ror(info *stepInfo) {
	if info.mode == modeAccumulator {
		c := cpu.C
		cpu.C = cpu.A & 1
		cpu.A = (cpu.A >> 1) | (c << 7)
		cpu.setZN(cpu.A)
	} else {
		c := cpu.C
		value := cpu.Read(info.address)
		cpu.C = value & 1
		value = (value >> 1) | (c << 7)
		cpu.setZN(value)
		cpu.Write(info.address, value)
	}
}

// PHA - Push A
func (cpu *CPU) pha(info *stepInfo) {
	cpu.push(cpu.A)
}

// PLA - Pull(Pop) A
func (cpu *CPU) pla(info *stepInfo) {
	cpu.A = cpu.pull()
	cpu.setZN(cpu.A)
}

// PHP - Push Processor-status
func (cpu *CPU) php(info *stepInfo) {
	cpu.push(cpu.getFlags())
}

// PLP - Pull Processor-status
func (cpu *CPU) plp(info *stepInfo) {
	cpu.setFlags(cpu.pull()&0xef | 0x20)
}

// JMP - Jump
func (cpu *CPU) jmp(info *stepInfo) {
	cpu.PC = info.address
}

// BEQ - Branch if Equal
func (cpu *CPU) beq(info *stepInfo) {
	if cpu.Z > 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BNE - Branch if Not Equal
func (cpu *CPU) bne(info *stepInfo) {
	if cpu.Z == 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BCS - Branch if Carry Set
func (cpu *CPU) bcs(info *stepInfo) {
	if cpu.C > 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BCC - Branch if Carry Clear
func (cpu *CPU) bcc(info *stepInfo) {
	if cpu.C == 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BMI - Branch if Minus
func (cpu *CPU) bmi(info *stepInfo) {
	if cpu.N > 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BPL - Branch if Plus
func (cpu *CPU) bpl(info *stepInfo) {
	if cpu.N == 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BVS - Branch if Overflow Set
func (cpu *CPU) bvs(info *stepInfo) {
	if cpu.V > 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BVC - Branch if Overflow Clear
func (cpu *CPU) bvc(info *stepInfo) {
	if cpu.V == 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// JSR - Jump to Subroutine
func (cpu *CPU) jsr(info *stepInfo) {
	cpu.push16(cpu.PC - 1)
	cpu.PC = info.address
}

// RTS - Return from Subroutine
func (cpu *CPU) rts(info *stepInfo) {
	cpu.PC = cpu.pull16() + 1
}

// NOP - do nothing... 哈？
func (cpu *CPU) nop(info *stepInfo) {}

// BRK 强制中断
func (cpu *CPU) brk(info *stepInfo) {
	cpu.push(byte(cpu.PC >> 8))
	cpu.push(byte(cpu.PC) & 0xff)
	cpu.push(cpu.getFlags())
	cpu.PC = cpu.Read16(IRQ)
}

// RTI - Return from Interrupt
func (cpu *CPU) rti(info *stepInfo) {
	cpu.setFlags(cpu.pull())
	cpu.PC = cpu.pull16()
}

// 以下指令未实现
func (cpu *CPU) kil(info *stepInfo) {}

func (cpu *CPU) slo(info *stepInfo) {}

func (cpu *CPU) anc(info *stepInfo) {}

func (cpu *CPU) sre(info *stepInfo) {}

func (cpu *CPU) sax(info *stepInfo) {}

func (cpu *CPU) rla(info *stepInfo) {}

func (cpu *CPU) alr(info *stepInfo) {}

func (cpu *CPU) rra(info *stepInfo) {}

func (cpu *CPU) arr(info *stepInfo) {}

func (cpu *CPU) xaa(info *stepInfo) {}

func (cpu *CPU) ahx(info *stepInfo) {}

func (cpu *CPU) shx(info *stepInfo) {}

func (cpu *CPU) shy(info *stepInfo) {}

func (cpu *CPU) tas(info *stepInfo) {}

func (cpu *CPU) lax(info *stepInfo) {}

func (cpu *CPU) las(info *stepInfo) {}

func (cpu *CPU) dcp(info *stepInfo) {}

func (cpu *CPU) isc(info *stepInfo) {}

func (cpu *CPU) axs(info *stepInfo) {}

/*
https://www.jianshu.com/p/ba75b1186ecd
指令 https://www.jianshu.com/p/ba75b1186ecd

P 状态寄存器
BIT	名称	含义
0	C	进位标志，如果计算结果产生进位，则置 1
1	Z	零标志，如果结算结果为 0，则置 1
2	I	中断去使能标志，置 1 则可屏蔽掉 IRQ 中断
3	D	十进制模式，未使用
4	B	BRK
5	U	未使用
6	V	溢出标志，如果结算结果产生了溢出，则置 1
7	N	负标志，如果计算结果为负，则置 1
*/

/**
BANK是每个Mapper载入的单位
根据地址划分为每8KB一个BANK, 一共8个区块:

0: [$0000, $2000) cpu 内存
1: [$2000, $4000) PPU 寄存器
2: [$4000, $6000) pAPU寄存器以及扩展区域
3: [$6000, $8000) 存档用SRAM区
剩下的全是 程序代码区 PRG-ROM [$8000, $0x10000)
0xfff0附近有中断相关信息

$FFFA-FFFB = NMI
$FFFC-FFFD = RESET
$FFFE-FFFF = IRQ/BRK

两byte存的是中断触发时跳转到的制定位置的16位地址
*/

/*
寻址模式
寻址, 顾名思义寻找地址.
寻址方式就是处理器根据指令中给出的地址信息来寻找有效地址的方式，
是确定本条指令的数据地址以及下一条要执行的指令地址的方法
*/

/*
指令：
（1）指令地址和指令+数据的关系

地址：0xffff 16位-2byte
数据：0xff 8位-1byte

指令长度不一样，最短是1，最长是3 （参见instructionSizes）

 // 用32位保存数据
    uint32_t    data;
    // 4个8位数据
    struct {
        // 操作码
        uint8_t op;
        // 地址码1
        uint8_t a1;
        // 地址码2
        uint8_t a2;
        // 显示控制
        uint8_t ctrl;
    };

这个c语言结构就表明：
最开始8位是opcode，后面有0-2个8位数据，显示控制暂时用不到
这也就是为什么有的指令长度是1（只有操作码）
有的指令长度是3（opcode + 2个8位数据）

opcode是一个8位数，长度0-255，也就对应了256个操作码

（2）指令共256个，有些是非官方的，有些不一定要实现
*/

/*

额外的时钟
有两种情况会额外增加时钟

(1)分支指令进行跳转时,分支指令比如 BNE，BEQ 这类指令，
如果检测条件为真，这时需要额外增加 1 个时钟

(2)跨 Page 访问
新地址和旧地址如果 Page 不一样，即 (newAddr & 0xFF00) !== (oldAddr & 0xFF00)，
则需要额外增加一个时钟。例如 0x1234 与 0x12FF 为同一 Page，但是与 0x1334 为不同 Page
以上两种情况可以同时存在，所以一条指令可能会额外增加 1 ~ 2 个时钟

*/
