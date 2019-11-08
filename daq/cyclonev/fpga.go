// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package cyclonev

import (
	"log"
	"syscall"
	"time"
	"unsafe"
)

const (
	HPS_TO_FPGA_BASE = 0xc0000000

	IN_HPS_BASE     = 0x0
	IN_HPS_SPAN     = 0x10
	SEND_2_HPS      = 0
	BLOCK_SEL_START = 8

	OUT_HPS_BASE = 0x1000
	OUT_HPS_SPAN = 0x10
	STR_DATA_AQ  = 0
	POWER_FEM    = 1

	SRAM_BUF_BASE = 0x20000000
	SRAM_BUF_SPAN = 0x1000000

	PIO_DATA_ADDR     = 0
	PIO_DIR_ADDR      = 4
	PIO_IRQ_MASK_ADDR = 8
	PIO_EDGE_CAP_ADDR = 12

	CHN_COUNT         = 8
	DATA_LEN          = 4
	ADC_COUNT         = 7
	NUM_SAMPLE_BLOCKS = 512
	SAMPLES_PER_BLOCK = 64
	MEM_ADDR_SIZE     = DATA_LEN * CHN_COUNT
	HEADER_SIZE       = 0
	SAMPLE_SIZE       = MEM_ADDR_SIZE * (ADC_COUNT + 1)
	BUF_DATA_SIZE     = SAMPLES_PER_BLOCK * SAMPLE_SIZE
	BUF_BLK_SIZE      = HEADER_SIZE + BUF_DATA_SIZE
)

type FpgaReader struct {
	memFd int

	outMap     []byte
	outDataPtr *byte

	inMap        []byte
	inDataPtr    *uint32
	inIrqMaskPtr *uint32
	inEdgeCapPtr *uint32

	blockMap   []byte
	block      [NUM_SAMPLE_BLOCKS][]byte
	blockSel   int
	blockAvail int
}

func NewFpgaReader() *FpgaReader {
	reader := &FpgaReader{}
	reader.Init()
	return reader
}

func (r *FpgaReader) Init() {
	var err error
	r.memFd, err = syscall.Open("/dev/mem", syscall.O_SYNC|syscall.O_RDWR, 0)
	if err != nil {
		log.Fatal("/dev/mem open:", err)
	}

	r.outMap, err = syscall.Mmap(r.memFd,
		HPS_TO_FPGA_BASE+OUT_HPS_BASE,
		OUT_HPS_SPAN,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		log.Fatal("out map:", err)
	} else {
		r.outDataPtr = (*byte)(unsafe.Pointer(&r.outMap[PIO_DATA_ADDR]))

		*r.outDataPtr = 0
	}

	r.inMap, err = syscall.Mmap(r.memFd,
		HPS_TO_FPGA_BASE+IN_HPS_BASE,
		IN_HPS_SPAN,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		log.Fatal("in map:", err)
	} else {
		r.inDataPtr = (*uint32)(unsafe.Pointer(&r.inMap[PIO_DATA_ADDR]))
		r.inIrqMaskPtr = (*uint32)(unsafe.Pointer(&r.inMap[PIO_IRQ_MASK_ADDR]))
		r.inEdgeCapPtr = (*uint32)(unsafe.Pointer(&r.inMap[PIO_EDGE_CAP_ADDR]))
	}

	r.blockMap, err = syscall.Mmap(r.memFd,
		SRAM_BUF_BASE,
		SRAM_BUF_SPAN,
		syscall.PROT_READ,
		syscall.MAP_SHARED,
	)
	if err != nil {
		log.Fatal("block map:", err)
	} else {
		for i := 0; i < NUM_SAMPLE_BLOCKS; i++ {
			r.block[i] = r.blockMap[i*BUF_BLK_SIZE : (i+1)*BUF_BLK_SIZE]
		}
	}

	*r.outDataPtr |= 1 << POWER_FEM
	log.Println("powered on FEMs")

	time.Sleep(time.Second)

	*r.outDataPtr |= 1 << STR_DATA_AQ
	log.Println("started acq")

	*r.inEdgeCapPtr = 1
	for *r.inEdgeCapPtr&(1<<SEND_2_HPS) == 0 {
		time.Sleep(time.Millisecond)
	}
	r.blockAvail = int(*r.inDataPtr >> BLOCK_SEL_START)
}

func (r *FpgaReader) Close() {
	if err := syscall.Munmap(r.blockMap); err != nil {
		log.Fatal(err)
	}

	*r.inIrqMaskPtr = 0
	if err := syscall.Munmap(r.inMap); err != nil {
		log.Fatal(err)
	}

	*r.outDataPtr = 0
	log.Println("stopped acq and powered off FEMs")
	if err := syscall.Munmap(r.outMap); err != nil {
		log.Fatal(err)
	}

	if err := syscall.Close(r.memFd); err != nil {
		log.Fatal(err)
	}

	r.memFd = 0
}

func (r *FpgaReader) ReadBlock(block []byte) int {
	r.blockSel = (r.blockSel + 1) % NUM_SAMPLE_BLOCKS
	for r.blockSel == r.blockAvail {
		*r.inEdgeCapPtr = 1
		for *r.inEdgeCapPtr&(1<<SEND_2_HPS) == 0 {
			time.Sleep(time.Millisecond)
		}
		r.blockAvail = int(*r.inDataPtr >> BLOCK_SEL_START)
	}

	copy(block, r.block[r.blockSel])

	return r.blockSel
}
