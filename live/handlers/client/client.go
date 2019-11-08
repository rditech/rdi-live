// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rditech/rdi-live/data"
	"github.com/rditech/rdi-live/live"
	"github.com/rditech/rdi-live/live/message"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proio-org/go-proio"
)

var nClients uint64

type ClientHandler struct {
	Redis  *redis.Client
	Addr   string
	MaxNPR float64
	Srv    *http.Server

	websocket.Upgrader
}

func (h *ClientHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get list of namespaces that the user has access to
	var namespaces []string

	authSession, _ := live.Store.Get(r, "auth-session")
	if appMetadata, ok := authSession.Values["app_metadata"]; ok {
		if appMetadata, ok := appMetadata.(map[string]interface{}); ok {
			if ns, ok := appMetadata["data namespaces"]; ok {
				if ns, ok := ns.([]interface{}); ok {
					for _, name := range ns {
						if name, ok := name.(string); ok {
							namespaces = append(namespaces, name)
						}
					}
				}
			}
		}
	}

	if len(namespaces) == 0 {
		namespaces = []string{"everyone"}
	}

	// Get nickname
	nickname := "nobody"
	if nick, ok := authSession.Values["nickname"]; ok {
		if nick, ok := nick.(string); ok {
			nickname = nick
		}
	}

	log.Println("starting client ws serve for", nickname, "with namespaces", namespaces)
	c, err := h.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	subClient := redis.NewClient(&redis.Options{Addr: h.Addr})
	var broadcasts []string
	for _, name := range namespaces {
		broadcasts = append(broadcasts, name+" broadcast")
	}
	sub := subClient.Subscribe(broadcasts...)
	_, err = sub.Receive()
	if err != nil {
		log.Println("PubSub.Receive():", err)
		subClient.Close()
		return
	}
	broadcast := sub.ChannelSize(10)

	ctx, cancel := context.WithCancel(context.Background())
	resp := make(chan *message.Msg)

	go func() {
		defer cancel()

		for cmd := range message.ReceiveWsCmds(ctx, c) {
			h.Execute(ctx, nickname, namespaces, cmd, resp, sub)
		}
	}()

	msgBufs := make(chan []byte, 100)
	priorityBufs := make(chan []byte, 10000)

	go func() {
		atomic.AddUint64(&nClients, 1)
		defer func() {
			log.Println("stopped client ws serve")
			time.Sleep(time.Second)
			atomic.AddUint64(&nClients, ^uint64(0))
			if h.Srv != nil && atomic.LoadUint64(&nClients) == 0 {
				log.Println("no clients, shutting down")
				h.Srv.Shutdown(context.Background())
			}
		}()
		defer subClient.Close()
		defer sub.Close()

		var buf []byte
		var msg *message.Msg
		for {
			select {
			case msg = <-resp:
				if msg == nil {
					continue
				}
				var err error
				buf, err = json.Marshal(msg)
				if err != nil {
					log.Println(err)
					continue
				}
			case redisMsg := <-broadcast:
				buf = []byte(redisMsg.Payload)
				msg = &message.Msg{}
				json.Unmarshal(buf, msg)
			case <-ctx.Done():
				return
			}

			channel := priorityBufs
			switch msg.Type {
			case "show frame", "stream status":
				channel = msgBufs
			default:
			}

			select {
			case channel <- buf:
			default:
			}
		}
	}()

	go func() {
		for {
			msg := &message.Msg{
				Type:     "system status",
				Metadata: make(map[string]string),
			}

			idle0, total0 := getCPUSample()
			time.Sleep(time.Second)
			idle1, total1 := getCPUSample()
			idleTicks := float64(idle1 - idle0)
			totalTicks := float64(total1 - total0)
			cpuUsage := (totalTicks - idleTicks) / totalTicks
			if !math.IsNaN(cpuUsage) {
				msg.Metadata["usage"] = fmt.Sprintf("%v", cpuUsage)
			}

			memStats := &runtime.MemStats{}
			runtime.ReadMemStats(memStats)
			msg.Metadata["mem alloc"] = fmt.Sprintf("%v", uint32(memStats.Alloc)/(2<<20))
			msg.Metadata["mem sys"] = fmt.Sprintf("%v", uint32(memStats.Sys)/(2<<20))

			select {
			case <-ctx.Done():
				return
			default:
				if buf, err := json.Marshal(msg); err == nil {
					priorityBufs <- buf
				}
			}
		}
	}()

	go func() {
		var buf []byte
		var npr float64
		last := time.Now()
		for {
			now := time.Now()
			alpha := now.Sub(last).Seconds()
			last = now
			if alpha > 1 {
				alpha = 1
			}
			npr *= 1 - alpha

			select {
			case buf = <-priorityBufs:
				for len(msgBufs) > 0 {
					select {
					case <-msgBufs:
					}
				}
			default:
				select {
				case buf = <-priorityBufs:
				case buf = <-msgBufs:
					if npr < h.MaxNPR {
						npr += 1
					} else {
						buf = nil
					}
				case <-ctx.Done():
					return
				}
			}

			if buf != nil {
				if err := c.WriteMessage(websocket.TextMessage, buf); err != nil {
					log.Println(err)
				}
			}
		}
	}()

}

