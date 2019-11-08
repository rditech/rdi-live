// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"log"

	"github.com/rditech/rdi-live/data"

	"github.com/google/uuid"
)

func main() {
	ops := data.OpArray{}
	reader := ops.GetReader()
	reader.Skip(0)
	uidBytes, ok := reader.Metadata["UID"]
	if !ok || len(uidBytes) != 8 {
		log.Println("falling back to random UID")
		uuidBytes := [16]byte(uuid.New())
		uidBytes = uuidBytes[:8]
	}
	uid := binary.BigEndian.Uint64(uidBytes)

	peds := &data.Pedestals{}
	recon := data.NewBeamReconstruction(uid)
	ops = data.OpArray{
		data.EventOp{
			Description:    "Maps event axes",
			EventProcessor: data.MapEvent,
		},
		data.EventOp{
			Description:    "Calculates and adds event correlation",
			EventProcessor: data.CorrelateCmEvent,
		},
		data.StreamOp{
			Description:     "Calculates and subtracts pedestals from stream",
			StreamProcessor: peds.Subtract,
		},
		data.EventOp{
			Description:    "Reconstruct beam paramters",
			EventProcessor: recon.FillBeamInfo,
		},
	}

	ops.RunCmd()
}
