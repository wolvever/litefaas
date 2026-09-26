package hello;

import java.util.Map;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.http.MediaType;
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

    @GetMapping(value = "/healthz", produces = MediaType.TEXT_PLAIN_VALUE)
    public String healthz() {
        return "ok";
    }

    @GetMapping("/")
    public Map<String, String> get() {
        return Map.of("message", "hello from {{name}}", "function", "{{name}}");
    }

    @PostMapping(value = "/", consumes = MediaType.ALL_VALUE)
    public Map<String, String> post(@RequestBody(required = false) Map<String, Object> in) {
        String who = "{{name}}";
        if (in != null && in.get("name") != null) {
            String v = String.valueOf(in.get("name"));
            if (!v.isEmpty() && !"null".equals(v)) {
                who = v;
            }
        }
        return Map.of("message", "hello from " + who, "function", "{{name}}");
    }
}
