// Fingerprint stub for node-nestjs-prisma (Nest deps in package.json; trivial HTTP for $PORT).
const http = require("http");
const PORT = process.env.PORT || "8080";
const SEED = [{ title: "zero-config nest tasks" }];

const server = http.createServer((req, res) => {
  const url = req.url.split("?", 1)[0];
  if (req.method === "GET" && url === "/healthz") {
    res.writeHead(200, { "Content-Type": "text/plain; charset=utf-8" });
    res.end("ok");
    return;
  }
  if (req.method === "GET" && (url === "/" || url === "")) {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ service: "tasks", store: "memory", items: SEED }));
    return;
  }
  res.writeHead(404);
  res.end();
});

server.listen(Number(PORT), "0.0.0.0", () => {
  console.log("tasks listening on 0.0.0.0:" + PORT);
});
