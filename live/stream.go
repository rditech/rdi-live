// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package live

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rditech/rdi-live/data"
	"github.com/rditech/rdi-live/live/message"
	"github.com/rditech/rdi-live/live/shows"
	"github.com/rditech/rdi-live/model/rdi/slowdata"

	"github.com/go-redis/redis"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/proio-org/go-proio"
)

type ShowInfo struct {
	Show          interface{}
	Cancel        context.CancelFunc
	SampleChannel chan<- interface{}
}

type ShowType int

const (
	Projection ShowType = iota
	RollXY
	XY
	Hist2D
)

type SourceType int

const (
	Normal SourceType = iota
	Advanced
)

type SourceInfo struct {
	Name        string
	ShowIds     []uuid.UUID
	CompatShows []ShowType
	Type        SourceType
}

type StreamManager struct {
	Namespace       string
	Name            string
	Redis           *redis.Client
	Addr            string
	InitShows       func(*StreamManager)
	GenerateSources func(*StreamManager, *proio.Event)
	CleanupRunData  []data.EventProcessor
	Metadata        map[string]string

	ctx context.Context

	showInfo   map[uuid.UUID]ShowInfo
	sourceInfo map[string]*SourceInfo

	runChannel  chan *proio.Event
	runFilename string

	doPubDesc                bool
	lastTempMeta, lastHvMeta []byte
	startTime                time.Time
}

func (m *StreamManager) Manage(input <-chan *proio.Event, output chan<- *proio.Event) {
	var cancel context.CancelFunc
	m.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	defer m.rmAllShows(&message.Cmd{})

	if m.sourceInfo == nil {
		m.sourceInfo = make(map[string]*SourceInfo)
	}
	if m.showInfo == nil {
		m.showInfo = make(map[uuid.UUID]ShowInfo)
	}

	if m.InitShows != nil {
		m.InitShows(m)
	}

	cmds := message.ReceivePubSubCmds(m.ctx, m.Addr, m.Namespace+" stream cmd "+m.Name)
	m.announce()
	defer m.closeStream()

	m.startTime = time.Now()
	for {
		select {
		case event := <-input:
			if event == nil {
				return
			}

			m.handleMetadata(event)
			m.GenerateSources(m, event)

			if m.runChannel != nil {
				select {
				case m.runChannel <- event:
				default:
				}
			}
			output <- event
		case cmd := <-cmds:
			if cmd.Command == "kill" {
				return
			}

			m.execute(cmd)
		}
	}
}

func (m *StreamManager) GetSourceInfo(source string) *SourceInfo {
	sourceInfo := m.sourceInfo[source]
	if sourceInfo == nil {
		sourceInfo = &SourceInfo{Name: source}
		m.sourceInfo[source] = sourceInfo
	}
	return sourceInfo
}

