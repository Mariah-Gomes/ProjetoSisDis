// server/server.js
import { Reply, Publisher } from "zeromq";
import { pack, unpack } from "msgpackr";
import fs from "fs-extra";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname  = path.dirname(__filename);

// === Arquivos de persistência (NDJSON) ===
const DATA_DIR = "/app/data";
const LOGINS   = path.join(DATA_DIR, "logins.ndjson");
const CHANNELS = path.join(DATA_DIR, "channels.ndjson");
const PUBS     = path.join(DATA_DIR, "publications.ndjson");
const DMS      = path.join(DATA_DIR, "direct_messages.ndjson");

await fs.ensureDir(DATA_DIR);
await Promise.all([
  fs.ensureFile(LOGINS),
  fs.ensureFile(CHANNELS),
  fs.ensureFile(PUBS),
  fs.ensureFile(DMS),
]);

async function appendNdjson(filePath, obj) {
  await fs.appendFile(filePath, JSON.stringify(obj) + "\n");
}

async function ensureChannel(name) {
  // Se ainda não existir no arquivo, grava um registro de criação simples
  const txt = await fs.readFile(CHANNELS, "utf8").catch(() => "");
  if (!txt.includes(`"channel":"${name}"`)) {
    await appendNdjson(CHANNELS, { channel: name, createdAt: Date.now() });
  }
}

// === Sockets ZeroMQ ===
const rep = new Reply();      // REQ/REP
const pub = new Publisher();  // PUB/SUB

// ⚠️ NÃO vamos alterar broker/proxy.
// Seu broker hoje faz DEALER.connect("tcp://server:5556").
// Portanto o SERVER deve ESCUTAR em :5556 (bind).
await rep.bind("tcp://*:5556");            // <- casa com o seu broker atual
await pub.connect("tcp://proxy:5557");     // <- conecta no XSUB do proxy

console.log("[server] REP bind :5556  |  PUB → proxy:5557");
console.log("[server] aguardando REQs...");

// Envia publicação para assinantes: 2 frames (topic string, payload msgpack)
function publishChannel(topic, payloadObj) {
  pub.send([topic, pack(payloadObj)]);
}

for await (const [msgBytes] of rep) {
  let env;
  try {
    env = unpack(msgBytes); // decodifica MessagePack recebido do cliente
  } catch (e) {
    console.error("[server] msgpack inválido no REQ:", e?.message);
    await rep.send(pack({ service: "error", data: { status: "erro", message: "msgpack inválido" } }));
    continue;
  }

  const { service, data } = env || {};
  console.log("[server] REQ:", service);

  let reply = { service, data: { status: "sucesso", timestamp: Date.now() } };

  try {
    switch (service) {
      case "login": {
        const username = String(data?.username || "").trim();
        if (!username) throw new Error("username obrigatório");
        await appendNdjson(LOGINS, { username, timestamp: Date.now(), type: "login" });
        reply.data.description = "Login registrado";
        break;
      }

      case "channel": {
        const name = String(data?.name || "").trim();
        if (!name) throw new Error("nome de canal obrigatório");
        await ensureChannel(name);
        reply.data.description = "Canal assegurado";
        break;
      }

      case "publish": {
        const channel = String(data?.channel || "").trim();
        const author  = String(data?.author  || "").trim();
        const message = String(data?.message || "");
        if (!channel) throw new Error("canal obrigatório");

        const rec = { channel, author, message, timestamp: Date.now(), type: "publication" };
        await appendNdjson(PUBS, rec);

        // publica no proxy (XPUB/XSUB) — payload em MessagePack
        publishChannel(channel, rec);

        reply.data.description = "Publicado";
        break;
      }

      case "message": {
        const to   = String(data?.to   || "").trim();
        const from = String(data?.from || "").trim();
        const message = String(data?.message || "");
        if (!to || !from) throw new Error("to e from obrigatórios");

        const rec = { to, from, message, timestamp: Date.now(), type: "dm" };
        await appendNdjson(DMS, rec);

        reply.data.description = "DM registrada";
        break;
      }

      default:
        reply = { service: "error", data: { status: "erro", message: `serviço desconhecido: ${service}` } };
    }
  } catch (err) {
    reply = { service: "error", data: { status: "erro", message: err.message } };
  }

  await rep.send(pack(reply)); // responde em MessagePack
  console.log("[server] REP enviado:", service);
}
