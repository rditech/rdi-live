// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"github.com/proio-org/go-proio"
)

type StreamProcessor func(<-chan *proio.Event, chan<- *proio.Event)

type StreamOp struct {
	Description     string
	StreamProcessor StreamProcessor
	MaxEventBuf     int
}

func (o StreamOp) GetDescription() string {
	return o.Description
}

func (o StreamOp) Run(input <-chan *proio.Event) <-chan *proio.Event {
	if o.MaxEventBuf == 0 {
		o.MaxEventBuf = *maxEventBuf
	}

	output := make(chan *proio.Event, o.MaxEventBuf)

	go func() {
		defer close(output)

		o.StreamProcessor(input, output)
	}()

	return output
}
