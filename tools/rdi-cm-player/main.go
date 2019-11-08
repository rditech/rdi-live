// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"github.com/rditech/rdi-live/data"
)

var speed = data.FlagSet.Float64("s", 1.0, "relative speed of playback")

func main() {
	player := &data.Player{}
	playOp := data.OpArray{
		data.StreamOp{
			Description:     "Repeats data using timestamp information to \"play\" data at a realistic rate from a recording",
			StreamProcessor: player.PlayCmStream,
		},
	}
	playOp.RunCmdFlagParse()
	player.Speed = *speed
	playOp.RunCmd()
}
