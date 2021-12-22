package nes

// 帧计数器 240Hz
const FrameCounterRate = 240.0

// 5bit(<=31)的长度计数器值是索引，索引到下面的数组的值才是真实值
var lengthTable = []byte{
	10, 254, 20, 2, 40, 4, 80, 6, 160, 8, 60, 10, 14, 12, 26, 14,
	12, 16, 24, 18, 48, 20, 96, 22, 192, 24, 72, 26, 16, 28, 32, 30,
}

var triangleTable = []byte{
	15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

var noiseTable = []uint16{
	4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068,
}

var dmcTable = []byte{
	214, 190, 170, 160, 143, 127, 113, 107, 95, 80, 71, 64, 53, 42, 36, 27,
}

// 占空比的slice
var dutyTable = [][]byte{
	{0, 1, 0, 0, 0, 0, 0, 0},
	{0, 1, 1, 0, 0, 0, 0, 0},
	{0, 1, 1, 1, 1, 0, 0, 0},
	{1, 0, 0, 1, 1, 1, 1, 1},
}

// 查表法降低计算复杂度
var pulseTable [31]float32
var tndTable [203]float32

func init() {
	for i := 0; i < 31; i++ {
		pulseTable[i] = 95.52 / (8128.0/float32(i) + 100)
	}
	for i := 0; i < 203; i++ {
		tndTable[i] = 163.67 / (24329.0/float32(i) + 100)
	}
}

type APU struct {
	// channel    chan float32
	outputWork func(float32)
	sampleRate float64
	console    *Console
	cycle      uint64
	last_cycle uint64

	pulse1 Pulse
	pulse2 Pulse

	triangle Triangle

	noise Noise

	dmc DMC

	frameMode      byte // 0：4 步模式，1：5 步模式
	frameForbidIRQ byte // 中断禁止标志 0：使能中断，1：禁用中断
	frameCounter   uint64

	dmcIRQ    byte // DMC的IRQ标志
	frameIRQ  byte // 帧中断标志, 如果为1, 会确认IRQ, 返回1置为0
	dmcActive byte // DMC还有剩余样本(剩余字节>0)
}

type Pulse struct {
	enabled           bool // 使能位
	channel           byte // 哪个方波 1/2
	dutyMode          byte // 占空比模式
	dutyValue         byte // 分频器的计时器当前值(占空比序列index)
	lengthEnable      bool // 使能长度计数器
	lengthValue       byte
	timerPeriod       uint16 // 计数器
	timerValue        uint16
	envelopeEnable    bool // 使用包络的开关，关闭就使用固定音量
	envelopeLoop      bool // 循环开关
	envelopeStart     bool
	envelopePeriod    byte // 包络分频器P值
	envelopeValue     byte // 包络计时器
	envelopeVolume    byte // 包络音量
	constVolumeEnable bool // 使用固定音量
	constVolume       byte // 固定音量模式下的音量
	sweepEnable       bool
	sweepPeriod       byte
	sweepReverse      byte
	sweepShift        byte
	sweepValue        byte
	sweepReload       bool // 写入sweep时候设为true
}

// 方波的第一个字节作为控制信息
func (p *Pulse) writeCtrl(value byte) {
	p.dutyMode = (value >> 6) & 0x3
	p.envelopeLoop = (value>>5)&1 > 0
	p.lengthEnable = (value>>5)&1 == 0
	p.envelopeEnable = (value>>4)&1 == 0
	p.constVolumeEnable = (value>>4)&1 > 0
	p.constVolume = value & 0xf
	p.envelopePeriod = value & 0xf

	p.envelopeStart = true
}

// 声道周期即Timer周期
func (p *Pulse) writeTimerLow(value byte) {
	p.timerPeriod &= 0xff00
	p.timerPeriod |= uint16(value)
}

func (p *Pulse) writeTimerHigh(value byte) {
	p.timerPeriod &= 0xff
	p.timerPeriod |= (uint16(value&0x7) << 8)

	p.envelopeStart = true
	p.dutyValue = 0
}

func (p *Pulse) writeSweep(value byte) {
	p.sweepEnable = (value>>7)&1 > 0
	p.sweepPeriod = (value >> 4) & 0x7
	p.sweepReverse = (value >> 3) & 1
	p.sweepShift = value & 0x7
}

func (p *Pulse) writeLength(value byte) {
	lengthIndex := (value >> 3) & 0x1f
	p.lengthValue = lengthTable[lengthIndex]
}

func (p *Pulse) stepTimer() {
	if p.timerValue == 0 {
		p.timerValue = p.timerPeriod
		p.dutyValue = (p.dutyValue + 1) % 8
	} else {
		p.timerValue--
	}
}

func (p *Pulse) stepEnvelope() {
	if p.envelopeStart {
		p.envelopeVolume = 15
		p.envelopeValue = p.envelopePeriod
		p.envelopeStart = false
	} else if p.envelopeValue > 0 {
		p.envelopeValue--
	} else {
		if p.envelopeLoop {
			p.envelopeVolume = 15
		} else if p.envelopeVolume > 0 {
			p.envelopeVolume--
		}
		p.envelopeValue = p.envelopePeriod
	}
}

// EPPP NSSS  使能标志, 分频器周期(需要+1), 负向标志位, 位移数量
// sweep分频器周期sweepPeriod是3bit P+1最大是8
func (p *Pulse) stepSweep() {
	// 复位则重新执行
	if p.sweepReload {
		if p.sweepEnable && p.sweepValue == 0 {
			p.sweep()
		}
		p.sweepValue = p.sweepPeriod
		p.sweepReload = false
		// 正常计时器计时
	} else if p.sweepValue > 0 {
		p.sweepValue--
	} else {
		// 正常计时器 计时结束执行sweep输出
		if p.sweepEnable {
			p.sweep()
		}
		p.sweepValue = p.sweepPeriod
	}
}

// sweep扫描单元输出
func (p *Pulse) sweep() {
	delta := p.timerPeriod >> p.sweepShift
	if p.sweepReverse > 0 {
		p.timerPeriod -= delta
		// 方波一要额外减1
		if p.channel == 1 {
			p.timerPeriod--
		}
	} else {
		p.timerPeriod += delta
	}
}

func (p *Pulse) stepLength() {
	if p.lengthEnable && p.lengthValue > 0 {
		p.lengthValue--
	}
}

// 内部计数器到0的话就会静音
// 声道timer周期<8静音
// 声道timer周期超过11bit的范围($800), 应该被静音

// 每种通道各自的输出
func (p *Pulse) output() byte {
	if !p.enabled {
		return 0
	}
	if p.lengthValue == 0 {
		return 0
	}
	if dutyTable[p.dutyMode][p.dutyValue] == 0 {
		return 0
	}
	if p.timerPeriod < 8 || p.timerPeriod > 0x800 {
		return 0
	}
	// 当占空比切片对应dutyValue的值为1时，输出声音值float32
	// 这个声音要么是包络输出音量，要么是固定音量
	if p.envelopeEnable {
		return p.envelopeVolume
	} else {
		return p.constVolume
	}
}

type Triangle struct {
	enabled           bool
	timerPeriod       uint16 // 计数器
	timerValue        uint16
	dutyValue         byte // 波形index
	lengthEnable      bool
	lengthValue       byte
	lengthReload      bool
	linearEnable      bool
	linearValue       byte
	linearReloadValue byte
}

func (t *Triangle) writeLinearCtrl(value byte) {
	bool := (value>>7)&1 > 0
	t.lengthEnable = !bool
	t.linearEnable = bool
	t.linearReloadValue = value & 0x7f
}

func (t *Triangle) writePeriodLow(value byte) {
	t.timerPeriod = t.timerPeriod & 0xff00
	t.timerPeriod |= uint16(value)
}

func (t *Triangle) writePeriodHigh(value byte) {
	t.timerPeriod = t.timerPeriod & 0xff
	t.timerPeriod |= (uint16(value&0x7) << 8)
	t.lengthValue = lengthTable[(value >> 3)]
	t.lengthReload = true
	t.dutyValue = 0
}

// 本质就是一个计时器，定时器执行完一个循环后，则计算一次波形
func (t *Triangle) stepTimer() {
	if t.timerValue == 0 {
		t.timerValue = t.timerPeriod
		// 波形index对波形列表的长度取余
		t.dutyValue = (t.dutyValue + 1) % 32
	} else {
		t.timerValue--
	}
}

func (t *Triangle) stepLength() {
	if t.lengthEnable && t.lengthValue > 0 {
		t.lengthValue--
	}
}

func (t *Triangle) stepLinear() {
	// 如果之前写入了$400B, 则标记暂停
	if t.lengthReload {
		t.linearValue = t.linearReloadValue
		t.lengthReload = false
	} else if t.linearValue > 0 {
		t.linearValue--
	}
}

// 长度计数器到0/线性计数器到0/置空状态寄存器对应的使能位能 都让声道静音
// 输出triangleTable对应索引的值
func (t *Triangle) output() byte {
	if !t.enabled {
		return 0
	}
	if t.lengthValue == 0 || t.linearValue == 0 {
		return 0
	}
	return triangleTable[t.dutyValue]
}

type Noise struct {
	enabled         bool
	shortMode       bool
	shiftRegister   uint16 // 内部的移位寄存器15bit
	lengthEnabled   bool
	lengthValue     byte
	timerPeriod     uint16
	timerValue      uint16
	envelopeEnabled bool
	envelopeLoop    bool
	envelopeStart   bool
	envelopePeriod  byte
	envelopeValue   byte
	envelopeVolume  byte
	constantVolume  byte
}

// $400C	--lc.vvvv	Length counter halt, constant volume/envelope flag, const volume/envelope divider period (write)
func (n *Noise) writeEnvelope(value byte) {
	bool := (value>>5)&1 > 0
	n.lengthEnabled = !bool
	n.envelopeLoop = bool
	n.envelopeEnabled = (value>>4)&1 == 0
	// vvvv 既是envelope周期，又是固定音量大小
	n.envelopePeriod = value & 0xf
	n.constantVolume = value & 0xf
	n.envelopeStart = true
}

func (n *Noise) writeTimerPeriod(value byte) {
	n.shortMode = (value>>7)&1 > 0
	n.timerPeriod = noiseTable[uint16(value&0xf)]
}

func (n *Noise) writeLength(value byte) {
	n.lengthValue = lengthTable[(value>>3)&0x1f]
}

// 将D0位与D1做异或运算, 如果是短模式则是D0位和D6位做异或运算
// LFSR右移一位, 并将之前运算结果作为最高位(D14)
func (n *Noise) stepLFSR() {
	var top byte
	if n.shortMode {
		top = byte((n.shiftRegister & 1) ^ (n.shiftRegister & 0x2))
	} else {
		top = byte((n.shiftRegister & 1) ^ (n.shiftRegister & 0x40))
	}
	n.shiftRegister = ((n.shiftRegister >> 1) & 0x3fff) | (uint16(top) << 14)
}

func (n *Noise) stepTimer() {
	if n.timerValue == 0 {
		n.timerValue = n.timerPeriod
		// 这里更新一次LFSR
		n.stepLFSR()
	} else {
		n.timerValue--
	}
}

func (n *Noise) stepLength() {
	if n.lengthEnabled && n.lengthValue > 0 {
		n.lengthValue--
	}
}

func (n *Noise) stepEnvelope() {
	if n.envelopeStart {
		n.envelopeVolume = 15
		n.envelopeValue = n.envelopePeriod
		n.envelopeStart = false
	} else if n.envelopeValue > 0 {
		n.envelopeValue--
	} else {
		if n.envelopeLoop {
			n.envelopeVolume = 15
		} else if n.envelopeVolume > 0 {
			n.envelopeVolume--
		}
		n.envelopeValue = n.envelopePeriod
	}
}

func (n *Noise) output() byte {
	if !n.enabled {
		return 0
	}
	if !n.lengthEnabled || n.lengthValue == 0 {
		return 0
	}
	// 寄存器D0为0才能输出音量
	if n.shiftRegister&1 > 0 {
		return 0
	}
	if n.envelopeEnabled {
		return n.envelopeValue
	} else {
		return n.constantVolume
	}
}

type DMC struct {
	enabled        bool
	cpu            *CPU
	value          byte // DAC值
	sampleAddress  uint16
	sampleLength   uint16
	currentAddress uint16
	currentLength  uint16
	shiftRegister  byte
	bitCount       byte
	tickPeriod     byte
	tickValue      byte
	loop           bool
	irq            bool
}

// $4010
func (d *DMC) writePeriod(value byte) {
	d.irq = (value >> 7) > 0
	d.loop = (value >> 6) > 0
	d.tickPeriod = dmcTable[value&0xf]
}

// $4011
func (d *DMC) writeValue(value byte) {
	d.value = value & 0x7f
}

// $4012
// %11AAAAAA.AA000000
func (d *DMC) writeSampleAddr(value byte) {
	d.sampleAddress = (uint16(value) | (0x3 << 8)) << 6
}

// $4013
// LLLL.LLLL0001
func (d *DMC) writeSampleLength(value byte) {
	d.sampleLength = (uint16(value) << 4) | 1
}

func (d *DMC) restart() {
	d.currentAddress = d.sampleAddress
	d.currentLength = d.sampleLength
}

func (d *DMC) stepTimer() {
	if !d.enabled {
		return
	}
	d.stepReader()
	if d.tickValue == 0 {
		d.tickValue = d.tickPeriod
		d.stepShifter()
	} else {
		d.tickValue--
	}
}

func (d *DMC) stepReader() {
	if d.currentLength > 0 && d.bitCount == 0 {
		d.cpu.stall += 4
		d.shiftRegister = d.cpu.Read(d.currentAddress)
		d.bitCount = 8
		d.currentAddress++
		if d.currentAddress == 0 {
			d.currentAddress = 0x8000
		}
		d.currentLength--
		if d.currentLength == 0 && d.loop {
			d.restart()
		}
	}
}

func (d *DMC) stepShifter() {
	if d.bitCount == 0 {
		return
	}
	if d.shiftRegister&1 == 1 {
		if d.value <= 125 {
			d.value += 2
		}
	} else {
		if d.value >= 2 {
			d.value -= 2
		}
	}
	d.shiftRegister >>= 1
	d.bitCount--
}

func (d *DMC) output() byte {
	return d.value
}

func NewAPU(console *Console) *APU {
	apu := APU{}
	apu.console = console
	apu.noise.shiftRegister = 1
	apu.pulse1.channel = 1
	apu.pulse2.channel = 2

	apu.dmc.cpu = console.CPU
	return &apu
}

func (apu *APU) Step() {
	// APU基本时钟频率是cpu的一半, 三角波不是

	apu.stepTimer()

	// cycle时钟达到了240Hz
	if float64(apu.cycle-apu.last_cycle) >= float64(CPUFrequency)/FrameCounterRate {
		apu.stepFrameCounter()
		apu.last_cycle = apu.cycle
	}

	s1 := int(float64(apu.cycle) / apu.sampleRate)
	s2 := int(float64(apu.cycle+1) / apu.sampleRate)
	if s1 != s2 {
		apu.sendSample()
	}

	apu.cycle++
}

func (apu *APU) sendSample() {
	output := apu.output()
	// apu.channel <- output
	if apu.outputWork != nil {
		apu.outputWork(output)
	}
}

// 帧计数器
func (apu *APU) stepFrameCounter() {
	// 4步
	if apu.frameMode == 0 {
		apu.stepEnvelope()
		apu.stepLinear()
		switch apu.frameCounter % 4 {
		case 1:
			apu.stepLength()
			apu.stepSweep()
		case 3:
			apu.stepLength()
			apu.stepSweep()
			apu.triggerIRQ()
		}
	} else {
		// 5步
		switch apu.frameCounter % 5 {
		case 0, 2:
			apu.stepEnvelope()
			apu.stepLinear()
			apu.stepLength()
			apu.stepSweep()
		case 1, 3:
			apu.stepEnvelope()
			apu.stepLinear()
		}
	}
	apu.frameCounter++
}

func (apu *APU) stepTimer() {
	if apu.cycle%2 == 0 {
		apu.pulse1.stepTimer()
		apu.pulse2.stepTimer()
		apu.noise.stepTimer()
		apu.dmc.stepTimer()
	}
	apu.triangle.stepTimer()
}

// 包络
func (apu *APU) stepEnvelope() {
	apu.pulse1.stepEnvelope()
	apu.pulse2.stepEnvelope()
	apu.noise.stepEnvelope()
}

// 线性计数器
func (apu *APU) stepLinear() {
	apu.triangle.stepLinear()
}

// 长度计数器
func (apu *APU) stepLength() {
	apu.pulse1.stepLength()
	apu.pulse2.stepLength()
	apu.triangle.stepLength()
	apu.noise.stepLength()
}

// 扫描单元
func (apu *APU) stepSweep() {
	apu.pulse1.stepSweep()
	apu.pulse2.stepSweep()
}

/*
output = square_out + tnd_out
                          95.88
    square_out = -----------------------
                        8128
                 ----------------- + 100
                 square1 + square2


                          159.79
    tnd_out = ------------------------------
                          1
              ------------------------ + 100
              triangle   noise    dmc
              -------- + ----- + -----
                8227     12241   22638
*/

// 最终输出
func (apu *APU) output() float32 {
	p1 := apu.pulse1.output()
	p2 := apu.pulse2.output()
	t := apu.triangle.output()
	n := apu.noise.output()
	dmc := apu.dmc.output()

	pulseOut := pulseTable[p1+p2]
	tndOut := tndTable[3*t+2*n+dmc]

	return pulseOut + tndOut
}

func (apu *APU) triggerIRQ() {
	if apu.frameForbidIRQ == 0 {
		apu.console.CPU.TriggerIRQ()
	}
}

func (apu *APU) writeRegister(addr uint16, value byte) {
	switch addr {
	case 0x4000:
		apu.pulse1.writeCtrl(value)
	case 0x4001:
		apu.pulse1.writeSweep(value)
	case 0x4002:
		apu.pulse1.writeTimerLow(value)
	case 0x4003:
		apu.pulse1.writeTimerHigh(value)
		apu.pulse1.writeLength(value)
	case 0x4004:
		apu.pulse2.writeCtrl(value)
	case 0x4005:
		apu.pulse2.writeSweep(value)
	case 0x4006:
		apu.pulse2.writeTimerLow(value)
	case 0x4007:
		apu.pulse2.writeTimerHigh(value)
		apu.pulse2.writeLength(value)
	case 0x4008:
		apu.triangle.writeLinearCtrl(value)
	case 0x4009:
		// 未使用
	case 0x400a:
		apu.triangle.writePeriodLow(value)
	case 0x400b:
		apu.triangle.writePeriodHigh(value)
	case 0x400c:
		apu.noise.writeEnvelope(value)
	case 0x400e:
		apu.noise.writeTimerPeriod(value)
	case 0x400f:
		apu.noise.writeLength(value)
	case 0x4010:
		apu.dmc.writePeriod(value)
	case 0x4011:
		apu.dmc.writeValue(value)
	case 0x4012:
		apu.dmc.writeSampleAddr(value)
	case 0x4013:
		apu.dmc.writeSampleLength(value)
	case 0x4015:
		apu.writeStatus(value)
	case 0x4017:
		apu.writeFrameCounter(value)
	}
}

func (apu *APU) writeFrameCounter(value byte) {
	apu.frameMode = (value >> 7) & 1
	apu.frameForbidIRQ = (value >> 6) & 1

	// 5步模式
	if apu.frameMode == 1 {
		apu.stepEnvelope()
		apu.stepSweep()
		apu.stepLength()
	}
}

func (apu *APU) ReadRegister(addr uint16) byte {
	if addr == 0x4015 {
		return apu.readStatus(addr)
	}
	return 0
}

// 0x4015 APU状态寄存器，唯一可读寄存器
func (apu *APU) readStatus(addr uint16) byte {
	var status byte
	status |= (apu.dmcIRQ << 7)
	status |= (apu.frameIRQ << 6)
	status |= (apu.dmcActive << 4)
	if apu.noise.lengthValue > 0 {
		status |= (1 << 3)
	}
	if apu.triangle.lengthValue > 0 {
		status |= (1 << 2)
	}
	if apu.pulse2.lengthValue > 0 {
		status |= (1 << 1)
	}
	if apu.pulse1.lengthValue > 0 {
		status |= 1
	}
	return status
}

func (apu *APU) writeStatus(value byte) {
	apu.dmcIRQ = (value >> 7) & 1
	apu.frameIRQ = (value >> 6) & 1
	apu.dmcActive = (value >> 4) & 1

	apu.pulse1.enabled = value&1 > 0
	apu.pulse2.enabled = (value>>1)&1 > 0

	apu.triangle.enabled = (value>>2)&1 > 0

	apu.noise.enabled = (value>>3)&1 > 0

	// 一些初始化
	if !apu.pulse1.enabled {
		apu.pulse1.lengthValue = 0
	}
	if !apu.pulse2.enabled {
		apu.pulse2.lengthValue = 0
	}
	if !apu.triangle.enabled {
		apu.triangle.lengthValue = 0
	}
	if !apu.noise.enabled {
		apu.noise.lengthValue = 0
	}
	if !apu.dmc.enabled {
		apu.dmc.currentLength = 0
	} else {
		if apu.dmc.currentLength == 0 {
			apu.dmc.restart()
		}
	}
}

// divider
// 帧计数器(Frame Counter) / 帧序列器(Frame Sequencer)
/*
0x4007
xxxx xxxx
1 模式（0: 4步、1：五步）
01 IRQ禁止标志位

*/

/*
$4000-$4003	First pulse wave 方波1
$4004-$4007	Second pulse wave 方波2
$4008-$400B	Triangle wave 三角波
$400C-$400F	Noise 噪声

方波：
可控制占空比和音量 4个byte
0x4000/0x4004 --- %DDLCVVVV
占空比和音量控制 DD: 00=12.5% 01=25% 10=50% 11=75%; VVVV: 0000=silence 1111=maximum
L 包络循环/暂停长度计数器 C 固定音量标志位(1: 固定音量 0:使用包络的音量)
0x4001/0x4005 --- %EPPP NSSS
扫描单元: enabled (E), period (P), 负向扫描 (N), 位移次数 (S)
0x4002/0x4006 --- %LLLLLLLL 声道周期Timer低8位
0x4003/0x4007 --- %LLLLLHHH  H: 声道周期Timer高三位 L：长度计数器加载索引值

三角波：
可控制频率和静音
$4008	%CRRR RRRR 暂停长度计数器/线性计数器控制(C), 线性计数器 (R)
0x4009 未使用
$400A	%LLLLLLLL	声道周期Timer低8位
$400B	%LLLLLHHH	H：声道周期Timer高三位， L: 长度计数器加载索引值

噪声波：
可控制频率，音量和音色。
$400C	%--LC VVVV  包络循环/长度计数器停止标志位(L), 固定音量标志位(1: 固定音量 0:使用包络的音量) (C), VVVV: 0000=silence 1111=maximum
0x400E	L--- PPPP	Loop noise (L), noise period (P)
0x400F	LLLL L---	Length counter load (L)

注意：
1. 方波，三角形和噪声通道将在其长度计数器所有非零时播放它们的相应波形
2. 长度计数器可以被暂停. 暂停标志位是在D5位(方波、噪声), 或者D7位(三角波), 设置为0表示暂停
3. 寄存器写入0，对应的声道就会静音, 同时让长度计数器归零

状态寄存器$4015
$4015是APU相关寄存器唯一一个可读的寄存器.

*/

/*
APU两个时钟
基本时钟（APU 周期）：CPU clock / 2
用于控制波形频率
帧计数器：240Hz
用于控制波形持续时间

已经过去的时间(s)大于等于帧计数器的每次执行耗时，则执行一次帧计数器
(cycle - last_cycle) / CPUFrequency >= 1 / 240.0
转换下就变成了
(cycle - last_cycle) >= CPUFrequency / 240.0

帧计数器 位于地址 0x4017，用来驱动各通道的长度，包络等单元
BIT	作用
0	0：4 步模式，1：5 步模式
1	中断禁止标志，0：使能中断，1：禁用中断

4 步模式	5 步模式	功能
- - - f	- - - - -	产生中断
- l - l	- l - - l	驱动长度计数器（Length counter）和扫描单元（Sweep）
e e e e	e e e - e	驱动包络（Envelope）与线性计数器（Linear counter）

*/

/*
通道	单元
方波1 (pulse1)	Timer, length counter, envelope, sweep
方波2 (pulse2)	Timer, length counter, envelope, sweep
三角波 (triangle)	Timer, length counter, linear counter
噪声 (noise)	Timer, length counter, envelope, linear feedback shift register
DMC	Timer, memory reader, sample buffer, output unit

*/

/*
噪声的移位寄存器：
噪声声道有一个15bit的LFSR, 每次输出最低的bit位.算法如下:
1. 将D0位与D1做异或运算
2. 如果是短模式则是D0位和D6位做异或运算
3. LFSR右移一位, 并将之前运算结果作为最高位(D14)

噪音生成的逻辑:

每P次APU(CPU?)周期更新一次LFSR
每次采样时: 检测LFSR最低位
0: 输出音量
1: 输出0

*/
