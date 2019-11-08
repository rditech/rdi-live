// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package plot

func MakeSmoother(alpha, init float64) func(float64) float64 {
	inv_alpha := 1.0 - alpha
	val := init
	return func(newVal float64) float64 {
		val = inv_alpha*val + alpha*newVal
		return val
	}
}
