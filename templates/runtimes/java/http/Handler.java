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
        server.createContext("/healthz", ex -> write(ex, 200, "ok"));
        server.createContext("/", ex -> {
            byte[] body = ex.getRequestBody().readAllBytes();
            if (body.length == 0) {
                body = "{\"ok\":true,\"runtime\":\"java\"}".getBytes(StandardCharsets.UTF_8);
            }
            ex.getResponseHeaders().set("Content-Type", "application/json");
            write(ex, 200, new String(body, StandardCharsets.UTF_8));
        });
        System.out.println("java function listening on 0.0.0.0:" + port);
        server.start();
    }

    private static void write(HttpExchange ex, int status, String body) throws IOException {
        byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
        ex.sendResponseHeaders(status, bytes.length);
        try (OutputStream os = ex.getResponseBody(); InputStream ignore = ex.getRequestBody()) {
            os.write(bytes);
        }
    }
}
