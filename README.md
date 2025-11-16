# Projeto de Sistemas Distribuídos

> Esse projeto foi desenvolvido no 7º Semestre

> Durante a disciplina de Sistemas Distribuídos

### Tópicos


## Descrição geral da arquitetura
O sistema é composto por múltiplos serviços distribuídos que se comunicam utilizando ZeroMQ. Ele implementa dois fluxos principais:
- REQ/REP (comandos entre cliente/bot ↔ servidor)
- PUB/SUB (eventos de canais e mensagens diretas ↔ clientes)

**Todos os dados trafegam serializados em MessagePack, e o servidor registra eventos com relógio lógico para garantir ordenação e consistência. Cada componente roda em um contêiner Docker isolado, simulando um ambiente distribuído real.**

## Componentes e decisões de projeto
#### Cliente (Go)
- Conecta ao Broker usando REQ/REP para enviar comandos.
- Conecta ao Proxy usando SUB para receber publicações.
- Assina automaticamente o tópico do próprio username (DMs).

**Decisão de projeto:** Escolhemos Go para o cliente porque ele combina muito bem com um cliente de linha de comando concorrente: goroutines para ouvir o SUB em paralelo ao menu, binário único fácil de empacotar em Docker e tipagem forte para reduzir erros na montagem das mensagens.

#### Bot (Go)
- Reutiliza a mesma estrutura do cliente.
- Gera publicações automáticas e cria canais quando necessário.
- Pode rodar múltiplas instâncias sem alterar código.

**Decisão de projeto:** O bot foi feito em Go porque precisa realizar tarefas periódicas e concorrentes de forma leve. Goroutines facilitam publicar automaticamente em canais enquanto o SUB fica ouvindo mensagens. Como o Go gera binários independentes, é fácil rodar várias instâncias do bot no Docker sem esforço extra. Além disso, reutiliza o mesmo código-base do cliente, mantendo consistência no uso de MessagePack e ZeroMQ.

#### Servidor (Node.js)
- Responsável por toda lógica de negócio (login, canais, DMs, publicações).
- Responde comandos via REQ/REP e publica eventos via PUB.
- Persiste dados em arquivos NDJSON separados (logins, canais, mensagens, publicações).

**Decisão de projeto:** O servidor foi feito em Node.js porque o modelo de I/O assíncrono do JavaScript se encaixa bem em um servidor de mensagens com muitas requisições pequenas, e o ecossistema de bibliotecas facilita o uso de ZeroMQ, MessagePack e arquivos NDJSON.

#### Broker (Python)
- Intermediário entre clientes/bots e servidor no fluxo REQ/REP.
- Usa ROUTER / DEALER e zmq.proxy.
- Não interpreta mensagens, apenas encaminha.

**Decisão de projeto:** simplificar o acoplamento e permitir N clientes ↔ 1 servidor sem bloquear. E também, por conta da prática nas aulas.

#### Proxy (Python)
- Mediação do fluxo PUB/SUB usando XSUB / XPUB.
- Encaminha publicações do servidor para todos os clientes inscritos.
- Transparente, também implementado com zmq.proxy.

**Decisão de projeto:** separar tráfego de comandos e publicações aumenta escalabilidade.  E também, por conta da prática nas aulas.

## Fluxo das mensagens
#### Fluxo REQ/REP (cliente/bot → broker → servidor)
- Cliente/Bot cria um envelope MessagePack.
- Envia para o Broker (socket REQ).
- Broker repassa ao servidor (DEALER).
- Servidor processa e devolve resposta.
- Broker envia resposta ao cliente.

#### Fluxo PUB/SUB (servidor → proxy → clientes)
- Servidor publica um evento em um tópico (canal ou nome de usuário).
- Proxy recebe via XSUB e redistribui via XPUB.
- Clientes recebem apenas os tópicos inscritos.

<!--5. Relógio lógico

Cada evento (login, publicação, DM) recebe um timestamp lógico.

O servidor atualiza o relógio a cada evento recebido.

Garante ordenação parcial dos eventos distribuídos.

Decisão de projeto: evitar inconsistências entre mensagens próximas no tempo.
-->

## Explicação de como rodar o projeto
#### PASSO 1
Precisa ter o Docker `Desktop instalado` em seu computador e iniciar o programa

#### PASSO 2
Precisa ter o `VS Code` instalado em seu computar e iniciar o programa

#### PASSO 3
Ao realizar o `git clone` desse repositório abrir o terminal do `VS Code` com o seguinte comando:
```
CTRL + SHIFT + '
```

#### PASSO 4
Ao abrir o terminal limpo no VS Code digitar o seguinte comando
```
docker compose build
```

#### PASSO 5
Quando acabar de buildar os conteiners rodar o seguinte comando
```
docker compose run --rm -it client
```
