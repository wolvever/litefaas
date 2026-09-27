const http = require("http");

const port = Number(process.env.PORT || "8080");
const name = "{{name}}";

const server = http.createServer((req, res) => {
  const url = req.url.split("?", 1)[0];
  if (req.method === "GET" && url === "/healthz") {
    res.writeHead(200, { "Content-Type": "text/plain; charset=utf-8" });
    res.end("ok");
    return;
  }

  const out = JSON.stringify({ message: "hello from litefaas", function: name }) + "\n";
  res.writeHead(200, { "Content-Type": "application/json" });
  res.end(out);
});

server.listen(port, "0.0.0.0", () => {
  console.log(name + " listening on 0.0.0.0:" + port);
});