func (m *StreamManager) HandleSource(sourceInfo *SourceInfo, t SourceType, value ...interface{}) {
	if len(value) == 0 || sourceInfo == nil {
		return
	}

	if sourceInfo.Type != t {
		sourceInfo.Type = t
	}

	if len(value) == 3 {
		val0f, ok0f := value[0].(*float32)
		val1f, ok1f := value[1].(*float32)
		val2f, ok2f := value[2].(*float32)
		if ok0f && ok1f && ok2f {
			if sourceInfo.CompatShows == nil {
				sourceInfo.CompatShows = []ShowType{Hist2D}
				m.listSource(sourceInfo.Name, sourceInfo)
			}

			for _, showId := range sourceInfo.ShowIds {
				showInfo := m.showInfo[showId]
				show := showInfo.Show

				switch show.(type) {
				case *shows.Hist2D:
					showInfo.SampleChannel <- &shows.Hist2DSample{
						float64(*val0f),
						float64(*val1f),
						float64(*val2f),
					}
				}
			}
		}
	} else if len(value) == 2 {
		val0d, ok0d := value[0].(*float64)
		val0f, ok0f := value[0].(*float32)
		val1f, ok1f := value[1].(*float32)
		if ok0d && ok1f {
			if sourceInfo.CompatShows == nil {
				sourceInfo.CompatShows = []ShowType{RollXY}
				m.listSource(sourceInfo.Name, sourceInfo)
			}

			for _, showId := range sourceInfo.ShowIds {
				showInfo := m.showInfo[showId]
				show := showInfo.Show

				switch show.(type) {
				case *shows.RollXY:
					showInfo.SampleChannel <- &shows.RollXYSample{
						*val0d,
						float64(*val1f),
						sourceInfo.Name,
					}
				}
			}
		} else if ok0f && ok1f {
			if sourceInfo.CompatShows == nil {
				sourceInfo.CompatShows = []ShowType{XY}
				m.listSource(sourceInfo.Name, sourceInfo)
			}

			for _, showId := range sourceInfo.ShowIds {
				showInfo := m.showInfo[showId]
				show := showInfo.Show

				switch show.(type) {
				case *shows.XY:
					showInfo.SampleChannel <- &shows.XYSample{
						float64(*val0f),
						float64(*val1f),
						sourceInfo.Name,
					}
				}
			}
		}
	}

	valArray, okArray := value[0].([]float32)
	if okArray {
		if sourceInfo.CompatShows == nil {
			sourceInfo.CompatShows = []ShowType{Projection}
			m.listSource(sourceInfo.Name, sourceInfo)
		}

		for _, showId := range sourceInfo.ShowIds {
			showInfo := m.showInfo[showId]
			show := showInfo.Show

			switch show.(type) {
			case *shows.Projection:
				showInfo.SampleChannel <- &shows.ProjectionSample{
					valArray,
					sourceInfo.Name,
				}
			}
		}
	}
}

func (m *StreamManager) announce() {
	msg := &message.Msg{
		Metadata: make(map[string]string),
	}
	msg.Type = "stream announce"
	msg.Metadata["name"] = m.Name
	if err := message.PublishJsonMsg(m.Redis, m.Namespace+" broadcast", msg); err != nil {
		log.Println(err)
	}
}

func (m *StreamManager) closeStream() {
	msg := &message.Msg{
		Metadata: make(map[string]string),
	}
	msg.Type = "stream close"
	msg.Metadata["name"] = m.Name
	if err := message.PublishJsonMsg(m.Redis, m.Namespace+" broadcast", msg); err != nil {
		log.Println(err)
	}
}

func (m *StreamManager) execute(cmd *message.Cmd) {
	log.Println("StreamManager:", string(cmd.Command))

	switch cmd.Command {
	case "new show":
		m.newShow(cmd)
	case "map source":
		m.mapSource(cmd)
	case "rm show":
		m.rmShow(cmd)
	case "rm all shows":
		m.rmAllShows(cmd)
	case "show cmd":
		m.showCmd(cmd)
	case "pub all shows":
		m.pubAllShows(cmd)
	case "list all sources":
		m.listAllSources(cmd)
	case "start run":
		m.startRun(cmd)
	case "stop run":
		m.stopRun(cmd)
	case "pub run meta":
		m.pubRunMeta(cmd)
	case "pub desc":
		m.pubDesc(cmd)
	}
}

