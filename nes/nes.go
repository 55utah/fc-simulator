package nes

import (
	"io/ioutil"
	"os"
	"strings"
)

func LoadNESRom(url string) (*Cartridge, error) {
	file, err := os.Open(url)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	// []byte
	info, err2 := ioutil.ReadAll(file)
	if err2 != nil {
		panic(err2)
	}
	tag := info[0:4]
	if !strings.Contains(string(tag), "NES") {
		panic("Err: not NES file.")
	}

	prgNum := info[4] // PRG块数目 一块大小为 16KB
	chrNum := info[5] // CHR块数目 一块大小为 8KB

	flag := info[6]
	flag2 := info[7]

	// trained := int8(flag)&0b100 > 0
	isNesV2 := int8(flag2)&0b1100 > 0
	if isNesV2 {
		panic("Err: not support nes v2.")
	}
	mirror := flag & 1
	mapper := ((flag & 0xf0) >> 4) | (flag2 & 0xf0)

	prg := make([]byte, int(prgNum)*16384)
	copy(prg, info[16:16+int(prgNum)*16384])

	chr := make([]byte, int(chrNum)*8192)
	copy(chr, info[16+int(prgNum)*16384:])

	if chrNum == 0 {
		chr = make([]byte, 8192)
	}

	Logger("ROM: PRG-ROM: %d x 16kb, CHR_ROM: %d x 8kb Mapper: %d \n", prgNum, chrNum, mapper)
	return NewCartridge(prg, chr, mapper, mirror), nil
}

/*
FALG

76543210
||||||||
|||||||+- Mirroring: 0: 水平镜像（PPU 章节再介绍）
|||||||              1: 垂直镜像（PPU 章节再介绍）
||||||+-- 1: 卡带上有没有带电池的 SRAM
|||||+--- 1: Trainer 标志
||||+---- 1: 4-Screen 模式（PPU 章节再介绍）
++++----- Mapper 号的低 4 bit

FLAG2
76543210
||||||||
|||||||+- VS Unisystem，不需要了解
||||||+-- PlayChoice-10，不需要了解
||||++--- 如果为 2，代表 NES 2.0 格式，不需要了解
++++----- Mapper 号的高 4 bit

*/
