// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/proio-org/go-proio"
	"github.com/rditech/rdi-live/model/rdi/currentmode"
)

var (
	outFile   = flag.String("o", "", "file to save output to")
	compLevel = flag.Int("c", 1, "output compression level: 0 for uncompressed, 1 for LZ4 compression, 2 for GZIP compression, 3 for LZMA compression")
)

func printUsage() {
	fmt.Fprintf(os.Stderr,
		`Usage: `+os.Args[0]+` [options] <UID> <input-file>

Description

options:
`,
	)
	flag.PrintDefaults()
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	if flag.NArg() != 2 {
		printUsage()
		log.Fatal("Invalid arguments")
	}

	uidBytes, err := hex.DecodeString(flag.Arg(0))
	if err != nil {
		log.Fatal("failure to decode UID hex text")
	}
	uid := binary.BigEndian.Uint64(uidBytes)

	var reader io.Reader
	filename := flag.Arg(1)
	if filename == "-" {
		reader = os.Stdin
	} else {
		var file *os.File
		file, err = os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		reader = file
	}

	var writer *proio.Writer
	if *outFile == "" {
		writer = proio.NewWriter(os.Stdout)
	} else {
		writer, err = proio.Create(*outFile)
		if err != nil {
			log.Fatal(err)
		}
	}
	switch *compLevel {
	case 3:
		writer.SetCompression(proio.LZMA)
	case 2:
		writer.SetCompression(proio.GZIP)
	case 1:
		writer.SetCompression(proio.LZ4)
	default:
		writer.SetCompression(proio.UNCOMPRESSED)
	}
	writer.BucketDumpThres = 0x10000
	defer writer.Close()

	blocks := make(chan []byte, 1000)
	defer close(blocks)
	go func() {
		var offsets map[uint64]*currentmode.HpsSample
		timestamp := (uint64(time.Now().Second()) << 32)
		event := proio.NewEvent()
		for blockBuf := range blocks {
			//timeS := binary.LittleEndian.Uint32(blockBuf[:4])
			//timeNs := binary.LittleEndian.Uint32(blockBuf[4:8])
			checksum := int32(binary.LittleEndian.Uint32(blockBuf[8:12]))
			var doubleChecksum int32

			if offsets == nil {
				hpsOffsets := &currentmode.HpsSample{Channel: make([]int32, nChannels)}
				offsets = make(map[uint64]*currentmode.HpsSample)
				offsets[uid] = hpsOffsets

				offsetBuf := blockBuf[offsetsOff:sampleInitOff]
				for i := 0; i < nChannels; i++ {
					bufStart := i * 4
					value := int32(binary.LittleEndian.Uint32(offsetBuf[bufStart : bufStart+4]))
					hpsOffsets.Channel[i] = value
				}
			}

			frame := &currentmode.Frame{
				Timestamp: timestamp,
				Sample:    make([]*currentmode.Sample, samplesPerBlock),
				Offsets:   offsets,
			}

			sampleOff := sampleInitOff
			for sampleNum := 0; sampleNum < samplesPerBlock; sampleNum++ {
				sampleBuf := blockBuf[sampleOff : sampleOff+sampleBufSize]
				sampleOff += sampleBufSize

				sample := &currentmode.Sample{
					Hps:       make(map[uint64]*currentmode.HpsSample),
					Timestamp: timestamp - frame.Timestamp,
				}
				sample.Hps[uid] = &currentmode.HpsSample{
					Channel: make([]int32, nChannels),
					Sum:     int64(int32(binary.LittleEndian.Uint32(sampleBuf[:4]))),
				}
				hpsSample := sample.Hps[uid]

				for i := 0; i < nChannels; i++ {
					bufStart := (i + 1) * 4
					value := int32(binary.LittleEndian.Uint32(sampleBuf[bufStart : bufStart+4]))
					doubleChecksum ^= value
					hpsSample.Channel[i] = value
				}

				frame.Sample[sampleNum] = sample

				timestamp += 171799
			}

			if checksum == doubleChecksum {
				event.AddEntry("Frame", frame)
				if len(event.AllEntries()) == 4 {
					writer.Push(event)
					event = proio.NewEvent()
				}
			} else {
				log.Println("Incorrect checksum")
			}
		}
	}()

	magicByteBuf := make([]byte, 1)
	for {
		nRead := 0
		for {
			if _, err := io.ReadFull(reader, magicByteBuf); err != nil {
				log.Println(err)
				break
			}
			nRead++

			if magicByteBuf[0] == magicBytes[0] {
				var goodSeq = true
				for i := 1; i < len(magicBytes); i++ {
					if _, err := io.ReadFull(reader, magicByteBuf); err != nil {
						log.Println(err)
						break
					}
					nRead++

					if magicByteBuf[0] != magicBytes[i] {
						goodSeq = false
						break
					}
				}

				if goodSeq {
					break
				}
			}
		}
		if nRead < 8 {
			break
		}

		blockBuf := make([]byte, blockSize)
		if _, err := io.ReadFull(reader, blockBuf); err != nil {
			log.Println("Unable to read block:", err)
			break
		}
		blocks <- blockBuf
	}
}

var magicBytes = [...]byte{
	byte(0x71),
	byte(0x71),
	byte(0x97),
	byte(0xca),
	byte(0x45),
	byte(0xfd),
	byte(0x02),
	byte(0xab),
}

const (
	nChannels       = 96
	blockSize       = 25228
	offsetsOff      = 12
	offsetsBufSize  = nChannels * 4
	sampleInitOff   = offsetsOff + offsetsBufSize
	samplesPerBlock = 64
	sampleBufSize   = (nChannels + 1) * 4
)
