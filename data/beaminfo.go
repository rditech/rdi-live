// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"log"

	"github.com/rditech/rdi-live/model/rdi/currentmode"
	detmapmodel "github.com/rditech/rdi-live/model/rdi/detmap"

	"github.com/proio-org/go-proio"
	"gonum.org/v1/gonum/mat"
)

type BeamReconstruction struct {
	hpsConfig        *detmapmodel.HpsConfig
	linEstT, meanPos *mat.Dense
}

func NewBeamReconstruction(uid uint64) *BeamReconstruction {
	r := &BeamReconstruction{
		hpsConfig: GetHpsConfig(uid),
	}

	imageConfs := GetImageConfigs(uid)
	if len(imageConfs) > 0 {
		imageConf := imageConfs[0]
		if len(imageConf.LinEstT) > 0 {
			r.linEstT = mat.NewDense(len(imageConf.LinEstT), len(imageConf.LinEstT[0].Array), nil)
			for i, row := range imageConf.LinEstT {
				for j, val := range row.Array {
					r.linEstT.Set(i, j, float64(val))
				}
			}
			pos := mat.NewDense(2, len(imageConf.XPos), nil)
			if len(imageConf.XPos) == len(imageConf.YPos) {
				for j, val := range imageConf.XPos {
					pos.Set(0, j, float64(val))
				}
				for j, val := range imageConf.YPos {
					pos.Set(1, j, float64(val))
				}
			}
			r.meanPos = &mat.Dense{}
			r.meanPos.Mul(pos, r.linEstT.T())
		}
	}

	return r
}

func (r *BeamReconstruction) FillBeamInfo(event *proio.Event) {
	if r.meanPos == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in BeamReconstruction.FillBeamInfo:", r)
		}
	}()

	_, c := r.meanPos.Dims()

	for _, entryId := range event.TaggedEntries("Mapped") {
		frame, ok := event.GetEntry(entryId).(*currentmode.Frame)
		if !ok {
			continue
		}

		reducedFrame := &currentmode.Frame{
			Timestamp: frame.Timestamp,
		}

		for _, sample := range frame.Sample {
			reducedSample := &currentmode.Sample{
				Timestamp: sample.Timestamp,
				BeamInfo:  &currentmode.Sample_BeamInfo{},
			}
			reducedFrame.Sample = append(reducedFrame.Sample, reducedSample)

			qVec := mat.NewVecDense(c, nil)
			for i := 0; i < c; i++ {
				chanConfig := r.hpsConfig.Channel[uint32(i)]
				if chanConfig != nil {
					val := float64(sample.Axis[chanConfig.Axis].FloatChannel[chanConfig.AxisChannel])
					qVec.SetVec(i, val)
				}
			}

			beamInfo := reducedSample.BeamInfo

			sum := mat.Sum(qVec)
			if sum != 0 {
				qVec.ScaleVec(1.0/sum, qVec)
				var pos mat.VecDense
				pos.MulVec(r.meanPos, qVec)
				beamInfo.MeanXPos = float32(pos.AtVec(0))
				beamInfo.MeanYPos = float32(pos.AtVec(1))
				beamInfo.TotalCurrent = float32(sum)
			}
		}

		event.AddEntry("Reduced", reducedFrame)
	}
}
