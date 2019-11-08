// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"log"
	"sync"

	"github.com/rditech/rdi-live/model/rdi/currentmode"
	detmapmodel "github.com/rditech/rdi-live/model/rdi/detmap"

	"github.com/gobuffalo/packr"
	"github.com/golang/protobuf/proto"
	"github.com/proio-org/go-proio"
)

var DetmapBox = packr.NewBox("../detmap/full")

var detmapMutex sync.RWMutex
var detmapBytes []byte
var detmap *detmapmodel.Map

func unmarshalDetmap() {
	detmapMutex.Lock()
	var err error
	detmapBytes, err = DetmapBox.Find("dev.pb")
	if err != nil {
		log.Println("cannot find dev.pb:", err)
	}
	detmap = &detmapmodel.Map{}
	proto.Unmarshal(detmapBytes, detmap)
	detmapMutex.Unlock()
}

func MapEvent(event *proio.Event) {
	detmapMutex.RLock()
	if detmap == nil {
		detmapMutex.RUnlock()
		unmarshalDetmap()
		detmapMutex.RLock()
	}

	for _, entryId := range event.TaggedEntries("Mapped") {
		event.RemoveEntry(entryId)
	}

	for _, entryId := range event.TaggedEntries("Frame") {
		frame, ok := event.GetEntry(entryId).(*currentmode.Frame)
		if !ok {
			continue
		}

		mappedFrame := &currentmode.Frame{
			Timestamp: frame.Timestamp,
		}

		for _, sample := range frame.Sample {
			mappedSample := &currentmode.Sample{
				Timestamp: sample.Timestamp,
				Hps:       make(map[uint64]*currentmode.HpsSample),
			}
			mappedFrame.Sample = append(mappedFrame.Sample, mappedSample)

			for hpsId, hpsSample := range sample.Hps {
				hpsConfig := detmap.HpsConfig[uint32(hpsId>>32)]
				if hpsConfig == nil {
					continue
				}
				hpsCalib := detmap.HpsCalibration[uint32(hpsId)]

				mapChan := func(hpsChanNum int, val float32) {
					var currentConv *float32
					if hpsCalib != nil && hpsChanNum < len(hpsCalib.CurrentConv) {
						currentConv = &hpsCalib.CurrentConv[hpsChanNum]
					} else {
						currentConv = &hpsConfig.CurrentConv
					}

					chanConfig := hpsConfig.Channel[uint32(hpsChanNum)]
					if chanConfig == nil {
						return
					}
					for chanConfig.Axis >= uint32(len(mappedSample.Axis)) {
						mappedSample.Axis = append(mappedSample.Axis, &currentmode.AxisSample{})
					}

					axis := mappedSample.Axis[chanConfig.Axis]
					axisChan := chanConfig.AxisChannel
					if axisChan >= uint32(len(axis.FloatChannel)) {
						axis.FloatChannel = append(
							axis.FloatChannel,
							make([]float32, axisChan-uint32(len(axis.FloatChannel))+1)...,
						)
					}
					axis.FloatChannel[axisChan] = val * *currentConv

					axisNum := chanConfig.Axis
					if len(frame.AxisOffsets) > int(axisNum) {
						axisOffsets := frame.AxisOffsets[axisNum]
						if len(axisOffsets.FloatChannel) > int(axisChan) {
							axis.FloatChannel[axisChan] -= axisOffsets.FloatChannel[axisChan]
						}
					}

					axis.Sum += axis.FloatChannel[axisChan]
				}

				for i, val := range hpsSample.Channel {
					mapChan(i, float32(val))
				}

				if len(hpsSample.Channel) == 0 {
					for i, val := range hpsSample.FixedChannel {
						mapChan(i, float32(val))
					}
				}
			}
		}

		event.AddEntry("Mapped", mappedFrame)
	}

	detmapMutex.RUnlock()
}

func GetHpsConfig(uid uint64) *detmapmodel.HpsConfig {
	detmapMutex.RLock()
	defer detmapMutex.RUnlock()
	if detmap == nil {
		detmapMutex.RUnlock()
		unmarshalDetmap()
		detmapMutex.RLock()
	}

	configId := uint32(uid >> 32)
	hpsConfig, ok := detmap.HpsConfig[configId]
	if !ok {
		return detmap.HpsConfig[1]
	}
	return hpsConfig
}

func GetMode(uid uint64) detmapmodel.HpsConfig_Mode {
	detmapMutex.RLock()
	defer detmapMutex.RUnlock()
	if detmap == nil {
		detmapMutex.RUnlock()
		unmarshalDetmap()
		detmapMutex.RLock()
	}

	configId := uint32(uid >> 32)
	hpsConfig, ok := detmap.HpsConfig[configId]
	if !ok {
		return detmapmodel.HpsConfig_CURRENT
	}
	return hpsConfig.Mode
}

func GetDetName(uid uint64) string {
	detmapMutex.RLock()
	defer detmapMutex.RUnlock()
	if detmap == nil {
		detmapMutex.RUnlock()
		unmarshalDetmap()
		detmapMutex.RLock()
	}

	configId := uint32(uid >> 32)
	hpsConfig, ok := detmap.HpsConfig[configId]
	if !ok {
		log.Println("didn't find HpsConfig:", configId)
		return ""
	}
	detConfig, ok := detmap.DetConfig[hpsConfig.DetConfig]
	if !ok {
		log.Println("didn't find DetConfig:", hpsConfig.DetConfig)
		return ""
	}
	return detConfig.Name
}

func GetImageConfigs(uid uint64) []*detmapmodel.DetectorConfig_ImageConfig {
	detmapMutex.RLock()
	defer detmapMutex.RUnlock()
	if detmap == nil {
		detmapMutex.RUnlock()
		unmarshalDetmap()
		detmapMutex.RLock()
	}

	configId := uint32(uid >> 32)
	hpsConfig, ok := detmap.HpsConfig[configId]
	if !ok {
		hpsConfig, ok = detmap.HpsConfig[1]
	}
	if !ok {
		log.Println("didn't find HpsConfig:", configId)
		return nil
	}
	detConfig, ok := detmap.DetConfig[hpsConfig.DetConfig]
	if !ok {
		log.Println("didn't find DetConfig:", hpsConfig.DetConfig)
		return nil
	}
	return detConfig.ImageConfig
}
