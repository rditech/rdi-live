// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"github.com/rditech/rdi-live/model/rdi/currentmode"

	"github.com/proio-org/go-proio"
)

func CorrelateCmEvent(event *proio.Event) {
	for _, entryId := range event.TaggedEntries("Mapped") {
		frame, ok := event.GetEntry(entryId).(*currentmode.Frame)
		if !ok {
			continue
		}

		nSamples := len(frame.Sample)
		if nSamples == 0 {
			continue
		}

		nAxes := len(frame.Sample[0].Axis)
		sum := make([]float64, nAxes)
		prodSum := make([][]float64, nAxes)
		cov := make([][]float64, nAxes)
		for i := 0; i < nAxes; i++ {
			prodSum[i] = make([]float64, nAxes)
			cov[i] = make([]float64, nAxes)
		}

		for i := 0; i < nSamples; i++ {
			sample := frame.Sample[i]
			for j := 0; j < nAxes; j++ {
				axisJSum := float64(sample.Axis[j].Sum)
				sum[j] += axisJSum
				for k := j; k < nAxes; k++ {
					axisKSum := float64(sample.Axis[k].Sum)
					prodSum[j][k] += axisJSum * axisKSum
				}
			}
		}

		for i := 0; i < nAxes; i++ {
			for j := i; j < nAxes; j++ {
				cov[i][j] = prodSum[i][j] - sum[i]*sum[j]/float64(nSamples)
			}
		}

		corr := float64(1.0)
		for i := 0; i < nAxes; i++ {
			for j := i + 1; j < nAxes; j++ {
				corr *= cov[i][j] * cov[i][j] / (cov[i][i] * cov[j][j])
			}
		}

		frame.Correlation = float32(corr)
	}
}

type Correlator struct {
	NFrames int
	Default float32
}

func (c *Correlator) CorrelateCmEvent(input <-chan *proio.Event, output chan<- *proio.Event) {
	i := 0
	for event := range input {
		for _, entryId := range event.TaggedEntries("Mapped") {
			i++

			frame, ok := event.GetEntry(entryId).(*currentmode.Frame)
			if !ok {
				continue
			}

			if c.NFrames == 0 || i <= c.NFrames || (c.NFrames < 0 && i > -c.NFrames) {
				nSamples := len(frame.Sample)
				if nSamples == 0 {
					continue
				}

				nAxes := len(frame.Sample[0].Axis)
				sum := make([]float64, nAxes)
				prodSum := make([][]float64, nAxes)
				cov := make([][]float64, nAxes)
				for i := 0; i < nAxes; i++ {
					prodSum[i] = make([]float64, nAxes)
					cov[i] = make([]float64, nAxes)
				}

				for i := 0; i < nSamples; i++ {
					sample := frame.Sample[i]
					for j := 0; j < nAxes; j++ {
						axisJSum := float64(sample.Axis[j].Sum)
						sum[j] += axisJSum
						for k := j; k < nAxes; k++ {
							axisKSum := float64(sample.Axis[k].Sum)
							prodSum[j][k] += axisJSum * axisKSum
						}
					}
				}

				for i := 0; i < nAxes; i++ {
					for j := i; j < nAxes; j++ {
						cov[i][j] = prodSum[i][j] - sum[i]*sum[j]/float64(nSamples)
					}
				}

				corr := float64(1.0)
				for i := 0; i < nAxes; i++ {
					for j := i + 1; j < nAxes; j++ {
						corr *= cov[i][j] * cov[i][j] / (cov[i][i] * cov[j][j])
					}
				}

				frame.Correlation = float32(corr)
				continue
			}

			frame.Correlation = c.Default
		}

		output <- event
	}
}
