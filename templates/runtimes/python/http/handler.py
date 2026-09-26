import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("%s - %s" % (self.address_string(), fmt % args))

    def _send(self, status, body, content_type="application/json"):
        data = body if isinstance(body, bytes) else body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path.split("?", 1)[0] == "/healthz":
            self._send(200, b"ok", "text/plain")
            return
        self._send(200, b'{"ok":true,"runtime":"python"}')

    def do_POST(self):
        n = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(n) if n else b'{"ok":true,"runtime":"python"}'
        if not body:
            body = b'{"ok":true,"runtime":"python"}'
        self._send(200, body)


if __name__ == "__main__":
    port = int(os.environ.get("PORT") or "8080")
    print("python function listening on 0.0.0.0:%s" % port, flush=True)
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()
