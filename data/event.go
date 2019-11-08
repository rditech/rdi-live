// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"github.com/proio-org/go-proio"
)

type EventProcessor func(*proio.Event)

type EventOp struct {
	Description    string
	EventProcessor EventProcessor
	Concurrency    int
	MaxEventBuf    int
}

func (o EventOp) GetDescription() string {
	return o.Description
}

func (o EventOp) Run(input <-chan *proio.Event) <-chan *proio.Event {
	if o.Concurrency == 0 {
		o.Concurrency = *concurrency
	}

	if o.MaxEventBuf == 0 {
		o.MaxEventBuf = *maxEventBuf
	}

	output := make(chan *proio.Event, o.MaxEventBuf)

	go func() {
		defer close(output)

		procEvents := make(map[uint64]*proio.Event)
		doneEvents := make(map[uint64]*proio.Event)
		done := make(chan uint64)
		ackDone := func() {
			index := <-done
			doneEvents[index] = procEvents[index]
			delete(procEvents, index)
		}
		defer close(done)

		nRead := uint64(0)
		nWritten := uint64(0)
		writeOut := func() {
			for {
				if event, ok := doneEvents[nWritten]; ok {
					output <- event
					delete(doneEvents, nWritten)
					nWritten++
				} else {
					break
				}
			}
		}

		for event := range input {
			go func(event *proio.Event, done chan<- uint64, index uint64) {
				o.EventProcessor(event)
				done <- index
			}(event, done, nRead)
			procEvents[nRead] = event
			nRead++

			for len(procEvents) >= o.Concurrency || len(doneEvents) >= o.MaxEventBuf {
				ackDone()
				writeOut()
			}
		}

		for len(procEvents) > 0 {
			ackDone()
		}
		writeOut()
	}()

	return output
}
