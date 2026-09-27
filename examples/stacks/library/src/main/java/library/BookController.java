package library;

import java.util.List;
import java.util.Map;

import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class BookController {
    private final BookRepository books;

    public BookController(BookRepository books) {
        this.books = books;
    }

    @GetMapping(value = "/healthz", produces = MediaType.TEXT_PLAIN_VALUE)
    public String healthz() {
        return "ok";
    }

    @GetMapping("/")
    public Map<String, Object> root() {
        List<Book> all = books.findAll();
        if (all.isEmpty()) {
            Book a = new Book();
            a.setIsbn("rfc-0001");
            a.setTitle("litefaas architecture");
            Book b = new Book();
            b.setIsbn("http-port");
            b.setTitle("bind 0.0.0.0:$PORT");
            books.saveAll(List.of(a, b));
            all = books.findAll();
        }
        return Map.of(
            "service", "library",
            "store", "jpa",
            "items", all.stream().map(book -> Map.of("isbn", book.getIsbn(), "title", book.getTitle())).toList()
        );
    }
}
