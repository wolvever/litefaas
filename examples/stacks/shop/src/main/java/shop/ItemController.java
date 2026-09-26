package shop;

import java.util.List;
import java.util.Map;

import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class ItemController {
    private final ItemMapper items;

    public ItemController(ItemMapper items) {
        this.items = items;
    }

    @GetMapping(value = "/healthz", produces = MediaType.TEXT_PLAIN_VALUE)
    public String healthz() {
        return "ok";
    }

    @GetMapping("/")
    public Map<String, Object> root() {
        List<Item> all = items.findAll();
        return Map.of("service", "shop", "items", all);
    }
}
