// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package shows

import (
	"bytes"
	"image/color"
	"strconv"
	"sync"
	"time"

	"github.com/rditech/rdi-live/live/message"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgsvg"
)

type XYSample struct {
	X, Y     float64
	LineName string
}

type XY struct {
	FramePeriod time.Duration
	NSample     int

	scatter *plotter.Scatter

	frame        *message.Msg
	frameCount   uint64
	frameExpired bool

	sync.RWMutex
	plot.Plot
}

func (s *XY) Frame() (*message.Msg, uint64) {
	s.RLock()
	defer s.RUnlock()

	return s.frame, s.frameCount
}

func (s *XY) Execute(cmd *message.Cmd) error {
	s.Lock()
	defer s.Unlock()

	switch cmd.Command {
	case "set params":
		for param, value := range cmd.Metadata {
			switch param {
			case "min y":
				min, err := strconv.ParseFloat(value, 64)
				if err == nil {
					s.Y.Min = min
				}
			case "max y":
				max, err := strconv.ParseFloat(value, 64)
				if err == nil {
					s.Y.Max = max
				}
			case "min x":
				min, err := strconv.ParseFloat(value, 64)
				if err == nil {
					s.X.Min = min
				}
			case "max x":
				max, err := strconv.ParseFloat(value, 64)
				if err == nil {
					s.X.Max = max
				}
			case "nsample":
				nSample, err := strconv.ParseInt(value, 10, 64)
				if err == nil && nSample >= 0 {
					s.NSample = int(nSample)
				}
			}
		}
	}

	return nil
}

func (s *XY) AddSample(vi interface{}) {
	v, ok := vi.(*XYSample)
	if !ok {
		return
	}

	s.Lock()
	defer s.Unlock()

	if s.scatter == nil {
		pts := make(plotter.XYs, 0)
		s.scatter, _ = plotter.NewScatter(pts)
		s.scatter.GlyphStyle.Shape = draw.PlusGlyph{}
		s.scatter.GlyphStyle.Radius = 1
		s.Add(s.scatter)
	}
	pt := plotter.XY{
		X: v.X,
		Y: v.Y,
	}
	s.scatter.XYs = append(s.scatter.XYs, pt)

	if s.NSample == 0 {
		s.NSample = 100
	}
	if len(s.scatter.XYs) > s.NSample {
		s.scatter.XYs = s.scatter.XYs[len(s.scatter.XYs)-s.NSample:]
	}

	if s.frameExpired {
		s.frameExpired = false
		go s.updateFrame(true)
	}
}

func (s *XY) updateFrame(doLock bool) {
	if doLock {
		s.Lock()
		defer s.Unlock()
	}

	svg := vgsvg.New(4*vg.Inch, 2.5*vg.Inch)
	c := draw.New(svg)
	s.Draw(c)
	buf := &bytes.Buffer{}
	svg.WriteTo(buf)

	s.frame = &message.Msg{
		Metadata: make(map[string]string),
		Payload:  buf.Bytes(),
	}
	s.frame.Metadata["show type"] = "XY"
	s.frame.Metadata["min y"] = strconv.FormatFloat(s.Y.Min, 'g', 4, 64)
	s.frame.Metadata["max y"] = strconv.FormatFloat(s.Y.Max, 'g', 4, 64)
	s.frame.Metadata["min x"] = strconv.FormatFloat(s.X.Min, 'g', 4, 64)
	s.frame.Metadata["max x"] = strconv.FormatFloat(s.X.Max, 'g', 4, 64)
	s.frame.Metadata["nsample"] = strconv.FormatInt(int64(s.NSample), 10)

	s.frameCount++

	go func() {
		time.Sleep(s.FramePeriod)
		s.Lock()
		defer s.Unlock()
		s.frameExpired = true
	}()
}

func (s *XY) UpdateFrame() {
	s.updateFrame(true)
}

func (s *XY) UpdateFrameCount() {
	s.Lock()
	defer s.Unlock()
	s.frameCount++
}

func (s *XY) InitPlot() {
	s.Lock()
	defer s.Unlock()

	donor, _ := plot.New()
	s.BackgroundColor = color.Transparent
	s.X = donor.X
	s.X.Min = -100
	s.X.Max = 100
	s.Y = donor.Y
	s.Y.Min = -100
	s.Y.Max = 100
	s.Legend = donor.Legend
	s.Title = donor.Title
}
