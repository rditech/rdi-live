// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"runtime/pprof"
	"runtime/trace"
	"time"

	"github.com/rditech/rdi-live/daq/cyclonev"
	"github.com/rditech/rdi-live/model/rdi/currentmode"
	"github.com/rditech/rdi-live/model/rdi/slowdata"

	"github.com/golang/protobuf/proto"
	"github.com/proio-org/go-proio"
	"github.com/sevlyar/go-daemon"
	"golang.org/x/exp/io/i2c"
	"golang.org/x/net/websocket"
)

var (
	cpuProfile = flag.String("cpuprofile", "", "output file for cpu profiling")
	traceFile  = flag.String("trace", "", "output file for trace")
	daemonize  = flag.Bool("d", false, "daemonize data")
)

func printUsage() {
	fmt.Fprintf(os.Stderr,
		`Usage: `+os.Args[0]+` [options]

Description

options:
`,
	)
	flag.PrintDefaults()
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	if flag.NArg() != 0 {
		printUsage()
		log.Fatal("invalid arguments")
	}

	if *daemonize {
		ctxt := &daemon.Context{}
		d, err := ctxt.Reborn()
		if err != nil {
			log.Fatal("unable to daemonize data:", err)
		}
		if d != nil {
			return
		}
		log.Println("daemon started")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c)

	// enable i2c-1 mux
	enableMux()

	// set resolution for SoM LM73 temp sensor
	d, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-0"}, 0x4c)
	err = d.Write([]byte{0x4, 0x60})
	if err != nil {
		log.Printf("failure to set LM73 resolution: %v", err)
	}

	reader := cyclonev.NewFpgaReader()
	defer reader.Close()

	blocksIn := make(chan []byte, blockBufSize)
	blocksOut := make(chan []byte, blockBufSize)
	go writeBlocks(blocksIn, blocksOut)

	for len(blocksOut) < blockBufSize {
		blocksOut <- make([]byte, cyclonev.BUF_BLK_SIZE)
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal("could not create cpu profile file: ", err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *traceFile != "" {
		f, err := os.Create(*traceFile)
		if err != nil {
			log.Fatal("could not create trace file: ", err)
		}
		trace.Start(f)
		defer trace.Stop()
	}

	lastBlockSel := -1
	for block := range blocksOut {
		blockSel := reader.ReadBlock(block)
		if blockSel != (lastBlockSel+1)%cyclonev.NUM_SAMPLE_BLOCKS && lastBlockSel >= 0 {
			log.Println("non-consecutive blocks")
		}
		lastBlockSel = blockSel

		select {
		case <-c:
			goto wrapup
		case blocksIn <- block:
		}
	}

wrapup:
	close(blocksIn)
	for range blocksOut {
	}

	log.Println("quitting nicely")
}

func writeBlocks(blocksIn <-chan []byte, blocksOut chan<- []byte) {
	uidBytes, err := hex.DecodeString(os.Getenv("HPS_UID"))
	if err != nil {
		log.Fatal("failure to decode UID hex text")
	}

	url := os.Getenv("OUTPUT_URL")
	if len(url) == 0 {
		url = "ws://live.radiationimaging.com/ingress"
	}

	done := make(chan bool, 1)
	push := make(chan *proio.Event, blockBufSize)
	go func() {
		tempData := make(chan []byte)
		go func() {
			defer close(tempData)
			for {
				select {
				case <-done:
					return
				case tempData <- getTempData():
				}
				time.Sleep(time.Second)
			}
		}()

		hvData := make(chan []byte)
		go func() {
			defer close(hvData)
			for {
				select {
				case <-done:
					return
				case hvData <- getHvData():
				}
				time.Sleep(time.Second)
			}
		}()

		var writer *proio.Writer
		tryConn := func() error {
			conn, err := websocket.Dial(url, "", "http://localhost/")
			if err != nil {
				writer = nil
				return err
			}

			writer = proio.NewWriter(conn)
			writer.SetCompression(proio.UNCOMPRESSED)
			writer.BucketDumpThres = 0x1
			writer.DeferUntilClose(conn.Close)
			writer.PushMetadata("UID", uidBytes)

			return nil
		}

		for i := 0; i < 2; i++ {
			err := tryConn()
			if err == nil {
				defer writer.Close()
				goto pushLoop
			}
			log.Println(err)
		}
		goto wrapup

	pushLoop:
		for {
			select {
			case event := <-push:
				if event == nil {
					goto wrapup
				}
				if err := writer.Push(event); err != nil {
					log.Println(err)
					for i := 0; i < 2; i++ {
						err := tryConn()
						if err == nil {
							defer writer.Close()
							goto pushLoop
						}
						log.Println(err)
					}
					goto wrapup
				}
			case buf := <-hvData:
				writer.PushMetadata("HV", buf)
			case buf := <-tempData:
				writer.PushMetadata("Temp", buf)
			}
		}

	wrapup:
		close(done)
	}()

	desc, _ := (&currentmode.HpsSample{}).Descriptor()

	for block := range blocksIn {
		event := proio.NewEvent()

		sampleOff := sampleInitOff
		for i := 0; i < samplesPerBlock; i++ {
			sampleHdr := block[sampleOff : sampleOff+sampleHdrSize]
			sampleOff += sampleHdrSize
			sampleBuf := block[sampleOff : sampleOff+sampleBufSize]
			sampleOff += sampleBufSize

			sampleSize := binary.LittleEndian.Uint32(sampleHdr[4:8])
			sampleNum := binary.LittleEndian.Uint32(sampleHdr[8:12])
			checksum := sampleHdr[12]

			if sampleSize > sampleBufSize {
				log.Println("invalid reported sample size")
				continue
			}

			doublecheck := byte(0)
			for i := uint32(0); i < sampleSize; i++ {
				doublecheck ^= sampleBuf[i]
			}

			if checksum != doublecheck {
				log.Printf(
					"checksum failed! checksum: %v vs doublecheck: %v\n\tsampleSize = %v",
					checksum,
					doublecheck,
					sampleSize,
				)
				continue
			}

			sampleSizeEnc := proto.EncodeVarint(uint64(sampleSize))
			sampleNumEnc := proto.EncodeVarint(uint64(sampleNum))

			sampleEntrySize := 1 + len(sampleSizeEnc) + int(sampleSize) + 1 + len(sampleNumEnc)
			sampleEnc := make([]byte, sampleEntrySize)[:0]

			sampleEnc = append(sampleEnc, 0xa)
			sampleEnc = append(sampleEnc, sampleSizeEnc...)
			sampleEnc = append(sampleEnc, sampleBuf[:sampleSize]...)
			sampleEnc = append(sampleEnc, 0x28)
			sampleEnc = append(sampleEnc, sampleNumEnc...)

			_, err := event.AddSerializedEntry(
				"Sample",
				sampleEnc,
				"rdi.currentmode.HpsSample",
				desc,
			)
			if err != nil {
				log.Println(err)
				goto wrapup
			}
		}

		select {
		case <-done:
			goto wrapup
		case push <- event:
		}

		blocksOut <- block
	}

wrapup:
	close(push)
	<-done
	close(blocksOut)
}

const (
	blockBufSize = 1000

	nChannels       = cyclonev.CHN_COUNT * cyclonev.ADC_COUNT
	sampleInitOff   = cyclonev.HEADER_SIZE
	samplesPerBlock = cyclonev.SAMPLES_PER_BLOCK
	sampleBufSize   = nChannels * cyclonev.DATA_LEN
	sampleHdrSize   = cyclonev.MEM_ADDR_SIZE
)

func enableMux() {
	d1, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x43)
	if err != nil {
		log.Fatalf("FAIL: %v", err)
	}

	err = d1.Write([]byte{0x3, 0xff})
	if err != nil {
		log.Printf("FAIL: %v", err)
	}
	err = d1.Write([]byte{0x5, 0xff})
	if err != nil {
		log.Printf("FAIL: %v", err)
	}
	err = d1.Write([]byte{0x7, 0x0})
	if err != nil {
		log.Printf("FAIL: %v", err)
	}

	d1.Close()
}

func getHvData() []byte {
	hv := &slowdata.Hv{}

	d, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x0e)
	buf := make([]byte, 2)
	d.Read(buf)
	if err == nil {
		val := ((uint32(buf[0]&0xf) << 8) + uint32(buf[1]&0xfc)) >> 2
		hv.DacValue = append(hv.DacValue, val)
	}
	d.Close()

	buf, _ = proto.Marshal(hv)
	return buf
}

func getTempData() []byte {
	t := &slowdata.Temp{}

	d, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-0"}, 0x4c)
	err = d.Write([]byte{0x0})
	if err != nil {
		log.Printf("FAIL: %v", err)
	}
	buf := make([]byte, 2)
	d.Read(buf)
	if err == nil {
		major := float64(int8(buf[0])) * 2.0
		minor := math.Copysign(float64(uint8(buf[1]))/128.0, major)
		t.Som = append(t.Som, float32(major+minor))
	}
	d.Close()

	d, err = i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x40)
	err = d.Write([]byte{0xe3})
	if err != nil {
		log.Printf("FAIL: %v", err)
	}
	buf = make([]byte, 2)
	d.Read(buf)
	if err == nil {
		major := uint16(buf[0]) << 8
		minor := uint16(buf[1])
		t.Fem = append(t.Fem, float32(major+minor)*175.72/(1<<16)-46.85)
	}
	d.Close()

	buf, _ = proto.Marshal(t)
	return buf
}
