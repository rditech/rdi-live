// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package live

import (
	"fmt"

	"github.com/rditech/rdi-live/data"
	"github.com/rditech/rdi-live/model/rdi/currentmode"
	detmapmodel "github.com/rditech/rdi-live/model/rdi/detmap"

	"github.com/go-redis/redis"
	"github.com/proio-org/go-proio"
)

func BuildPlayer(
	namespace, stream string,
	client *redis.Client,
	addr string,
	uid uint64,
) data.OpArray {
	player := &data.Player{Speed: 1}
	var sp data.StreamProcessor
	switch data.GetMode(uid) {
	case detmapmodel.HpsConfig_CURRENT:
		sp = player.PlayCmStream
	default:
	}
	if sp == nil {
		return nil
	}

	ops := BuildOpArray(namespace, stream, client, addr, uid)
	if ops == nil {
		return nil
	}
	ops = append(
		data.OpArray{
			data.StreamOp{
				StreamProcessor: sp,
			},
		},
		ops...,
	)
	return ops
}

func BuildOpArray(
	namespace, stream string,
	client *redis.Client,
	addr string,
	uid uint64,
) data.OpArray {
	var ops data.OpArray
	switch data.GetMode(uid) {
	case detmapmodel.HpsConfig_CURRENT:
		ops = BuildCmOpArray(namespace, stream, client, addr, uid)
	default:
	}
	return ops
}

func BuildCmOpArray(namespace, stream string, client *redis.Client, addr string, uid uint64) data.OpArray {
	corr := &data.Correlator{}
	peds := &data.Pedestals{}
	recon := data.NewBeamReconstruction(uid)
	streamManager := StreamManager{
		Namespace:       namespace,
		Name:            stream,
		Redis:           client,
		Addr:            addr,
		GenerateSources: CmGenerateSources,
		CleanupRunData: []data.EventProcessor{
			data.KeepOnlyRawFrames,
		},
	}

	cmAggregateOps := append(
		data.OpArray{
			data.EventOp{
				EventProcessor: data.AssembleFrame,
				Concurrency:    16,
				MaxEventBuf:    1,
			},
		},
		data.StreamOp{
			Description:     "Merge current-mode stream aggregate",
			StreamProcessor: data.CmMerge,
			MaxEventBuf:     1,
		},
		data.EventOp{
			EventProcessor: data.MapEvent,
			Concurrency:    16,
			MaxEventBuf:    1,
		},
		data.StreamOp{
			StreamProcessor: corr.CorrelateCmEvent,
			MaxEventBuf:     1,
		},
		data.StreamOp{
			StreamProcessor: peds.Subtract,
			MaxEventBuf:     1,
		},
		data.EventOp{
			EventProcessor: recon.FillBeamInfo,
			Concurrency:    16,
			MaxEventBuf:    1,
		},
		data.StreamOp{
			StreamProcessor: streamManager.Manage,
			MaxEventBuf:     1000,
		},
	)

	return cmAggregateOps
}

var one = float32(1)

func CmGenerateSources(m *StreamManager, event *proio.Event) {
	totalCurrentInfo := m.GetSourceInfo("Total Current")
	correlationInfo := m.GetSourceInfo("Correlation")
	axisCurrentInfoCache := make(map[int]*SourceInfo)
	axisChannelsInfoCache := make(map[int]*SourceInfo)
	axisChannelInfoCache := make(map[int]*SourceInfo)

	for _, frameId := range event.TaggedEntries("Mapped") {
		frame, ok := event.GetEntry(frameId).(*currentmode.Frame)
		if !ok {
			continue
		}

		tFrame := float64(frame.Timestamp) / (1 << 32)

		m.HandleSource(correlationInfo, Normal, &tFrame, &frame.Correlation)

		for _, sample := range frame.Sample {
			tSample := tFrame + float64(sample.Timestamp)/(1<<32)

			var totI float32
			for axis, axisSample := range sample.Axis {
				totI += axisSample.Sum

				axisCurrent := axisCurrentInfoCache[axis]
				if axisCurrent == nil {
					axisCurrent = m.GetSourceInfo(fmt.Sprintf("Axis %d Current", axis))
					axisCurrentInfoCache[axis] = axisCurrent
				}
				m.HandleSource(axisCurrent, Normal, &tSample, &axisSample.Sum)

				axisChannels := axisChannelsInfoCache[axis]
				if axisChannels == nil {
					axisChannels = m.GetSourceInfo(fmt.Sprintf("Axis %d Channels", axis))
					axisChannelsInfoCache[axis] = axisChannels
				}
				m.HandleSource(axisChannels, Normal, axisSample.FloatChannel)

				for axisChan, chanVal := range axisSample.FloatChannel {
					axisChanCurrIndex := (1<<16)*axis + axisChan
					axisChanCurr := axisChannelInfoCache[axisChanCurrIndex]
					if axisChanCurr == nil {
						axisChanCurr = m.GetSourceInfo(fmt.Sprintf("Axis %d Chan %03d Current", axis, axisChan))
						axisChannelInfoCache[axisChanCurrIndex] = axisChanCurr
					}
					m.HandleSource(axisChanCurr, Advanced, &tSample, &chanVal)
				}
			}

			m.HandleSource(totalCurrentInfo, Normal, &tSample, &totI)
		}
	}

	meanXInfo := m.GetSourceInfo("Mean X")
	meanYInfo := m.GetSourceInfo("Mean Y")
	meanXYInfo := m.GetSourceInfo("Mean XY")
	meanAndTotalI := m.GetSourceInfo("Mean and Total Current")
	for _, frameId := range event.TaggedEntries("Reduced") {
		frame, ok := event.GetEntry(frameId).(*currentmode.Frame)
		if !ok {
			continue
		}

		tFrame := float64(frame.Timestamp) / (1 << 32)

		for _, sample := range frame.Sample {
			tSample := tFrame + float64(sample.Timestamp)/(1<<32)

			m.HandleSource(meanXInfo, Normal, &tSample, &sample.BeamInfo.MeanXPos)
			m.HandleSource(meanYInfo, Normal, &tSample, &sample.BeamInfo.MeanYPos)
			m.HandleSource(meanXYInfo, Normal, &sample.BeamInfo.MeanXPos, &sample.BeamInfo.MeanYPos)
			m.HandleSource(meanAndTotalI, Normal,
				&sample.BeamInfo.MeanXPos,
				&sample.BeamInfo.MeanYPos,
				&sample.BeamInfo.TotalCurrent,
			)
		}
	}
}
