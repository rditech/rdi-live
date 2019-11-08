// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package shows

import (
	"github.com/rditech/rdi-live/live/message"
)

type Show interface {
	Frame() (*message.Msg, uint64)
	UpdateFrame()
	UpdateFrameCount()
	AddSample(interface{})
}
