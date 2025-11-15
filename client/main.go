package main

import (
	"bufio"
	"bytes"
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

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func hintIfJSON(b []byte, stage string) {
	x := bytes.TrimSpace(b)
	if len(x) > 0 && (x[0] == '{' || x[0] == '[') {
		fmt.Printf("[diag] %s parece JSON (começa com %q). O server provavelmente não está usando MessagePack.\n", stage, x[0])
	}
}

func sendAndRecv(req *zmq.Socket, env Envelope) (Envelope, error) {
	msgBytes, err := msgpack.Marshal(&env)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal msgpack: %w", err)
	}
	if _, err := req.SendBytes(msgBytes, 0); err != nil {
		return Envelope{}, fmt.Errorf("send REQ: %w", err)
	}
	replyBytes, err := req.RecvBytes(0)
	if err != nil {
		return Envelope{}, fmt.Errorf("recv REP: %w", err)
	}
	var resp Envelope
	if err := msgpack.Unmarshal(replyBytes, &resp); err != nil {
		hintIfJSON(replyBytes, "Resposta REP")
		return Envelope{}, fmt.Errorf("unmarshal msgpack: %w (len=%d)", err, len(replyBytes))
	}
	return resp, nil
}

func main() {
	// usa container_name por padrão; pode sobrescrever com envs
	broker := getenv("BROKER_ENDPOINT", "tcp://sd-broker:5555")
	proxy := getenv("PROXY_ENDPOINT", "tcp://sd-proxy:5558")
	fmt.Println("[endpoints] REQ→", broker, " | SUB→", proxy)

	// REQ para broker
	req, err := zmq.NewSocket(zmq.REQ)
	if err != nil {
		panic(err)
	}
	defer req.Close()
	_ = req.SetRcvtimeo(3 * time.Second) // evita travar esperando resposta
	_ = req.SetSndtimeo(3 * time.Second)
	_ = req.SetLinger(0)
	if err := req.Connect(broker); err != nil {
		panic(err)
	}

	// SUB para proxy
	sub, err := zmq.NewSocket(zmq.SUB)
	if err != nil {
		panic(err)
	}
	defer sub.Close()
	_ = sub.SetLinger(0)
	if err := sub.Connect(proxy); err != nil {
		panic(err)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Informe seu nome de usuário para login: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	// Tenta login, mas não bloqueia o menu se falhar
	if resp, err := sendAndRecv(req, Envelope{
		Service: "login",
		Data:    map[string]interface{}{"username": username, "timestamp": nowMillis()},
	}); err != nil {
		fmt.Println("[aviso] login não confirmado agora:", err)
		fmt.Println("[aviso] verifique se o server está rodando e em MessagePack; abrindo menu mesmo assim.")
	} else {
		fmt.Println("Resposta do servidor:", resp)
	}

	for {
		fmt.Println("\n=== MENU ===")
		fmt.Println("1) Assinar canal")
		fmt.Println("2) Publicar mensagem")
		fmt.Println("3) Enviar DM")
		fmt.Println("4) Ouvir mensagens (bloqueante)")
		fmt.Println("0) Sair")
		fmt.Print("> ")
		opt, _ := reader.ReadString('\n')
		opt = strings.TrimSpace(opt)

		switch opt {
		case "1":
			fmt.Print("Canal para assinar (ex: general): ")
			ch, _ := reader.ReadString('\n')
			ch = strings.TrimSpace(ch)
			if ch == "" {
				fmt.Println("Canal vazio.")
				continue
			}
			_, _ = sendAndRecv(req, Envelope{
				Service: "channel",
				Data:    map[string]interface{}{"name": ch},
			})
			if err := sub.SetSubscribe(ch); err != nil {
				fmt.Println("Erro ao assinar:", err)
			} else {
				fmt.Println("Assinado em:", ch)
			}
		case "2":
			fmt.Print("Canal: ")
			ch, _ := reader.ReadString('\n')
			ch = strings.TrimSpace(ch)
			fmt.Print("Mensagem: ")
			msg, _ := reader.ReadString('\n')
			msg = strings.TrimSpace(msg)

			resp, err := sendAndRecv(req, Envelope{
				Service: "publish",
				Data: map[string]interface{}{
					"channel":   ch,
					"author":    username,
					"message":   msg,
					"timestamp": nowMillis(),
				},
			})
			if err != nil {
				fmt.Println("Erro publicar:", err)
			} else {
				fmt.Println("Resposta:", resp)
			}
		case "3":
			fmt.Print("Para (username): ")
			to, _ := reader.ReadString('\n')
			to = strings.TrimSpace(to)
			fmt.Print("Mensagem: ")
			msg, _ := reader.ReadString('\n')
			msg = strings.TrimSpace(msg)

			resp, err := sendAndRecv(req, Envelope{
				Service: "message",
				Data: map[string]interface{}{
					"to":        to,
					"from":      username,
					"message":   msg,
					"timestamp": nowMillis(),
				},
			})
			if err != nil {
				fmt.Println("Erro DM:", err)
			} else {
				fmt.Println("Resposta:", resp)
			}
		case "4":
			fmt.Println("Aguardando mensagens de canais assinados (Ctrl+C para parar)...")
			for {
				frames, err := sub.RecvMessageBytes(0)
				if err != nil {
					fmt.Println("Erro SUB:", err)
					break
				}
				if len(frames) != 2 {
					fmt.Printf("Mensagem SUB malformada: esperado 2 frames, veio %d\n", len(frames))
					continue
				}
				topic := string(frames[0])
				payload := frames[1]
				var obj map[string]interface{}
				if err := msgpack.Unmarshal(payload, &obj); err != nil {
					hintIfJSON(payload, "Payload SUB")
					fmt.Println("Falha msgpack:", err)
					continue
				}
				fmt.Printf("[%s] %v\n", topic, obj)
			}
		case "0":
			fmt.Println("Tchau!")
			return
		default:
			fmt.Println("Opção inválida.")
		}
	}
}
