// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"encoding/binary"
	"log"

	"github.com/rditech/rdi-live/model/rdi/currentmode"

	"github.com/proio-org/go-proio"
)

func AssembleFrame(event *proio.Event) {
	if len(event.Metadata["UID"]) < 8 {
		return
	}
	uid := binary.BigEndian.Uint64(event.Metadata["UID"])

	frame := &currentmode.Frame{}
	for i, sampleId := range event.TaggedEntries("Sample") {
		hpsSample, ok := event.GetEntry(sampleId).(*currentmode.HpsSample)
		if !ok {
			if event.Err != nil {
				log.Println(event.Err)
				event.Err = nil
			}
			continue
		}

		sampleTs := uint64(hpsSample.SampleNumber) * 171799
		if i == 0 {
			frame.Timestamp = sampleTs
		}

		sample := &currentmode.Sample{
			Timestamp: sampleTs - frame.Timestamp,
			Hps:       make(map[uint64]*currentmode.HpsSample),
		}
		sample.Hps[uid] = hpsSample

		frame.Sample = append(frame.Sample, sample)
	}

	if len(frame.Sample) > 0 {
		event.AddEntry("Frame", frame)
	}
}
