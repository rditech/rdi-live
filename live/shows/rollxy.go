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

type smoothLine struct {
	smoother func(float64) float64
	i        int
	trigEnd  int

	plotter.Line
}

type RollXYSample struct {
	X, Y     float64
	LineName string
}

type RollXY struct {
	Alpha             float64
	DisableAutorange  bool
	Downsample        int
	DrawMagnitude     bool
	FramePeriod       time.Duration
	NSample           int
	Trigger           string
	TriggerFalling    bool
	TriggerLeadSample int
	TriggerLevel      float64

	frame        *message.Msg
	frameCount   uint64
	frameExpired bool
	lines        map[string]*smoothLine
	triggered    bool

	sync.RWMutex
	plot.Plot
}

func (s *RollXY) Frame() (*message.Msg, uint64) {
	s.RLock()
	defer s.RUnlock()

	return s.frame, s.frameCount
}

func (s *RollXY) Execute(cmd *message.Cmd) error {
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
			case "logscale":
				if strings.ToLower(value) == "false" {
					s.Y.Tick.Marker = plot.DefaultTicks{}
					s.Y.Scale = plot.LinearScale{}
				} else {
					s.Y.Tick.Marker = rdiplot.LogTicks{}
					s.Y.Scale = &rdiplot.FuncScale{Func: rdiplot.Log10Min15}
				}
			case "alpha":
				alpha, err := strconv.ParseFloat(value, 64)
				if err == nil && alpha > 0 && alpha <= 1 {
					s.Alpha = alpha
					for _, line := range s.lines {
						if len(line.XYs) > 0 {
							line.smoother = rdiplot.MakeSmoother(alpha, line.XYs[len(line.XYs)-1].Y)
						} else {
							line.smoother = rdiplot.MakeSmoother(alpha, 0)
						}
					}
				}
			case "nsample":
				nSample, err := strconv.ParseInt(value, 10, 64)
				if err == nil && nSample >= 0 {
					s.NSample = int(nSample)
				}
			case "downsample":
				downsample, err := strconv.ParseInt(value, 10, 64)
				if err == nil && downsample >= 0 {
					s.Downsample = int(downsample)
				}
			case "trigger":
				s.Trigger = value
			case "triglevel":
				trigLevel, err := strconv.ParseFloat(value, 64)
				if err == nil {
					s.TriggerLevel = trigLevel
				}
			case "trigleadsample":
				leadsample, err := strconv.ParseInt(value, 10, 64)
				if err == nil && leadsample >= 0 {
					s.TriggerLeadSample = int(leadsample)
				}
			case "trigfall":
				if strings.ToLower(value) == "false" {
					s.TriggerFalling = false
				} else {
					s.TriggerFalling = true
				}
			}
		}
	}

	return nil
}

func (s *RollXY) AddSample(vi interface{}) {
	v, ok := vi.(*RollXYSample)
	if !ok {
		return
	}

	s.Lock()
	defer s.Unlock()

	if s.NSample == 0 {
		s.NSample = 500
	}
	if s.Downsample == 0 {
		s.Downsample = 1
	}
	if s.Alpha == 0 {
		s.Alpha = 1
	}

	if s.lines == nil {
		s.lines = make(map[string]*smoothLine)
	}

	line := s.lines[v.LineName]
	if line == nil {
		line = &smoothLine{
			smoother: rdiplot.MakeSmoother(s.Alpha, 0),
		}
		line.LineStyle = plotter.DefaultLineStyle
		line.Color = plotutil.Color(len(s.lines))
		s.lines[v.LineName] = line
		s.Add(line)
		s.Legend.Add(v.LineName, line)
	}
	line.i++

	if s.DrawMagnitude {
		v.Y = math.Abs(v.Y)
	}
	ySmooth := line.smoother(v.Y)

	if !s.triggered {
		if len(line.XYs) > s.TriggerLeadSample {
			if s.Trigger == v.LineName {
				if s.TriggerFalling {
					if ySmooth <= s.TriggerLevel && line.XYs[len(line.XYs)-1].Y > s.TriggerLevel {
						s.triggered = true
						line.trigEnd = line.i + (s.NSample-s.TriggerLeadSample)*s.Downsample
					}
				} else {
					if ySmooth >= s.TriggerLevel && line.XYs[len(line.XYs)-1].Y < s.TriggerLevel {
						s.triggered = true
						line.trigEnd = line.i + (s.NSample-s.TriggerLeadSample)*s.Downsample
					}
				}
			}
		}
	}

	if line.i%s.Downsample == 0 {
		if len(line.XYs) > 0 && v.X < line.XYs[len(line.XYs)-1].X {
			line.XYs = nil
			line.smoother = rdiplot.MakeSmoother(s.Alpha, 0)
			ySmooth = line.smoother(v.Y)
		}
		line.XYs = append(line.XYs, plotter.XY{X: v.X, Y: ySmooth})
		for len(line.XYs) > s.NSample {
			line.XYs = line.XYs[1:]
		}
	}

	if line.i == line.trigEnd {
		if s.frameExpired {
			s.frameExpired = false
			s.updateFrame(false)
		}
		for _, line := range s.lines {
			line.XYs = nil
		}
		s.triggered = false
	} else if s.Trigger == "" {
		if s.frameExpired {
			s.frameExpired = false
			go s.updateFrame(true)
		}
	}
}

func (s *RollXY) updateFrame(doLock bool) {
	if doLock {
		s.Lock()
		defer s.Unlock()
	}

	s.X.Min = math.Inf(+1)
	s.X.Max = math.Inf(-1)
	if !s.DisableAutorange {
		s.Y.Min = math.Inf(+1)
		s.Y.Max = math.Inf(-1)
	}
	for _, line := range s.lines {
		xmin, xmax, ymin, ymax := line.DataRange()
		s.X.Min = math.Min(s.X.Min, xmin)
		s.X.Max = math.Max(s.X.Max, xmax)
		if !s.DisableAutorange {
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
	s.frame.Metadata["show type"] = "Roll XY"
	s.frame.Metadata["trigger"] = s.Trigger
	s.frame.Metadata["triglevel"] = strconv.FormatFloat(s.TriggerLevel, 'g', 4, 64)
	s.frame.Metadata["trigfall"] = strconv.FormatBool(s.TriggerFalling)
	s.frame.Metadata["trigleadsample"] = strconv.FormatInt(int64(s.TriggerLeadSample), 10)
	s.frame.Metadata["alpha"] = strconv.FormatFloat(s.Alpha, 'g', 8, 64)
	s.frame.Metadata["nsample"] = strconv.FormatInt(int64(s.NSample), 10)
	s.frame.Metadata["downsample"] = strconv.FormatInt(int64(s.Downsample), 10)
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

func (s *RollXY) UpdateFrame() {
	s.updateFrame(true)
}

func (s *RollXY) UpdateFrameCount() {
	s.Lock()
	defer s.Unlock()
	s.frameCount++
}

func (s *RollXY) InitPlot() {
	s.Lock()
	defer s.Unlock()

	donor, _ := plot.New()
	s.BackgroundColor = color.Transparent
	s.X = donor.X
	s.Y = donor.Y
	s.Legend = donor.Legend
	s.Title = donor.Title

	s.X.Tick.Marker = rdiplot.RollTicks{}
}
