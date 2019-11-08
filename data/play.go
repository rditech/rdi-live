// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"log"
	"math"
	"time"

	"github.com/rditech/rdi-live/model/rdi/currentmode"

	"github.com/proio-org/go-proio"
)

const subsecdiv = float64(1 << 32)

type Player struct {
	Speed float64
}

func (p *Player) PlayCmStream(input <-chan *proio.Event, output chan<- *proio.Event) {
	if p.Speed == 0.0 {
		p.Speed = 1.0
	}
	durationScale := 1.0 / p.Speed

	var start time.Time
	initStamp := uint64(math.MaxUint64)
	lastStamp := uint64(math.MaxUint64)

	for event := range input {
		var earliest uint64
		earliest = math.MaxUint64
		for _, frameId := range event.TaggedEntries("Frame") {
			frame, ok := event.GetEntry(frameId).(*currentmode.Frame)
			if !ok {
				log.Println(event.Err)
				return
			}
			if frame.Timestamp < earliest {
				earliest = frame.Timestamp
			}
		}
		var stampDiff float64
		if earliest < lastStamp {
			start = time.Now()
			initStamp = earliest
		} else {
			stampDiff = durationScale * float64(earliest-initStamp) / subsecdiv
		}
		lastStamp = earliest
		relTime := time.Duration(uint64(stampDiff))*time.Second +
			time.Duration(uint64(math.Mod(stampDiff, 1.0)*1e9))*time.Nanosecond
		time.Sleep(time.Until(start.Add(relTime)))

		output <- event
	}
}
