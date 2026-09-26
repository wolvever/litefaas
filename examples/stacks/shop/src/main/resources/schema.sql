CREATE TABLE IF NOT EXISTS items (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  sku VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  stock INT NOT NULL DEFAULT 0
);

INSERT INTO items (sku, name, stock) VALUES ('mug-1', 'Litefaas mug', 12);
INSERT INTO items (sku, name, stock) VALUES ('tee-1', 'Control-plane tee', 4);