func (h *ClientHandler) Execute(
	ctx context.Context,
	nickname string,
	namespaces []string,
	cmd *message.Cmd,
	resp chan<- *message.Msg,
	sub *redis.PubSub,
) {
	log.Println("ClientHandler:", cmd.Command)

	switch cmd.Command {
	case "get nickname":
		h.GetNickname(nickname, cmd, resp)
	case "list streams":
		h.ListStreams(namespaces, cmd, resp)
	case "stream cmd":
		h.StreamCmd(namespaces, cmd)
	case "stream sub":
		h.StreamSub(namespaces, cmd, sub, resp)
	case "stream unsub":
		h.StreamUnsub(namespaces, cmd, sub, resp)
	case "ls":
		h.ListResourceRuns(ctx, cmd, resp)
	case "get meta":
		h.GetRunMetadata(ctx, cmd, resp)
	case "play run":
		h.PlayRun(namespaces, ctx, cmd, resp)
	default:
		log.Printf("unknown command\n%v", cmd)
	}
}

func (h *ClientHandler) GetNickname(nickname string, cmd *message.Cmd, resp chan<- *message.Msg) {
	msg := &message.Msg{
		Metadata: make(map[string]string),
	}
	msg.Type = "nickname"
	msg.Metadata["name"] = nickname
	resp <- msg
}

func (h *ClientHandler) ListStreams(namespaces []string, cmd *message.Cmd, resp chan<- *message.Msg) {
	for _, namespace := range namespaces {
		for _, stream := range h.Redis.PubSubChannels(namespace + " stream cmd *").Val() {
			msg := &message.Msg{
				Metadata: make(map[string]string),
			}
			msg.Type = "stream announce"
			msg.Metadata["name"] = strings.TrimPrefix(stream, namespace+" stream cmd ")
			resp <- msg
		}
	}
}

func (h *ClientHandler) StreamCmd(namespaces []string, cmd *message.Cmd) {
	stream := cmd.Metadata["stream"]
	cmd.Command = cmd.Metadata["stream cmd"]
	delete(cmd.Metadata, "stream")
	delete(cmd.Metadata, "stream cmd")
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		log.Println(err)
		return
	}

	for _, namespace := range namespaces {
		h.Redis.Publish(namespace+" stream cmd "+stream, string(cmdBytes))
	}
}

func (h *ClientHandler) StreamSub(namespaces []string, cmd *message.Cmd, sub *redis.PubSub, resp chan<- *message.Msg) {
	stream := cmd.Metadata["stream"]
	for _, namespace := range namespaces {
		channel := namespace + " stream " + stream
		log.Println("sub to", channel)
		sub.Subscribe(channel)
	}

	msg := &message.Msg{
		Type:     "stream sub",
		Metadata: make(map[string]string),
	}
	msg.Metadata["stream"] = stream
	resp <- msg
}