func (m *StreamManager) newShow(cmd *message.Cmd) {
	var show shows.Show
	var period time.Duration

	if v, ok := cmd.Metadata["period"]; ok {
		ns, err := strconv.Atoi(v)
		if err == nil {
			period = time.Duration(ns)
		}
	}
	if period == 0 {
		period = 50 * time.Millisecond
	} else if period < 10*time.Millisecond {
		period = 10 * time.Millisecond
	}

	switch cmd.Metadata["type"] {
	case "Histogram 2D":
		plot := &shows.Hist2D{FramePeriod: period}
		plot.InitPlot()
		show = plot
	case "XY":
		plot := &shows.XY{FramePeriod: period}
		plot.InitPlot()
		show = plot
	case "Roll XY":
		plot := &shows.RollXY{FramePeriod: period}
		plot.InitPlot()
		show = plot
	case "Projection":
		plot := &shows.Projection{FramePeriod: period}
		plot.InitPlot()
		show = plot
	default:
		return
	}

	ctx, cancel := context.WithCancel(m.ctx)
	showId := uuid.New()
	idString := showId.String()
	channel := make(chan interface{}, 10000)
	showInfo := ShowInfo{
		Show:          show,
		Cancel:        cancel,
		SampleChannel: channel,
	}
	m.showInfo[showId] = showInfo

	go func() {
		log.Println("starting show", idString, "frame pusher")
		defer log.Println("stopped show", idString, "frame pusher")
		defer func() {
			msg := &message.Msg{
				Type:     "show close",
				Metadata: make(map[string]string),
			}
			msg.Metadata["stream"] = m.Name
			msg.Metadata["show id"] = idString
			message.PublishJsonMsg(m.Redis, m.Namespace+" stream "+m.Name, msg)
		}()

		show.UpdateFrame()

		var lastFrameCount uint64
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			frame, frameCount := show.Frame()
			if frameCount != lastFrameCount {
				frame.Type = "show frame"
				frame.Metadata["show id"] = idString
				frame.Metadata["stream name"] = m.Name
				if err := message.PublishJsonMsg(m.Redis, m.Namespace+" stream "+m.Name, frame); err != nil {
					log.Println(err)
				}
				time.Sleep(period)
			} else {
				time.Sleep(1 * time.Millisecond)
			}
			lastFrameCount = frameCount
		}
	}()

	go func() {
		log.Println("starting show", idString, "sample getter")
		defer log.Println("stopped show", idString, "sample getter")
		defer close(channel)

		for {
			select {
			case <-ctx.Done():
				return
			case sample := <-channel:
				show.AddSample(sample)
			}
		}
	}()

	cmd.Metadata["show id"] = idString
	m.mapSource(cmd)

	cmd.Metadata["show cmd"] = "set params"
	m.showCmd(cmd)
}

func (m *StreamManager) mapSource(cmd *message.Cmd) {
	source := cmd.Metadata["source"]
	if len(source) == 0 {
		return
	}

	if idString, ok := cmd.Metadata["show id"]; ok {
		showId, _ := uuid.Parse(idString)
		if _, ok := m.showInfo[showId]; !ok {
			return
		}
		for _, source := range strings.Split(source, ",") {
			source = strings.TrimSpace(source)

			sourceInfo, sourceInfoOk := m.sourceInfo[source]
			if !sourceInfoOk {
				sourceInfo = &SourceInfo{Name: source}
			}

			mapped := false
			for _, thisId := range sourceInfo.ShowIds {
				if thisId == showId {
					mapped = true
					break
				}
			}

			if !mapped {
				sourceInfo.ShowIds = append(sourceInfo.ShowIds, showId)
			}

			if !sourceInfoOk {
				m.sourceInfo[source] = sourceInfo
			}
		}
	}
}

func (m *StreamManager) rmShow(cmd *message.Cmd) {
	var showId uuid.UUID
	if idString, ok := cmd.Metadata["show id"]; ok {
		showId, _ = uuid.Parse(idString)
		if info, ok := m.showInfo[showId]; ok {
			info.Cancel()
			delete(m.showInfo, showId)
		}
	}

	for _, sourceInfo := range m.sourceInfo {
		list := sourceInfo.ShowIds
		tmp := list[:0]
		for i := range list {
			if list[i] != showId {
				tmp = append(tmp, list[i])
			}
		}
		sourceInfo.ShowIds = tmp
	}
}

