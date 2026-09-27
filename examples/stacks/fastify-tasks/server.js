const Fastify = require("fastify");

const PORT = process.env.PORT || "8080";
const SEED = [
  { code: "f-1", title: "wire fastify pack", status: "open" },
  { code: "f-2", title: "bind 0.0.0.0:$PORT", status: "open" },
];

const app = Fastify({ logger: false });

let prisma = null;
if (process.env.DATABASE_URL) {
  try {
    const { PrismaClient } = require("@prisma/client");
    prisma = new PrismaClient();
  } catch (err) {
    console.warn("prisma client unavailable:", err.message);
  }
}

app.get("/healthz", async () => "ok");

app.get("/", async () => {
  if (!prisma) {
    return { service: "fastify-tasks", store: "memory", items: SEED };
  }
  try {
    let rows = await prisma.task.findMany();
    if (rows.length === 0) {
      await prisma.task.createMany({ data: SEED });
      rows = await prisma.task.findMany();
    }
    return {
      service: "fastify-tasks",
      store: "sql",
      items: rows.map((r) => ({ code: r.code, title: r.title, status: r.status })),
    };
  } catch (err) {
    console.warn("database unavailable:", err.message);
    return { service: "fastify-tasks", store: "memory", items: SEED };
  }
});

app.listen({ port: Number(PORT), host: "0.0.0.0" }, (err) => {
  if (err) {
    console.error(err);
    process.exit(1);
  }
  console.log(`fastify-tasks listening on 0.0.0.0:${PORT}`);
});
