// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package shows

import (
	"bytes"
	"image/color"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rditech/rdi-live/live/message"
	rdiplot "github.com/rditech/rdi-live/plot"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgsvg"
)

type linePoints struct {
	plotter.XYs

	line   *plotter.Line
	points *plotter.Scatter
}

type ProjectionSample struct {
	Y        []float32
	LineName string
}

type Projection struct {
	Alpha            float64
	DisableAutorange bool
	DrawMagnitude    bool
	FramePeriod      time.Duration

	linepoints map[string]*linePoints
	inv_alpha  float64

	frame        *message.Msg
	frameCount   uint64
	frameExpired bool

	sync.RWMutex
	plot.Plot
}

func (s *Projection) Frame() (*message.Msg, uint64) {
	s.RLock()
	defer s.RUnlock()

	return s.frame, s.frameCount
}

func (s *Projection) Execute(cmd *message.Cmd) error {
	s.Lock()
	defer s.Unlock()

	switch cmd.Command {
	case "set params":
		for param, value := range cmd.Metadata {
			switch param {
			case "autorange":
				if strings.ToLower(value) == "false" {
					s.DisableAutorange = true
				} else {
					s.DisableAutorange = false
				}
			case "magnitude":
				if strings.ToLower(value) == "false" {
					s.DrawMagnitude = false
				} else {
					s.DrawMagnitude = true
				}
			case "min":
				min, err := strconv.ParseFloat(value, 64)
				if err == nil {
					s.Y.Min = min
				}
			case "max":
				max, err := strconv.ParseFloat(value, 64)
				if err == nil {
					s.Y.Max = max
				}
			case "alpha":
				alpha, err := strconv.ParseFloat(value, 64)
				if err == nil && alpha > 0 && alpha <= 1 {
					s.Alpha = alpha
					s.inv_alpha = 1.0 - alpha
				}
			case "logscale":
				if strings.ToLower(value) == "false" {
					s.Y.Tick.Marker = plot.DefaultTicks{}
					s.Y.Scale = plot.LinearScale{}
				} else {
					s.Y.Tick.Marker = rdiplot.LogTicks{}
					s.Y.Scale = &rdiplot.FuncScale{Func: rdiplot.Log10Min15}
				}
			}
		}
	}

	return nil
}

func (s *Projection) AddSample(vi interface{}) {
	v, ok := vi.(*ProjectionSample)
	if !ok {
		return
	}

	s.Lock()
	defer s.Unlock()

	if s.Alpha <= 0 {
		s.Alpha = 1
		s.inv_alpha = 0
	}

	if s.linepoints == nil {
		s.linepoints = make(map[string]*linePoints)
	}

	line := s.linepoints[v.LineName]
	if line == nil {
		line = &linePoints{
			XYs:    make(plotter.XYs, len(v.Y)),
			line:   &plotter.Line{},
			points: &plotter.Scatter{},
		}
		for i := range line.XYs {
			line.XYs[i].X = float64(i)
		}
		line.line.XYs = line.XYs
		line.line.LineStyle = plotter.DefaultLineStyle
		line.line.Dashes = plotutil.Dashes(len(s.linepoints))
		line.line.Color = plotutil.Color(len(s.linepoints))
		line.points.XYs = line.XYs
		line.points.GlyphStyle = plotter.DefaultGlyphStyle
		line.points.Shape = plotutil.Shape(len(s.linepoints))
		line.points.Color = plotutil.Color(len(s.linepoints))
		s.linepoints[v.LineName] = line
		s.Add(line.line)
		s.Add(line.points)
		s.Legend.Add(v.LineName, line.line, line.points)
	}

	if len(line.XYs) != len(v.Y) {
		line.XYs = make(plotter.XYs, len(v.Y))
		for i := range line.XYs {
			line.XYs[i].X = float64(i)
		}
		line.line.XYs = line.XYs
		line.points.XYs = line.XYs
	}

	for i, thisY := range v.Y {
		if s.DrawMagnitude {
			thisY = float32(math.Abs(float64(thisY)))
		}
		line.XYs[i].Y *= s.inv_alpha
		line.XYs[i].Y += s.Alpha * float64(thisY)
	}

	if s.frameExpired {
		s.frameExpired = false
		go s.updateFrame(true)
	}
}

func (s *Projection) updateFrame(doLock bool) {
	if doLock {
		s.Lock()
		defer s.Unlock()
	}

	if !s.DisableAutorange && len(s.linepoints) > 0 {
		s.X.Min = math.Inf(+1)
		s.X.Max = math.Inf(-1)
		s.Y.Min = math.Inf(+1)
		s.Y.Max = math.Inf(-1)
		for _, line := range s.linepoints {
			xmin, xmax, ymin, ymax := line.line.DataRange()
			s.X.Min = math.Min(s.X.Min, xmin)
			s.X.Max = math.Max(s.X.Max, xmax)
			s.Y.Min = math.Min(s.Y.Min, ymin)
			s.Y.Max = math.Max(s.Y.Max, ymax)

			xmin, xmax, ymin, ymax = line.points.DataRange()
			s.X.Min = math.Min(s.X.Min, xmin)
			s.X.Max = math.Max(s.X.Max, xmax)
			s.Y.Min = math.Min(s.Y.Min, ymin)
			s.Y.Max = math.Max(s.Y.Max, ymax)
		}
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
	s.frame.Metadata["show type"] = "Projection"
	s.frame.Metadata["alpha"] = strconv.FormatFloat(s.Alpha, 'g', 8, 64)
	s.frame.Metadata["autorange"] = strconv.FormatBool(!s.DisableAutorange)
	s.frame.Metadata["magnitude"] = strconv.FormatBool(s.DrawMagnitude)
	s.frame.Metadata["min"] = strconv.FormatFloat(s.Y.Min, 'g', 4, 64)
	s.frame.Metadata["max"] = strconv.FormatFloat(s.Y.Max, 'g', 4, 64)
	switch s.Y.Scale.(type) {
	case plot.LinearScale:
		s.frame.Metadata["logscale"] = "false"
	default:
		s.frame.Metadata["logscale"] = "true"
	}

	s.frameCount++

	go func() {
		time.Sleep(s.FramePeriod)
		s.Lock()
		defer s.Unlock()
		s.frameExpired = true
	}()
}

func (s *Projection) UpdateFrame() {
	s.updateFrame(true)
}

func (s *Projection) UpdateFrameCount() {
	s.Lock()
	defer s.Unlock()
	s.frameCount++
}

func (s *Projection) InitPlot() {
	s.Lock()
	defer s.Unlock()

	donor, _ := plot.New()
	s.BackgroundColor = color.Transparent
	s.X = donor.X
	s.Y = donor.Y
	s.Legend = donor.Legend
	s.Title = donor.Title
}