func (h *ClientHandler) StreamUnsub(namespaces []string, cmd *message.Cmd, sub *redis.PubSub, resp chan<- *message.Msg) {
	stream := cmd.Metadata["stream"]
	for _, namespace := range namespaces {
		channel := namespace + " stream " + stream
		log.Println("unsub from", channel)
		sub.Unsubscribe(channel)
	}

	msg := &message.Msg{
		Type:     "stream unsub",
		Metadata: make(map[string]string),
	}
	msg.Metadata["stream"] = stream
	resp <- msg
}

func (h *ClientHandler) ListResourceRuns(ctx context.Context, cmd *message.Cmd, resp chan<- *message.Msg) {
	go func() {
		msg := &message.Msg{
			Type:     "run list",
			Metadata: make(map[string]string),
		}
		msg.Metadata["name"] = cmd.Metadata["name"]
		msg.Metadata["status"] = "failure"
		msg.Metadata["url"] = cmd.Metadata["url"]
		defer func() { resp <- msg }()

		runs, err := data.ListResourceRuns(ctx, cmd.Metadata["url"], cmd.Metadata["credentials"])
		if err != nil {
			msg.Payload = []byte(err.Error())
			return
		}
		msg.Payload, err = json.Marshal(runs)
		if err != nil {
			msg.Payload = []byte(err.Error())
			return
		}

		msg.Metadata["status"] = "success"
	}()
}

func (h *ClientHandler) GetRunMetadata(ctx context.Context, cmd *message.Cmd, resp chan<- *message.Msg) {
	go func() {
		msg := &message.Msg{
			Type:     "run meta",
			Metadata: make(map[string]string),
		}
		msg.Metadata["status"] = "failure"
		msg.Metadata["url"] = cmd.Metadata["url"]
		defer func() { resp <- msg }()

		reader, err := data.GetReader(ctx, cmd.Metadata["url"], cmd.Metadata["credentials"])
		if err != nil {
			msg.Payload = []byte(err.Error())
			return
		}

		reader.Skip(0)
		msg.Payload, err = json.Marshal(reader.Metadata)
		if err != nil {
			msg.Payload = []byte(err.Error())
			return
		}

		reader.Close()

		msg.Metadata["status"] = "success"
	}()
}

func (h *ClientHandler) PlayRun(namespaces []string, ctx context.Context, cmd *message.Cmd, resp chan<- *message.Msg) {
	msg := &message.Msg{
		Type:     "player failure",
		Metadata: make(map[string]string),
	}
	msg.Metadata["url"] = cmd.Metadata["url"]
	log.Println("url:", cmd.Metadata["url"])

	reader, err := data.GetReader(ctx, cmd.Metadata["url"], cmd.Metadata["credentials"])
	if err != nil {
		msg.Payload = []byte(err.Error())
		resp <- msg
		return
	}

	thisUrl, err := url.Parse(cmd.Metadata["url"])
	streamName := path.Base(thisUrl.Path)
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	go func() {
		defer cancel()

		reader.Skip(0)
		uidBytes, ok := reader.Metadata["UID"]
		if !ok || len(uidBytes) != 8 {
			log.Println("falling back to random UID")
			uuidBytes := [16]byte(uuid.New())
			uidBytes = uuidBytes[:8]
		}
		uid := binary.BigEndian.Uint64(uidBytes)

		input := make(chan *proio.Event)
		go func() {
			defer close(input)

			log.Println("player reader for", thisUrl, "started")
			defer log.Println("player reader for", thisUrl, "stopped")

			for {
				for event := range reader.ScanEvents(1000) {
					select {
					case input <- event:
					case <-ctx.Done():
						reader.Lock()
						reader.Close()
						reader.Unlock()
						return
					}
				}
				reader.Close()
				reader, err = data.GetReader(ctx, cmd.Metadata["url"], cmd.Metadata["credentials"])
				if err != nil {
					msg.Payload = []byte(err.Error())
					resp <- msg
					return
				}
			}
		}()

		ops := live.BuildPlayer(namespaces[len(namespaces)-1], streamName, h.Redis, h.Addr, uid)
		if ops != nil {
			log.Println("player for", thisUrl, "started")
			defer log.Println("player for", thisUrl, "stopped")
			ops.Sink(input)
		}
	}()
}

// CPU sampling for usage calculation
func getCPUSample() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					fmt.Println("Error: ", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}
