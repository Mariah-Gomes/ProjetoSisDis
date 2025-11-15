import zmq

# Endereços do proxy PUB/SUB
XSUB_BIND = "tcp://*:5557"  # Publishers (servidor) conectam aqui
XPUB_BIND = "tcp://*:5558"  # Subscribers (clientes) conectam aqui

# Cria o contexto ZeroMQ
ctx = zmq.Context.instance()

# Socket XSUB: recebe mensagens dos publishers
xsub = ctx.socket(zmq.XSUB)
xsub.bind(XSUB_BIND)

# Socket XPUB: envia mensagens aos subscribers
xpub = ctx.socket(zmq.XPUB)
# Opcional: loga novas inscrições de tópicos
# xpub.setsockopt(zmq.XPUB_VERBOSE, 1)
xpub.bind(XPUB_BIND)

print(f"[proxy] Iniciado XSUB {XSUB_BIND} <-> XPUB {XPUB_BIND}")

# O proxy faz a ponte entre publishers e subscribers
# A ordem correta é (XSUB, XPUB)
zmq.proxy(xsub, xpub)

# (Só roda se o proxy encerrar)
xsub.close()
xpub.close()
ctx.term()
