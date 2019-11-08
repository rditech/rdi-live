// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"math"

	"github.com/rditech/rdi-live/model/rdi/currentmode"

	"github.com/proio-org/go-proio"
)

type Pedestals struct {
	Alpha   float64
	CovFrac float64
	values  [][]float64
}

func (p *Pedestals) Subtract(input <-chan *proio.Event, output chan<- *proio.Event) {
	if p.Alpha == 0 {
		p.Alpha = 0.0001
	}
	inv_alpha := 1 - p.Alpha

	if p.CovFrac == 0 {
		p.CovFrac = 0.1
	}
	covFrac2 := p.CovFrac * p.CovFrac

	for event := range input {
		rawFrameIds := event.TaggedEntries("Frame")
		mappedFrameIds := event.TaggedEntries("Mapped")
		if len(mappedFrameIds) != len(rawFrameIds) {
			continue
		}

		for i, entryId := range mappedFrameIds {
			frame, ok := event.GetEntry(entryId).(*currentmode.Frame)
			if !ok {
				continue
			}
			rawFrame, ok := event.GetEntry(rawFrameIds[i]).(*currentmode.Frame)

			// if the axis offsets already exist in the stream, assume that
			// they were taken care of in the detector mapping, and do nothing
			if rawFrame.AxisOffsets != nil {
				continue
			}

			nSamples := len(frame.Sample)
			if nSamples == 0 {
				continue
			}

			nAxes := len(frame.Sample[0].Axis)
			thres := float32(math.Pow(covFrac2, float64(nAxes*(nAxes-1)/2)))

			for sampleNum, sample := range frame.Sample {
				for i, axis := range sample.Axis {
					axis.Sum = 0

					if len(p.values) <= i {
						p.values = append(p.values, make([]float64, 0))
					}

					for j, val := range axis.FloatChannel {
						if len(p.values[i]) <= j {
							p.values[i] = append(p.values[i], 0)
						}

						if frame.Correlation < thres {
							p.values[i][j] *= inv_alpha
							p.values[i][j] += p.Alpha * float64(val)
						}

						axis.FloatChannel[j] -= float32(p.values[i][j])
						axis.Sum += axis.FloatChannel[j]
					}
				}

				if sampleNum == 0 {
					rawFrame.AxisOffsets = make([]*currentmode.AxisSample, len(sample.Axis))
					for i := 0; i < len(rawFrame.AxisOffsets); i++ {
						axis := &currentmode.AxisSample{}
						rawFrame.AxisOffsets[i] = axis
						axis.FloatChannel = make([]float32, len(sample.Axis[i].FloatChannel))
						for j := range axis.FloatChannel {
							axis.FloatChannel[j] = float32(p.values[i][j])
						}
					}
				}

			}
		}

		output <- event
	}
}
