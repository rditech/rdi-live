// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"github.com/proio-org/go-proio"
)

func CmMerge(input <-chan *proio.Event, output chan<- *proio.Event) {
	for event := range input {
		output <- event
	}
}
