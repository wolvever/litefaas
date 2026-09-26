package hello;

import java.nio.charset.StandardCharsets;
import java.util.Map;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
@RestController
public class Application {
    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }

    @GetMapping("/healthz")
    public String healthz() {
        return "ok";
    }

    @GetMapping("/")
    public Map<String, Object> get() {
        return Map.of("ok", true, "runtime", "java", "preset", "spring-boot");
    }

    @PostMapping(value = "/", consumes = MediaType.ALL_VALUE)
    public ResponseEntity<byte[]> post(@RequestBody(required = false) byte[] body) {
        if (body == null || body.length == 0) {
            body = "{\"ok\":true,\"runtime\":\"java\",\"preset\":\"spring-boot\"}"
                    .getBytes(StandardCharsets.UTF_8);
        }
        return ResponseEntity.ok().contentType(MediaType.APPLICATION_JSON).body(body);
    }
}
