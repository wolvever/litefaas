const express = require("express");

const PORT = process.env.PORT || "8080";
const SEED = [
  { code: "t-1", title: "wire stack pack", status: "open" },
  { code: "t-2", title: "bind 0.0.0.0:$PORT", status: "open" },
];

const app = express();

let prisma = null;
if (process.env.DATABASE_URL) {
  try {
    const { PrismaClient } = require("@prisma/client");
    prisma = new PrismaClient();
  } catch (err) {
    console.warn("prisma client unavailable:", err.message);
  }
}

app.get("/healthz", (_req, res) => {
  res.type("text").send("ok");
});

app.get("/", async (_req, res) => {
  if (!prisma) {
    return res.json({ service: "tickets", store: "memory", items: SEED });
  }
  try {
    let rows = await prisma.ticket.findMany();
    if (rows.length === 0) {
      await prisma.ticket.createMany({ data: SEED });
      rows = await prisma.ticket.findMany();
    }
    return res.json({
      service: "tickets",
      store: "sql",
      items: rows.map((r) => ({ code: r.code, title: r.title, status: r.status })),
    });
  } catch (err) {
    console.warn("database unavailable:", err.message);
    return res.json({ service: "tickets", store: "memory", items: SEED });
  }
});

app.listen(Number(PORT), "0.0.0.0", () => {
  console.log(`tickets listening on 0.0.0.0:${PORT}`);
});
