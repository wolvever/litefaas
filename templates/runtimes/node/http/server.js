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

  let body = Buffer.alloc(0);
  req.on("data", (chunk) => {
    body = Buffer.concat([body, chunk]);
  });
  req.on("end", () => {
    let who = name;
    if (body.length) {
      try {
        const data = JSON.parse(body.toString("utf8"));
        if (data && data.name) who = String(data.name);
      } catch (_) {}
    }
    const out = JSON.stringify({ message: "hello from " + who, function: name }) + "\n";
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(out);
  });
});

server.listen(port, "0.0.0.0", () => {
  console.log(name + " listening on 0.0.0.0:" + port);
});