func (m *StreamManager) rmAllShows(cmd *message.Cmd) {
	for _, info := range m.showInfo {
		info.Cancel()
	}

	m.showInfo = make(map[uuid.UUID]ShowInfo)
	for _, sourceInfo := range m.sourceInfo {
		sourceInfo.ShowIds = nil
	}
}

func (m *StreamManager) showCmd(cmd *message.Cmd) {
	if idString, ok := cmd.Metadata["show id"]; ok {
		showId, _ := uuid.Parse(idString)

		cmd.Command = cmd.Metadata["show cmd"]
		if info, ok := m.showInfo[showId]; ok {
			delete(cmd.Metadata, "show id")
			delete(cmd.Metadata, "show cmd")

			e := info.Show.(message.Executer)
			e.Execute(cmd)
		}
	}
}

func (m *StreamManager) pubAllShows(cmd *message.Cmd) {
	for _, info := range m.showInfo {
		framer := info.Show.(shows.Show)
		framer.UpdateFrameCount()
	}
}

func (m *StreamManager) listAllSources(*message.Cmd) {
	for source, sourceInfo := range m.sourceInfo {
		m.listSource(source, sourceInfo)
	}
}

func (m *StreamManager) listSource(source string, sourceInfo *SourceInfo) {
	msg := &message.Msg{
		Type:     "source announce",
		Metadata: make(map[string]string),
	}
	msg.Metadata["stream"] = m.Name
	msg.Metadata["source"] = source

	var compatShowList string
	for i, showType := range sourceInfo.CompatShows {
		switch showType {
		case Hist2D:
			compatShowList += "Histogram 2D"
		case XY:
			compatShowList += "XY"
		case RollXY:
			compatShowList += "Roll XY"
		case Projection:
			compatShowList += "Projection"
		}
		if i < len(sourceInfo.CompatShows)-1 {
			compatShowList += ", "
		}
	}
	msg.Metadata["compat shows"] = compatShowList

	var sourceType string
	switch sourceInfo.Type {
	case Normal:
		sourceType = "Normal"
	case Advanced:
		sourceType = "Advanced"
	}
	msg.Metadata["type"] = sourceType

	message.PublishJsonMsg(m.Redis, m.Namespace+" stream "+m.Name, msg)
}

var RunDateFormat = "2006_Jan2_15_04_05_UTC"

func (m *StreamManager) startRun(cmd *message.Cmd) {
	urlString := cmd.Metadata["url"] + "/" + time.Now().UTC().Format(RunDateFormat) + ".proio"
	writer, err := data.GetWriter(m.ctx, urlString, cmd.Metadata["credentials"])
	if err != nil {
		log.Println(err)
		return
	}

	thisUrl, err := url.Parse(urlString)
	m.runFilename = strings.TrimLeft(thisUrl.Path, "/")

	if m.runChannel != nil {
		m.runChannel <- nil
	}
	m.runChannel = make(chan *proio.Event, 10000)

	log.Printf("starting run %v://%v/%v", thisUrl.Scheme, thisUrl.Host, m.runFilename)

	writer.SetCompression(proio.LZ4)
	delete(cmd.Metadata, "credentials")
	delete(cmd.Metadata, "url")
	for key, value := range cmd.Metadata {
		writer.PushMetadata(key, []byte(value))
	}

	msg := &message.Msg{
		Type:     "stream status",
		Metadata: make(map[string]string),
	}
	msg.Metadata["stream"] = m.Name
	msg.Metadata["Run"] = m.runFilename
	message.PublishJsonMsg(m.Redis, m.Namespace+" stream "+m.Name, msg)

	ctx, cancel := context.WithCancel(m.ctx)
	go func() {
		defer writer.Close()

		go func() {
			start := time.Now()
			for {
				time.Sleep(100 * time.Millisecond)

				select {
				case <-ctx.Done():
					return
				default:
				}

				msg := &message.Msg{
					Type:     "stream status",
					Metadata: make(map[string]string),
				}
				msg.Metadata["stream"] = m.Name
				msg.Metadata["Run Time"] = fmt.Sprintf("%v", time.Since(start).Truncate(100*time.Millisecond))
				message.PublishJsonMsg(m.Redis, m.Namespace+" stream "+m.Name, msg)
			}
		}()
		defer cancel()
		defer log.Printf("stopping run %v://%v/%v", thisUrl.Scheme, thisUrl.Host, m.runFilename)

		for event := range m.runChannel {
			if event == nil {
				return
			}

			for key := range cmd.Metadata {
				delete(event.Metadata, key)
			}

			for _, proc := range m.CleanupRunData {
				proc(event)
			}
			writer.Push(event)
		}
	}()
}

