package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	zmq "github.com/pebbe/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

type Envelope struct {
	Service string      `msgpack:"service"`
	Data    interface{} `msgpack:"data"`
}

func nowMillis() int64 { return time.Now().UnixMilli() }

func sendAndRecv(socket *zmq.Socket, env Envelope) (string, error) {
	msgBytes, err := msgpack.Marshal(&env)
	if err != nil {
		return "", err
	}
	_, err = socket.SendBytes(msgBytes, 0)
	if err != nil {
		return "", err
	}
	reply, err := socket.RecvMessageBytes(0)
	if err != nil {
		return "", err
	}

	// Aceita tanto uma única frame quanto multipart
	if len(reply) == 1 {
		return decodeReply(reply[0])
	}

	joined := []byte{}
	for _, part := range reply {
		joined = append(joined, part...)
	}
	return decodeReply(joined)
}

func decodeReply(b []byte) (string, error) {
	var obj interface{}
	if err := msgpack.Unmarshal(b, &obj); err != nil {
		return fmt.Sprintf("(erro decodificando resposta: %v)", err), nil
	}
	return fmt.Sprintf("%v", obj), nil
}

func ask(prompt string, reader *bufio.Reader) string {
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func main() {
	// REQ → broker
	endpoint := os.Getenv("BROKER_ENDPOINT")
	if endpoint == "" {
		endpoint = "tcp://broker:5555"
	}

	ctx, _ := zmq.NewContext()
	defer ctx.Term()

	req, _ := ctx.NewSocket(zmq.REQ)
	defer req.Close()
	if err := req.Connect(endpoint); err != nil {
		fmt.Println("Erro ao conectar no broker:", err)
		return
	}

	// SUB → proxy
	sub, _ := ctx.NewSocket(zmq.SUB)
	defer sub.Close()
	if err := sub.Connect("tcp://proxy:5558"); err != nil {
		fmt.Println("Erro ao conectar no proxy (SUB):", err)
		return
	}

	// leitor assíncrono do SUB
	go func() {
		for {
			msg, err := sub.RecvMessageBytes(0) // [topic, payload]
			if err != nil {
				fmt.Println("erro SUB:", err)
				continue
			}
			if len(msg) >= 2 {
				var payload map[string]interface{}
				if err := msgpack.Unmarshal(msg[1], &payload); err == nil {
					fmt.Printf("\n[SUB %s] %v\n", string(msg[0]), payload)
				} else {
					fmt.Printf("\n[SUB %s] (erro decodificando msgpack)\n", string(msg[0]))
				}
				fmt.Print("Escolha: ")
			}
		}
	}()

	reader := bufio.NewReader(os.Stdin)

	// Login
	user := ask("Informe seu nome de usuário para login: ", reader)
	loginEnv := Envelope{
		Service: "login",
		Data: map[string]interface{}{
			"user":      user,
			"timestamp": nowMillis(),
		},
	}
	if resp, err := sendAndRecv(req, loginEnv); err != nil {
		fmt.Println("Falha no login:", err)
		return
	} else {
		fmt.Println("Resposta do servidor:", resp)
	}

	// obrigatório: subscribe no próprio usuário (DMs)
	sub.SetSubscribe(user)
	fmt.Println("Assinado no tópico do usuário:", user)

	for {
		fmt.Println()
		fmt.Println("=== MENU ===")
		fmt.Println("1) users (listar usuários)")
		fmt.Println("2) channel (criar canal)")
		fmt.Println("3) channels (listar canais)")
		fmt.Println("4) subscribe em canal (PUB/SUB)")
		fmt.Println("5) publish em canal (PUB/SUB)")
		fmt.Println("6) message (DM para usuário)")
		fmt.Println("0) sair")
		fmt.Print("Escolha: ")

		opStr := ask("", reader)
		switch opStr {
		case "1":
			env := Envelope{Service: "users", Data: map[string]interface{}{"timestamp": nowMillis()}}
			resp, err := sendAndRecv(req, env)
			if err != nil {
				fmt.Println("Erro:", err)
				continue
			}
			fmt.Println("Resposta:", resp)

		case "2":
			ch := ask("Nome do canal: ", reader)
			if ch == "" {
				fmt.Println("Canal não pode ser vazio.")
				continue
			}
			env := Envelope{Service: "channel", Data: map[string]interface{}{"channel": ch, "timestamp": nowMillis()}}
			resp, err := sendAndRecv(req, env)
			if err != nil {
				fmt.Println("Erro:", err)
				continue
			}
			fmt.Println("Resposta:", resp)

		case "3":
			env := Envelope{Service: "channels", Data: map[string]interface{}{"timestamp": nowMillis()}}
			resp, err := sendAndRecv(req, env)
			if err != nil {
				fmt.Println("Erro:", err)
				continue
			}
			fmt.Println("Resposta:", resp)

		case "4":
			ch := ask("Canal para assinar: ", reader)
			if ch == "" {
				fmt.Println("Canal vazio.")
				continue
			}
			sub.SetSubscribe(ch)
			fmt.Println("Assinado no canal:", ch)

		case "5":
			ch := ask("Canal: ", reader)
			msg := ask("Mensagem: ", reader)
			env := Envelope{
				Service: "publish",
				Data: map[string]interface{}{
					"user":      user,
					"channel":   ch,
					"message":   msg,
					"timestamp": nowMillis(),
				},
			}
			resp, err := sendAndRecv(req, env)
			if err != nil {
				fmt.Println("Erro:", err)
				continue
			}
			fmt.Println("Resposta:", resp)

		case "6":
			dst := ask("Destino (usuário): ", reader)
			msg := ask("Mensagem: ", reader)
			env := Envelope{
				Service: "message",
				Data: map[string]interface{}{
					"src":       user,
					"dst":       dst,
					"message":   msg,
					"timestamp": nowMillis(),
				},
			}
			resp, err := sendAndRecv(req, env)
			if err != nil {
				fmt.Println("Erro:", err)
				continue
			}
			fmt.Println("Resposta:", resp)

		case "0":
			fmt.Println("Saindo…")
			return

		default:
			fmt.Println("Opção inválida.")
		}
	}
}
