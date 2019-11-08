// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package ingress

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/rditech/rdi-live/data"
	"github.com/rditech/rdi-live/live"
	"github.com/rditech/rdi-live/live/message"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/proio-org/go-proio"
	"golang.org/x/net/websocket"
)

// WsCollector is a Websocket ProIO data collector
type WsCollector struct {
	Redis            *redis.Client
	Addr             string
	DefaultNamespace string
}

func (wsc *WsCollector) Collect(c *websocket.Conn) {
	log.Println("serving websocket data collector to", c.Request().RemoteAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := proio.NewReader(c)
	defer reader.Close()
	reader.Skip(0)
	input := reader.ScanEvents(1000)

	// look at the metadata and use it to name a PubSub stream
	uidBytes, ok := reader.Metadata["UID"]
	if !ok || len(uidBytes) != 8 {
		log.Println("falling back to random UID")
		uuidBytes := [16]byte(uuid.New())
		uidBytes = uuidBytes[:8]
	}
	uid := binary.BigEndian.Uint64(uidBytes)

	namespace := wsc.DefaultNamespace
	streamName := data.GetDetName(uid)
	if streamName == "" {
		streamName = strconv.FormatUint(uid, 16)
	}
	chanString := namespace + " ingress " + streamName

	// if there is no stream data handler, create one
	nSub := wsc.Redis.PubSubNumSub(chanString).Val()
	if nSub[chanString] == 0 {
		if err := wsc.makeNewDataHandler(
			ctx,
			namespace,
			streamName,
			uid,
		); err != nil {
			log.Println(err)
			return
		}
	}

	redisClient := redis.NewClient(&redis.Options{Addr: wsc.Addr})
	defer redisClient.Close()
	writer := proio.NewWriter(&PubSubWriter{Redis: redisClient, Channel: chanString})
	defer writer.Close()
	writer.BucketDumpThres = 0x1
	writer.SetCompression(proio.UNCOMPRESSED)
	log.Println("data collector starting writing to channel", chanString)
	defer log.Println("data collector done writing to channel", chanString)

	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	for event := range input {
		// loop over all input events and retransmit them over Redis PubSub

		// if there is no stream data handler, break
		nSub := wsc.Redis.PubSubNumSub(chanString).Val()
		if nSub[chanString] == 0 {
			log.Printf("no stream handler for \"%s\"", chanString)
			break
		}

		// retransmit over Redis PubSub
		writer.Push(event)

		// update the websocket read deadline
		c.SetReadDeadline(time.Now().Add(10 * time.Second))
	}
}

func (wsc *WsCollector) makeNewDataHandler(
	ctx context.Context,
	namespace,
	streamName string,
	uid uint64,
) error {
	chanString := namespace + " ingress " + streamName
	log.Println("subscribing new data handler to channel", chanString)

	redisClient := redis.NewClient(&redis.Options{Addr: wsc.Addr})
	pubSub := redisClient.Subscribe(chanString)
	_, err := pubSub.Receive()
	if err != nil {
		redisClient.Close()
		return err
	}

	go func() {
		defer redisClient.Close()
		defer pubSub.Close()
		reader := proio.NewReader(
			&PubSubReader{
				Channel: pubSub.ChannelSize(1000),
				Ctx:     ctx,
			},
		)
		defer reader.Close()
		input := reader.ScanEvents(1000)

		// publish input buffer size
		go func() {
			for {
				msg := &message.Msg{
					Type:     "stream status",
					Metadata: make(map[string]string),
				}
				msg.Metadata["stream"] = streamName
				msg.Metadata["Buffer Size"] = fmt.Sprintf("%v", len(input))
				message.PublishJsonMsg(redisClient, namespace+" stream "+streamName, msg)

				select {
				case <-ctx.Done():
					msg := &message.Msg{
						Type:     "stream status",
						Metadata: make(map[string]string),
					}
					msg.Metadata["stream"] = streamName
					msg.Metadata["Buffer Size"] = fmt.Sprintf("stream disconnected, wrapping up")
					message.PublishJsonMsg(redisClient, namespace+" stream "+streamName, msg)
					return
				default:
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()

		// make operations array for the stream
		ops := live.BuildOpArray(namespace, streamName, redisClient, wsc.Addr, uid)

		// execute operations as a data sink
		if ops != nil {
			ops.Sink(input)
		}

		log.Println("quitting subscriber goroutine on channel", chanString)
	}()

	return nil
}
