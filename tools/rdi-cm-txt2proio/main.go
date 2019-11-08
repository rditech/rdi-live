// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/rditech/rdi-live/model/rdi/currentmode"

	"github.com/proio-org/go-proio"
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

	var reader *bufio.Scanner
	filename := flag.Arg(1)
	if filename == "-" {
		stdin := bufio.NewReader(os.Stdin)
		reader = bufio.NewScanner(stdin)
	} else {
		var file *os.File
		file, err = os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		reader = bufio.NewScanner(file)
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
	defer writer.Close()

	event := proio.NewEvent()
	var timestamp uint64
	timestamp = (uint64(time.Now().Second()) << 32)
	frame := &currentmode.Frame{Timestamp: timestamp}
	count := 0
	for reader.Scan() {
		tSample := strings.Split(reader.Text(), "\t")

		sample := &currentmode.Sample{
			Hps:       make(map[uint64]*currentmode.HpsSample),
			Timestamp: timestamp - frame.Timestamp,
		}
		sample.Hps[uid] = &currentmode.HpsSample{}
		hpsSample := sample.Hps[uid]
		hpsSample.Channel = make([]int32, len(tSample))

		for i, text := range tSample {
			sampleBytes, err := hex.DecodeString(text)
			if err != nil {
				log.Fatal("failure to decode sample hex text")
			}

			hpsSample.Channel[i] = int32(binary.BigEndian.Uint32(sampleBytes))
		}

		timestamp += 171799
		count++
		if count == 64 {
			event.AddEntry("Frame", frame)
			writer.Push(event)
			count = 0
			event = proio.NewEvent()
			frame.Sample = frame.Sample[:0]
			frame.Timestamp = timestamp
		} else {
			frame.Sample = append(frame.Sample, sample)
		}
	}
}
