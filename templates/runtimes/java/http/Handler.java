import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;

public class Handler {
    public static void main(String[] args) throws IOException {
        int port = 8080;
        String env = System.getenv("PORT");
        if (env != null && !env.isEmpty()) {
            port = Integer.parseInt(env);
        }
        HttpServer server = HttpServer.create(new InetSocketAddress("0.0.0.0", port), 0);
        server.createContext("/healthz", ex -> write(ex, 200, "ok", "text/plain; charset=utf-8"));
        server.createContext("/", Handler::handle);
        System.out.println("{{name}} listening on 0.0.0.0:" + port);
        server.start();
    }

    private static void handle(HttpExchange ex) throws IOException {
        String body = new String(ex.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
        String who = "{{name}}";
        int key = body.indexOf("\"name\"");
        if (key >= 0) {
            int q1 = body.indexOf('"', key + 6);
            int q2 = body.indexOf('"', q1 + 1);
            if (q1 >= 0 && q2 > q1) {
                String v = body.substring(q1 + 1, q2);
                if (!v.isEmpty()) {
                    who = v;
                }
            }
        }
        String out = "{\"message\":\"hello from " + escape(who) + "\",\"function\":\"{{name}}\"}\n";
        write(ex, 200, out, "application/json");
    }

    private static String escape(String s) {
        return s.replace("\\", "\\\\").replace("\"", "\\\"");
    }

    private static void write(HttpExchange ex, int status, String body, String contentType) throws IOException {
        byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
        ex.getResponseHeaders().set("Content-Type", contentType);
        ex.sendResponseHeaders(status, bytes.length);
        try (OutputStream os = ex.getResponseBody(); InputStream ignore = ex.getRequestBody()) {
            os.write(bytes);
        }
    }
}
