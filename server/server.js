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

async function readNdjson(filePath) {
  const txt = await fs.readFile(filePath, "utf8").catch(() => "");
  return txt
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean)
    .map((l) => {
      try { return JSON.parse(l); } catch { return null; }
    })
    .filter(Boolean);
}

async function ensureChannel(name) {
  const rows = await readNdjson(CHANNELS);
  if (!rows.some((r) => r?.channel === name)) {
    await appendNdjson(CHANNELS, { channel: name, createdAt: Date.now() });
  }
}

// === Sockets ZeroMQ ===
const rep = new Reply();
const pub = new Publisher();

// ✅ compatível com o SEU broker/proxy:
// broker: DEALER.connect("tcp://server:5556")  → aqui precisa bind
// proxy:  XSUB.bind :5557 / XPUB.bind :5558   → aqui conectamos no XSUB
await rep.bind("tcp://*:5556");
await pub.connect("tcp://proxy:5557");

console.log("[server] REP bind :5556  |  PUB → proxy:5557");
console.log("[server] aguardando REQs...");

function publishTopic(topic, payloadObj) {
  // 2 frames: [topic, msgpack(payload)]
  pub.send([topic, pack(payloadObj)]);
}

for await (const [msgBytes] of rep) {
  let env;
  try {
    env = unpack(msgBytes); // Request em MessagePack vindo do client
  } catch (e) {
    console.error("[server] msgpack inválido no REQ:", e?.message);
    await rep.send(pack({
      service: "error",
      data: { status: "erro", message: "msgpack inválido" },
    }));
    continue;
  }

  const { service, data } = env || {};
  console.log("[server] REQ:", service, data);

  let reply = { service, data: { status: "ok", timestamp: Date.now() } };

  try {
    switch (service) {
      // =========================================================
      // login: Data { user, timestamp }
      // =========================================================
      case "login": {
        const user = String(data?.user || data?.username || "").trim();
        if (!user) throw new Error("user obrigatório");
        const rec = {
          user,
          timestamp: Number(data?.timestamp) || Date.now(),
          type: "login",
        };
        await appendNdjson(LOGINS, rec);
        reply.data.message = "Login registrado";
        reply.data.user = user;
        break;
      }

      // =========================================================
      // users: lista usuários que já fizeram login
      // =========================================================
      case "users": {
        const rows = await readNdjson(LOGINS);
        const set = new Set(
          rows
            .map((r) => r.user || r.username)
            .filter((u) => typeof u === "string" && u.trim() !== "")
        );
        reply.data.users = Array.from(set);
        break;
      }

      // =========================================================
      // channel: Data { channel, timestamp }
      // cria/assegura canal
      // =========================================================
      case "channel": {
        const name = String(data?.channel || data?.name || "").trim();
        if (!name) throw new Error("channel obrigatório");
        await ensureChannel(name);
        const rec = {
          channel: name,
          timestamp: Number(data?.timestamp) || Date.now(),
          type: "channel",
        };
        await appendNdjson(CHANNELS, rec);
        reply.data.message = "Canal assegurado";
        reply.data.channel = name;
        break;
      }

      // =========================================================
      // channels: lista canais
      // =========================================================
      case "channels": {
        const rows = await readNdjson(CHANNELS);
        const set = new Set(
          rows
            .map((r) => r.channel)
            .filter((c) => typeof c === "string" && c.trim() !== "")
        );
        reply.data.channels = Array.from(set);
        break;
      }

      // =========================================================
      // publish: Data { user, channel, message, timestamp }
      // grava e publica no tópico igual ao nome do canal
      // =========================================================
      case "publish": {
        const user = String(data?.user || "").trim();
        const channel = String(data?.channel || "").trim();
        const message = String(data?.message || "");
        if (!channel) throw new Error("channel obrigatório");

        await ensureChannel(channel);

        const rec = {
          user,
          channel,
          message,
          timestamp: Number(data?.timestamp) || Date.now(),
          type: "publication",
        };
        await appendNdjson(PUBS, rec);

        // PUB/SUB: tópico = nome do canal
        publishTopic(channel, rec);

        reply.data.message = "Publicado no canal";
        reply.data.channel = channel;
        break;
      }

      // =========================================================
      // message: Data { src, dst, message, timestamp }
      // DM: grava e publica no tópico do dst (para o SUB do usuário)
      // =========================================================
      case "message": {
        const src = String(data?.src || data?.from || "").trim();
        const dst = String(data?.dst || data?.to   || "").trim();
        const message = String(data?.message || "");
        if (!src || !dst) throw new Error("src e dst obrigatórios");

        const rec = {
          src,
          dst,
          message,
          timestamp: Number(data?.timestamp) || Date.now(),
          type: "dm",
        };
        await appendNdjson(DMS, rec);

        // Envia DM para o tópico do destinatário (e opcionalmente para o remetente)
        publishTopic(dst, rec);
        publishTopic(src, rec); // assim quem enviou também vê no SUB próprio

        reply.data.message = "DM registrada";
        reply.data.src = src;
        reply.data.dst = dst;
        break;
      }

      default:
        reply = {
          service: "error",
          data: { status: "erro", message: `serviço desconhecido: ${service}` },
        };
    }
  } catch (err) {
    console.error("[server] erro ao processar serviço", service, err?.message);
    reply = {
      service: "error",
      data: { status: "erro", message: err.message || String(err) },
    };
  }

  await rep.send(pack(reply)); // resposta em MessagePack
}