func (m *StreamManager) stopRun(cmd *message.Cmd) {
	log.Printf("stopping run")
	if m.runChannel != nil {
		m.runChannel <- nil
	}
	m.runChannel = nil
}

func (m *StreamManager) pubRunMeta(cmd *message.Cmd) {
}

func (m *StreamManager) pubDesc(cmd *message.Cmd) {
	m.doPubDesc = true
}

func (m *StreamManager) handleMetadata(event *proio.Event) {
	if m.doPubDesc {
		m.doPubDesc = false

		msg := &message.Msg{
			Type:     "stream status",
			Metadata: make(map[string]string),
		}
		msg.Metadata["stream"] = m.Name
		msg.Metadata["Description"] = string(event.Metadata["Description"])
		message.PublishJsonMsg(m.Redis, m.Namespace+" stream "+m.Name, msg)
	}

	tempMeta := event.Metadata["Temp"]
	if len(tempMeta) > 0 && (m.lastTempMeta == nil || &tempMeta[0] != &m.lastTempMeta[0]) {
		m.lastTempMeta = tempMeta

		t := &slowdata.Temp{}
		err := proto.Unmarshal(tempMeta, t)
		if err == nil {
			msg := &message.Msg{
				Type:     "stream status",
				Metadata: make(map[string]string),
			}
			msg.Metadata["stream"] = m.Name
			msg.Metadata["Temp"] = t.String()
			message.PublishJsonMsg(m.Redis, m.Namespace+" stream "+m.Name, msg)
		}

		tStamp := float64(time.Since(m.startTime).Nanoseconds()) / 1e9

		for i, val := range t.Som {
			temp := m.GetSourceInfo(fmt.Sprintf("SoM %d Temp", i))
			m.HandleSource(temp, Advanced, &tStamp, &val)
		}

		for i, val := range t.Fem {
			temp := m.GetSourceInfo(fmt.Sprintf("FEM %d Temp", i))
			m.HandleSource(temp, Advanced, &tStamp, &val)
		}

		for i, val := range t.Board {
			temp := m.GetSourceInfo(fmt.Sprintf("Board Temp %d", i))
			m.HandleSource(temp, Advanced, &tStamp, &val)
		}
	}

	hvMeta := event.Metadata["HV"]
	if len(hvMeta) > 0 && (m.lastHvMeta == nil || &hvMeta[0] != &m.lastHvMeta[0]) {
		m.lastHvMeta = hvMeta

		t := &slowdata.Hv{}
		err := proto.Unmarshal(hvMeta, t)
		if err == nil {
			msg := &message.Msg{
				Type:     "stream status",
				Metadata: make(map[string]string),
			}
			msg.Metadata["stream"] = m.Name
			msg.Metadata["HV"] = t.String()
			message.PublishJsonMsg(m.Redis, m.Namespace+" stream "+m.Name, msg)
		}

		tStamp := float64(time.Since(m.startTime).Nanoseconds()) / 1e9

		for i, val := range t.DacValue {
			dacval := m.GetSourceInfo(fmt.Sprintf("DAC %d Value", i))
			floatVal := float32(val)
			m.HandleSource(dacval, Advanced, &tStamp, &floatVal)
		}
	}
}
