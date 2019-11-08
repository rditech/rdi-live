// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/proio-org/go-proio"
	"golang.org/x/net/websocket"
)

type Op interface {
	GetDescription() string
	Run(input <-chan *proio.Event) <-chan *proio.Event
}

type OpArray []Op

func (ops OpArray) Run(stream <-chan *proio.Event) <-chan *proio.Event {
	for _, o := range ops {
		stream = o.Run(stream)
	}
	return stream
}

func (ops OpArray) Sink(stream <-chan *proio.Event) {
	for range ops.Run(stream) {
	}
}

var FlagSet = flag.NewFlagSet("", flag.ExitOnError)

var (
	outFile     = FlagSet.String("o", "", "file to save output to")
	compLevel   = FlagSet.Int("c", 1, "output compression level: 0 for uncompressed, 1 for LZ4 compression, 2 for GZIP compression, 3 for LZMA compression")
	readBufSize = FlagSet.Int("b", 10, "read buffer size in number of events")
	concurrency = FlagSet.Int("t", 1, "level of concurrency")
	maxEventBuf = FlagSet.Int("e", 200, "max event buffer for maintaining event order")
	bucketThres = FlagSet.Int("d", 0x10000, "bucket dump threshold in bytes")
	loop        = FlagSet.Bool("l", false, "infinite loop over data")
	cpuProfile  = FlagSet.String("cpuprofile", "", "output file for cpu profiling")
	memProfile  = FlagSet.String("memprofile", "", "output file for memory profiling")
)

func (ops OpArray) RunCmdFlagParse() {
	var desc string
	for i, o := range ops {
		desc += strconv.Itoa(i) + ") "
		desc += o.GetDescription()
		if i < len(ops)-1 {
			desc += "\n"
		}
	}

	FlagSet.Usage = func() {
		fmt.Fprintf(os.Stderr,
			`Usage: `+os.Args[0]+` [options] <proio-input-file>

`+desc+`

options:
`,
		)
		FlagSet.PrintDefaults()
	}
	FlagSet.Parse(os.Args[1:])

	if FlagSet.NArg() != 1 {
		FlagSet.Usage()
		log.Fatal("Invalid arguments")
	}
}

func (ops OpArray) GetReader() *proio.Reader {
	ops.RunCmdFlagParse()

	var reader *proio.Reader
	var err error
	filename := FlagSet.Arg(0)
	if filename == "-" {
		stdin := bufio.NewReader(os.Stdin)
		reader = proio.NewReader(stdin)
	} else {
		reader, err = proio.Open(filename)
	}
	if err != nil {
		log.Fatal(err)
	}

	return reader
}

func (ops OpArray) RunCmd() {
	ops.RunCmdFlagParse()

	reader := ops.GetReader()
	defer reader.Close()

	var writer *proio.Writer
	var conn *websocket.Conn
	var err error
	if strings.HasPrefix(*outFile, "ws") && strings.Contains(*outFile, "://") {
		conn, err = websocket.Dial(*outFile, "", "http://localhost/")
		if err != nil {
			log.Fatal(err)
		}
		writer = proio.NewWriter(conn)
		writer.DeferUntilClose(conn.Close)
	} else if *outFile == "" {
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
	writer.BucketDumpThres = *bucketThres
	defer writer.Close()

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal("could not create cpu profile file: ", err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	for {
		stream := ops.Run(reader.ScanEvents(*readBufSize))
		for event := range stream {
			if conn != nil {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			}
			if err := writer.Push(event); err != nil {
				goto wrapup
			}
		}

		if reader.Err == io.EOF {
			if !*loop {
				break
			}
			reader.SeekToStart()
		} else {
			log.Print(reader.Err)
		}
	}

wrapup:
	if *memProfile != "" {
		f, err := os.Create(*memProfile)
		if err != nil {
			log.Fatal(err)
		}
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
		f.Close()
	}
}
