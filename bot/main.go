package main

import (
	"fmt"
	"math/rand"
	"time"

	zmq "github.com/pebbe/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

// Definição da estrutura do Envelope
type Envelope struct {
	Service string      `msgpack:"service"`
	Data    interface{} `msgpack:"data"`
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}

// Relógio lógico do bot
var logicalClock int64 = 0

// Incrementa o relógio antes de enviar qualquer mensagem
func tick() int64 {
	logicalClock++
	return logicalClock
}

// Atualiza o relógio lógico a partir de um clock remoto
func updateClock(remote int64) {
	if remote > logicalClock {
		logicalClock = remote
	}
}

func sendAndRecv(requisicao *zmq.Socket, envio Envelope) error {
	msg, err := msgpack.Marshal(&envio) // Empacotar o envelope em MessagePack
	// Verificação de erro no empacotamento
	if err != nil {
		return err
	}

	_, err = requisicao.SendBytes(msg, 0) //Enviar pelo socket REQ
	// Verifição de erro no envio
	if err != nil {
		return err
	}

	_, err = requisicao.RecvBytes(0) // Receber/esperar a resposta
	return err
}

func main() {
	rand.Seed(time.Now().UnixNano())                      // Inicializa o gerador de números aleatórios
	username := fmt.Sprintf("bot_%04d", rand.Intn(10000)) // Gera um nome de usuário aleatório

	requisicao, _ := zmq.NewSocket(zmq.REQ) // Cria um socket REQ
	defer requisicao.Close()                // Fecha o socket ao final da função main
	requisicao.Connect("tcp://broker:5555") // Conecta ao broker

	// Primeiro passo: login
	// Chama a função e ignora o erro
	_ = sendAndRecv(requisicao, Envelope{
		Service: "login",
		// Cria um dicionário (tipo das chaves e valores)
		Data: map[string]interface{}{
			"username":  username,
			"timestamp": nowMillis(),
			"clock":     tick(),
		},
	})

	// Segundo passo: canais
	channels := []string{"general", "random", "news"} // Lista de canais
	// Escolhe um item da lista
	for _, ch := range channels {
		_ = sendAndRecv(requisicao, Envelope{ // envia o envelope
			Service: "channel",
			Data: map[string]interface{}{
				"name":      ch,
				"timestamp": nowMillis(),
				"clock":     tick(),
			},
		})
	}

	// Terceiro passo: publicação
	// loop de publicação
	i := 0
	for {
		ch := channels[rand.Intn(len(channels))]                 // Escolhe um canal aleatório
		msg := fmt.Sprintf("[%s] msg %d de %s", ch, i, username) // Cria a mensagem
		_ = sendAndRecv(requisicao, Envelope{                    // envia o envelope
			Service: "publish",
			Data: map[string]interface{}{
				"channel":   ch,
				"author":    username,
				"message":   msg,
				"timestamp": nowMillis(),
				"clock":     tick(),
			},
		})
		i++
		time.Sleep(500 * time.Millisecond)
	}
}
