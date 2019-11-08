// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package message

import (
	"context"
	"encoding/json"
	"log"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

type Msg struct {
	Type     string
	Metadata map[string]string
	Payload  []byte
}

func PublishJsonMsg(redis *redis.Client, channel string, msg *Msg) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	redis.Publish(channel, string(msgBytes))
	return nil
}

type Cmd struct {
	Command  string
	Metadata map[string]string
}

type Executer interface {
	Execute(*Cmd) error
}

func ReceivePubSubCmds(ctx context.Context, addr, channel string) <-chan *Cmd {
	cmds := make(chan *Cmd)

	go func() {
		defer close(cmds)

		redisClient := redis.NewClient(&redis.Options{Addr: addr})
		defer redisClient.Close()
		sub := redisClient.Subscribe(channel)
		_, err := sub.Receive()
		if err != nil {
			log.Println("sub.Receive():", err)
			redisClient.Close()
			return
		}
		defer sub.Close()

		log.Println("listening for commands on channel", channel)
		defer log.Println("done listening for commands on channel", channel)

		channel := sub.ChannelSize(10)
		for {
			select {
			case msg := <-channel:
				var cmd Cmd
				err := json.Unmarshal([]byte(msg.Payload), &cmd)
				if err != nil {
					return
				}
				cmds <- &cmd
			case <-ctx.Done():
				return
			}
		}
	}()

	return cmds
}

func ReceiveWsCmds(ctx context.Context, c *websocket.Conn) <-chan *Cmd {
	cmds := make(chan *Cmd)

	go func() {
		defer close(cmds)

		for {
			var cmd Cmd
			err := c.ReadJSON(&cmd)
			if err != nil {
				return
			}
			cmds <- &cmd
		}
	}()

	return cmds
}
