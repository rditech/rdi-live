// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package plot

import (
	"math"
	"strconv"

	"gonum.org/v1/plot"
)

type FuncScale struct {
	Func func(float64) float64
}

func (s *FuncScale) Normalize(min, max, x float64) float64 {
	if s.Func == nil {
		panic("s.Func is nil")
	}
	fMin := s.Func(min)
	return (s.Func(x) - fMin) / (s.Func(max) - fMin)
}

func Log10Min3(x float64) float64 {
	if x <= 0.001 {
		return -3
	}
	return math.Log10(x)
}

func Log10Min15(x float64) float64 {
	if x <= 1e-15 {
		return -15
	}
	return math.Log10(x)
}

type RollTicks struct {
	NSuggestedTicks int
}

func (t RollTicks) Ticks(min, max float64) []plot.Tick {
	if t.NSuggestedTicks == 0 {
		t.NSuggestedTicks = 4
	}

	if max <= min {
		panic("illegal range")
	}

	tens := math.Pow10(int(math.Floor(math.Log10(max - min))))
	n := (max - min) / tens
	for n < float64(t.NSuggestedTicks)-1 {
		tens /= 10
		n = (max - min) / tens
	}

	majorMult := int(n / float64(t.NSuggestedTicks-1))
	switch majorMult {
	case 7:
		majorMult = 6
	case 9:
		majorMult = 8
	}
	majorDelta := float64(majorMult) * tens
	val := math.Floor(min/majorDelta) * majorDelta
	// Makes a list of non-truncated y-values.
	var labels []float64
	for val <= max {
		if val >= min {
			labels = append(labels, val)
		}
		val += majorDelta
	}
	prec := int(math.Ceil(math.Log10(val)) - math.Floor(math.Log10(majorDelta)))
	// Makes a list of big ticks.
	var ticks []plot.Tick
	for _, v := range labels {
		vRounded := round(v, prec)
		ticks = append(ticks, plot.Tick{Value: vRounded, Label: formatFloatTick(vRounded, -1)})
	}
	minorDelta := majorDelta / 2
	if ticks[len(ticks)-1].Value > max-minorDelta {
		ticks = ticks[:len(ticks)-1]
	}
	switch majorMult {
	case 3, 6:
		minorDelta = majorDelta / 3
	case 5:
		minorDelta = majorDelta / 5
	}

	val = math.Floor(min/minorDelta) * minorDelta
	for val <= max {
		found := false
		for _, t := range ticks {
			if t.Value == val {
				found = true
			}
		}
		if val >= min && val <= max && !found {
			ticks = append(ticks, plot.Tick{Value: val})
		}
		val += minorDelta
	}
	return ticks
}

func round(x float64, prec int) float64 {
	if x == 0 {
		// Make sure zero is returned
		// without the negative bit set.
		return 0
	}
	// Fast path for positive precision on integers.
	if prec >= 0 && x == math.Trunc(x) {
		return x
	}
	pow := math.Pow10(prec)
	intermed := x * pow
	if math.IsInf(intermed, 0) {
		return x
	}
	if x < 0 {
		x = math.Ceil(intermed - 0.5)
	} else {
		x = math.Floor(intermed + 0.5)
	}

	if x == 0 {
		return 0
	}

	return x / pow
}

type LogTicks struct{}

func (LogTicks) Ticks(min, max float64) []plot.Tick {
	val := math.Pow10(int(Log10Min15(min)))
	max = math.Pow10(int(math.Ceil(Log10Min15(max))))
	var ticks []plot.Tick
	for val < max {
		for i := 1; i < 10; i++ {
			if i == 1 {
				ticks = append(ticks, plot.Tick{Value: val, Label: formatFloatTick(val, 5)})
			}
			ticks = append(ticks, plot.Tick{Value: val * float64(i)})
		}
		val *= 10
	}
	ticks = append(ticks, plot.Tick{Value: val, Label: formatFloatTick(val, 5)})

	return ticks
}

func formatFloatTick(v float64, prec int) string {
	return strconv.FormatFloat(v, 'g', prec, 64)
}
