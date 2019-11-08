// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package ingress

import (
	"context"
	"io"

	"github.com/go-redis/redis"
)

// PubSubWriter is an io.Writer that publishes to a redis PubSub channel
type PubSubWriter struct {
	Redis   *redis.Client
	Channel string
}

func (wrt *PubSubWriter) Write(p []byte) (int, error) {
	intCmd := wrt.Redis.Publish(wrt.Channel, string(p))
	if intCmd.Err() != nil {
		return 0, intCmd.Err()
	}
	return len(p), nil
}

// PubSubReader is an io.Reader that reads from a redis PubSub channel
type PubSubReader struct {
	Channel  <-chan *redis.Message
	Ctx      context.Context
	leftover []byte
}

func (rdr *PubSubReader) Read(p []byte) (int, error) {
	if len(rdr.leftover) == 0 {
		select {
		case msg := <-rdr.Channel:
			if msg == nil {
				return 0, io.EOF
			}

			rdr.leftover = append(rdr.leftover, []byte(msg.Payload)...)
		case <-rdr.Ctx.Done():
			return 0, io.EOF
		}
	}

	min := len(p)
	if len(rdr.leftover) < min {
		min = len(rdr.leftover)
	}

	copy(p, rdr.leftover[:min])
	rdr.leftover = rdr.leftover[min:]

	return min, nil
}
