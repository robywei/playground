CREATE TABLE purchases (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  brand         TEXT NOT NULL DEFAULT '',
  purchase_date TEXT NOT NULL,
  channel       TEXT NOT NULL DEFAULT '',
  price         REAL NOT NULL CHECK (price >= 0),
  weight_g      REAL NOT NULL CHECK (weight_g > 0)
);
CREATE INDEX idx_purchases_name ON purchases(name);
CREATE INDEX idx_purchases_date ON purchases(purchase_date);

CREATE TABLE doughs (
  id   TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE dough_ingredients (
  dough_id TEXT NOT NULL REFERENCES doughs(id) ON DELETE CASCADE,
  name     TEXT NOT NULL,
  pct      REAL NOT NULL CHECK (pct > 0),
  sort     INTEGER NOT NULL,
  PRIMARY KEY (dough_id, sort)
);

CREATE TABLE fillings (
  id   TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE filling_ingredients (
  filling_id TEXT NOT NULL REFERENCES fillings(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  weight_g   REAL NOT NULL CHECK (weight_g > 0),
  sort       INTEGER NOT NULL,
  PRIMARY KEY (filling_id, sort)
);

CREATE TABLE products (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL UNIQUE,
  price           REAL NOT NULL CHECK (price >= 0),
  dough_id        TEXT NOT NULL REFERENCES doughs(id) ON DELETE RESTRICT,
  dough_weight_g  REAL NOT NULL CHECK (dough_weight_g > 0),
  fill1_id        TEXT REFERENCES fillings(id) ON DELETE RESTRICT,
  fill1_weight_g  REAL NOT NULL DEFAULT 0 CHECK (fill1_weight_g >= 0),
  fill2_id        TEXT REFERENCES fillings(id) ON DELETE RESTRICT,
  fill2_weight_g  REAL NOT NULL DEFAULT 0 CHECK (fill2_weight_g >= 0)
);

CREATE TABLE sales (
  id           TEXT PRIMARY KEY,
  sale_date    TEXT NOT NULL,
  product_id   TEXT REFERENCES products(id) ON DELETE SET NULL,
  product_name TEXT NOT NULL,
  qty          INTEGER NOT NULL CHECK (qty > 0),
  unit_cost    REAL NOT NULL,
  unit_price   REAL NOT NULL
);
CREATE INDEX idx_sales_date ON sales(sale_date);

CREATE TABLE production_logs (
  id           TEXT PRIMARY KEY,
  logged_date  TEXT NOT NULL,
  product_id   TEXT REFERENCES products(id) ON DELETE SET NULL,
  product_name TEXT NOT NULL,
  qty          INTEGER NOT NULL CHECK (qty > 0)
);
CREATE INDEX idx_production_date ON production_logs(logged_date);

CREATE TABLE production_consumption (
  log_id          TEXT NOT NULL REFERENCES production_logs(id) ON DELETE CASCADE,
  ingredient_name TEXT NOT NULL,
  consumed_g      REAL NOT NULL CHECK (consumed_g >= 0),
  PRIMARY KEY (log_id, ingredient_name)
);
