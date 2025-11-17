package main

// Importações necessárias
import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	zmq "github.com/pebbe/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

// Definição da estrutura do Envelope
type Envelope struct {
	Service string      `msgpack:"service"`
	Data    interface{} `msgpack:"data"`
}

// Retorna o tempo atual
func nowMillis() int64 {
	return time.Now().UnixMilli()
}

// Relógio lógico local do cliente
var logicalClock int64 = 0

// Incrementa o relógio antes de enviar uma mensagem
func tick() int64 {
	logicalClock++
	return logicalClock
}

// Atualiza o relógio lógico ao receber um clock remoto (vamos usar depois)
func updateClock(remote int64) {
	if remote > logicalClock {
		logicalClock = remote
	}
}

// Mensagem
func sendAndRecv(socket *zmq.Socket, env Envelope) (string, error) {
	msgBytes, err := msgpack.Marshal(&env) // Empacota o envelope em MessagePack
	if err != nil {
		return "", err
	}

	_, err = socket.SendBytes(msgBytes, 0) // Envia a mensagem
	if err != nil {
		return "", err
	}

	reply, err := socket.RecvMessageBytes(0) // Recebe a resposta
	if err != nil {
		return "", err
	}

	// Uma parte
	if len(reply) == 1 {
		return decodeReply(reply[0])
	}

	// Multipart
	joined := []byte{}
	for _, part := range reply {
		joined = append(joined, part...)
	}
	return decodeReply(joined)
}

// Essa função pega a resposta msgpack do servidor, converte para um valor Go e transforma isso em uma string legível para o usuário.
func decodeReply(b []byte) (string, error) {
	var obj interface{}
	if err := msgpack.Unmarshal(b, &obj); err != nil {
		return fmt.Sprintf("(erro decodificando resposta: %v)", err), nil // Se der erro, retorna mensagem de erro
	}
	return fmt.Sprintf("%v", obj), nil // Retorna a representação em string do objeto
}

// Função para ler entrada do usuário existe para não repetir código
func ask(prompt string, reader *bufio.Reader) string {
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func main() {
	// Conexão com o broker
	endpoint := os.Getenv("BROKER_ENDPOINT")
	if endpoint == "" {
		endpoint = "tcp://broker:5555"
	}

	// contexto do ZeroMQ
	ctx, _ := zmq.NewContext()
	defer ctx.Term()

	// Criei um socket REQ, conectei no broker, se der ruim eu paro tudo
	req, _ := ctx.NewSocket(zmq.REQ)
	defer req.Close()
	if err := req.Connect(endpoint); err != nil {
		fmt.Println("Erro ao conectar no broker:", err)
		return
	}

	// riei um socket PUB, conectei no proxy, se der ruim eu paro tudo
	sub, _ := ctx.NewSocket(zmq.SUB)
	defer sub.Close()
	if err := sub.Connect("tcp://proxy:5558"); err != nil {
		fmt.Println("Erro ao conectar no proxy (SUB):", err)
		return
	}

	// leitor assíncrono do SUB
	go func() {
		for { // Loop infinito
			msg, err := sub.RecvMessageBytes(0) // Recebe a mensagem
			if err != nil {                     // Tratamento de erro
				fmt.Println("erro SUB:", err)
				continue
			}
			if len(msg) >= 2 { // Verifica se tem pelo menos 2 partes
				var payload map[string]interface{}                          // Transforma em "dicionário"
				if err := msgpack.Unmarshal(msg[1], &payload); err == nil { // Decodifica e testa se deu certo
					fmt.Printf("\n[SUB %s] %v\n", string(msg[0]), payload) // Imprime se deu certo
				} else {
					fmt.Printf("\n[SUB %s] (erro decodificando msgpack)\n", string(msg[0])) // Imprime erro
				}
				fmt.Print("Escolha: ") // Escolhe do menu
			}
		}
	}()

	reader := bufio.NewReader(os.Stdin) // Continua fluxo principal

	// Login
	user := ask("Informe seu nome de usuário para login: ", reader) // Informa o nome do usuário
	// Cria o envelope de login
	loginEnv := Envelope{
		Service: "login",
		Data: map[string]interface{}{
			"user":      user,
			"timestamp": nowMillis(),
			"clock":     tick(),
		},
	}
	// Envia o envelope de login e trata a resposta
	if resp, err := sendAndRecv(req, loginEnv); err != nil {
		fmt.Println("Falha no login:", err)
		return
	} else {
		fmt.Println("Resposta do servidor:", resp)
	}

	// Está logado, agora assina o tópico do usuário para mensagens diretas
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
			env := Envelope{
				Service: "channel",
				Data: map[string]interface{}{
					"channel":   ch,
					"timestamp": nowMillis(),
					"clock":     tick(), 
				},
			}
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
			env := Envelope{
				Service: "channel", 
				Data: map[string]interface{}{
					"channel": ch, 
					"timestamp": nowMillis(),
					"clock":     tick(),		
				},
			}
			resp, err := sendAndRecv(req, env)
			if err != nil {
				fmt.Println("Erro:", err)
				continue
			}
			fmt.Println("Resposta:", resp)

		case "3":
			env := Envelope{
				Service: "channels", 
				Data: map[string]interface{}{
					"timestamp": nowMillis(),
					"clock":     tick(),
				}
			}
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
					"clock":     tick(),
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
					"clock":     tick(),
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
