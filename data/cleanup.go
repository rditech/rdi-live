// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"github.com/proio-org/go-proio"
)

func KeepOnlyRawFrames(event *proio.Event) {
	rawFrameIds := event.TaggedEntries("Frame")
	for _, frameId := range event.AllEntries() {
		isRaw := false
		for _, rawFrameId := range rawFrameIds {
			if frameId == rawFrameId {
				isRaw = true
				break
			}
		}
		if !isRaw {
			event.RemoveEntry(frameId)
		}
	}
}

func RemoveLooseSamples(event *proio.Event) {
	for _, sampleId := range event.TaggedEntries("Sample") {
		event.RemoveEntry(sampleId)
	}
}
