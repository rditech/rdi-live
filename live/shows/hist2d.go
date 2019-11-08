// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package shows

import (
	"bytes"
	"image/color"
	"image/png"
	"strconv"
	"sync"
	"time"

	"github.com/rditech/rdi-live/live/message"

	"go-hep.org/x/hep/hbook"
	"go-hep.org/x/hep/hplot"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette/moreland"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

type Hist2DSample struct {
	X, Y   float64
	Weight float64
}

type Hist2D struct {
	FramePeriod time.Duration

	hb *hbook.H2D

	frame        *message.Msg
	frameCount   uint64
	frameExpired bool

	sync.RWMutex
}

func (s *Hist2D) Frame() (*message.Msg, uint64) {
	s.RLock()
	defer s.RUnlock()

	return s.frame, s.frameCount
}

func (s *Hist2D) Execute(cmd *message.Cmd) error {
	s.Lock()
	defer s.Unlock()

	switch cmd.Command {
	case "set params":
		for param, value := range cmd.Metadata {
			switch param {
			case "reset":
				b := s.hb.Binning
				s.hb = hbook.NewH2D(
					b.Nx,
					b.XRange.Min,
					b.XRange.Max,
					b.Ny,
					b.YRange.Min,
					b.YRange.Max,
				)
			case "min y":
				min, err := strconv.ParseFloat(value, 64)
				if err == nil {
					b := s.hb.Binning
					s.hb = hbook.NewH2D(
						b.Nx,
						b.XRange.Min,
						b.XRange.Max,
						b.Ny,
						min,
						b.YRange.Max,
					)
				}
			case "max y":
				max, err := strconv.ParseFloat(value, 64)
				if err == nil {
					b := s.hb.Binning
					s.hb = hbook.NewH2D(
						b.Nx,
						b.XRange.Min,
						b.XRange.Max,
						b.Ny,
						b.YRange.Min,
						max,
					)
				}
			case "min x":
				min, err := strconv.ParseFloat(value, 64)
				if err == nil {
					b := s.hb.Binning
					s.hb = hbook.NewH2D(
						b.Nx,
						min,
						b.XRange.Max,
						b.Ny,
						b.YRange.Min,
						b.YRange.Max,
					)
				}
			case "max x":
				max, err := strconv.ParseFloat(value, 64)
				if err == nil {
					b := s.hb.Binning
					s.hb = hbook.NewH2D(
						b.Nx,
						b.XRange.Min,
						max,
						b.Ny,
						b.YRange.Min,
						b.YRange.Max,
					)
				}
			case "nbins x":
				nBins, err := strconv.ParseInt(value, 10, 64)
				if err == nil && nBins > 0 {
					b := s.hb.Binning
					s.hb = hbook.NewH2D(
						int(nBins),
						b.XRange.Min,
						b.XRange.Max,
						b.Ny,
						b.YRange.Min,
						b.YRange.Max,
					)
				}
			case "nbins y":
				nBins, err := strconv.ParseInt(value, 10, 64)
				if err == nil && nBins > 0 {
					b := s.hb.Binning
					s.hb = hbook.NewH2D(
						b.Nx,
						b.XRange.Min,
						b.XRange.Max,
						int(nBins),
						b.YRange.Min,
						b.YRange.Max,
					)
				}
			}
		}
	}

	return nil
}

func (s *Hist2D) AddSample(vi interface{}) {
	v, ok := vi.(*Hist2DSample)
	if !ok {
		return
	}

	s.Lock()
	defer s.Unlock()

	s.hb.Fill(v.X, v.Y, v.Weight)

	if s.frameExpired {
		s.frameExpired = false
		go s.updateFrame(true)
	}
}

func (s *Hist2D) updateFrame(doLock bool) {
	if doLock {
		s.Lock()
		defer s.Unlock()
	}

	p, _ := plot.New()
	p.BackgroundColor = color.Transparent
	hp := &hplot.Plot{
		Plot:  p,
		Style: hplot.DefaultStyle,
	}
	if s.hb != nil {
		colorMap := moreland.Kindlmann()
		h := hplot.NewH2D(s.hb, colorMap.Palette(1000))
		h.Infos.Style = hplot.HInfoMean | hplot.HInfoStdDev
		hp.Add(h)
		hp.Add(hplot.NewGrid())
	}

	img := vgimg.New(4*vg.Inch, 2.5*vg.Inch)
	c := draw.New(img)
	p.Draw(c)
	buf := &bytes.Buffer{}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	encoder.Encode(buf, img.Image())

	s.frame = &message.Msg{
		Metadata: make(map[string]string),
		Payload:  buf.Bytes(),
	}
	s.frame.Metadata["show type"] = "Histogram 2D"
	s.frame.Metadata["is png"] = "true"
	s.frame.Metadata["reset"] = ""
	if s.hb != nil {
		b := s.hb.Binning
		s.frame.Metadata["nbins x"] = strconv.FormatInt(int64(b.Nx), 10)
		s.frame.Metadata["min x"] = strconv.FormatFloat(b.XRange.Min, 'g', 4, 64)
		s.frame.Metadata["max x"] = strconv.FormatFloat(b.XRange.Max, 'g', 4, 64)
		s.frame.Metadata["nbins y"] = strconv.FormatInt(int64(b.Ny), 10)
		s.frame.Metadata["min y"] = strconv.FormatFloat(b.YRange.Min, 'g', 4, 64)
		s.frame.Metadata["max y"] = strconv.FormatFloat(b.YRange.Max, 'g', 4, 64)
	}

	s.frameCount++

	go func() {
		time.Sleep(s.FramePeriod)
		s.Lock()
		defer s.Unlock()
		s.frameExpired = true
	}()
}

func (s *Hist2D) UpdateFrame() {
	s.updateFrame(true)
}

func (s *Hist2D) UpdateFrameCount() {
	s.Lock()
	defer s.Unlock()
	s.frameCount++
}

func (s *Hist2D) InitPlot() {
	s.Lock()
	defer s.Unlock()

	s.hb = hbook.NewH2D(100, -100, 100, 100, -100, 100)
}
