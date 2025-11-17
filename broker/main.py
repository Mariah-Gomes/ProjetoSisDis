import zmq

context = zmq.Context()

# ROUTER recebe requisições dos clientes/bots (porta 5555)
client_socket = context.socket(zmq.ROUTER)
client_socket.bind("tcp://*:5555")

# DEALER conecta aos servidores (que escutam em tcp://serverX:5556)
server_socket = context.socket(zmq.DEALER)

# Conecta em TODAS as instâncias de servidor
for name in ["server1", "server2", "server3"]:
    endpoint = f"tcp://{name}:5556"
    print("[broker] conectando backend em", endpoint)
    server_socket.connect(endpoint)

print("[broker] ROUTER :5555  <->  DEALER -> server1/2/3:5556")

# Proxy simples ROUTER <-> DEALER (balanceamento round-robin)
zmq.proxy(client_socket, server_socket)

# Fechamento (só será executado se o proxy encerrar)
client_socket.close()
server_socket.close()
context.term()
