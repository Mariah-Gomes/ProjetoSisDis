import zmq

context = zmq.Context()

# ROUTER recebe requisições dos clientes (porta 5555)
client_socket = context.socket(zmq.ROUTER)
client_socket.bind("tcp://*:5555")

# DEALER conecta ao servidor (que está escutando em tcp://server:5556)
server_socket = context.socket(zmq.DEALER)
server_socket.connect("tcp://server:5556")

# Proxy simples ROUTER <-> DEALER
zmq.proxy(client_socket, server_socket)

# Fechamento (só será executado se o proxy encerrar)
client_socket.close()
server_socket.close()
context.term()
