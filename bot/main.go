package main

import (
	"fmt"
	"math/rand"
	"time"

	zmq "github.com/pebbe/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

type Envelope struct {
	Service string      `msgpack:"service"`
	Data    interface{} `msgpack:"data"`
}

func nowMillis() int64 { return time.Now().UnixMilli() }

func sendAndRecv(req *zmq.Socket, env Envelope) error {
	msg, err := msgpack.Marshal(&env)
	if err != nil {
		return err
	}
	_, err = req.SendBytes(msg, 0)
	if err != nil {
		return err
	}
	_, err = req.RecvBytes(0) // ignoramos conteúdo da resposta
	return err
}

func main() {
	rand.Seed(time.Now().UnixNano())
	username := fmt.Sprintf("bot_%04d", rand.Intn(10000))

	req, _ := zmq.NewSocket(zmq.REQ)
	defer req.Close()
	req.Connect("tcp://broker:5555")

	// login
	_ = sendAndRecv(req, Envelope{
		Service: "login",
		Data: map[string]interface{}{
			"username":  username,
			"timestamp": nowMillis(),
		},
	})

	// garante canais
	channels := []string{"general", "random", "news"}
	for _, ch := range channels {
		_ = sendAndRecv(req, Envelope{
			Service: "channel",
			Data:    map[string]interface{}{"name": ch},
		})
	}

	// loop de publicação
	i := 0
	for {
		ch := channels[rand.Intn(len(channels))]
		msg := fmt.Sprintf("[%s] msg %d de %s", ch, i, username)
		_ = sendAndRecv(req, Envelope{
			Service: "publish",
			Data: map[string]interface{}{
				"channel":   ch,
				"author":    username,
				"message":   msg,
				"timestamp": nowMillis(),
			},
		})
		i++
		time.Sleep(500 * time.Millisecond)
	}
}
