-- Initial schema for the QMart quick-commerce backend (PostgreSQL).

CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'customer',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE products (
    id          BIGSERIAL PRIMARY KEY,
    sku         TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL DEFAULT '',
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    currency    TEXT NOT NULL DEFAULT 'USD',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_products_category ON products (category);
CREATE INDEX idx_products_active ON products (is_active);
CREATE INDEX idx_products_name ON products (name);

CREATE TABLE inventory (
    product_id BIGINT NOT NULL REFERENCES products (id),
    store_id   TEXT NOT NULL,
    quantity   BIGINT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    reserved   BIGINT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (product_id, store_id)
);

CREATE TABLE carts (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL UNIQUE REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE cart_items (
    id         BIGSERIAL PRIMARY KEY,
    cart_id    BIGINT NOT NULL REFERENCES carts (id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products (id),
    quantity   BIGINT NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (cart_id, product_id)
);

CREATE INDEX idx_cart_items_cart ON cart_items (cart_id);

CREATE TABLE orders (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES users (id),
    store_id           TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'pending',
    subtotal_cents     BIGINT NOT NULL,
    delivery_fee_cents BIGINT NOT NULL DEFAULT 0,
    total_cents        BIGINT NOT NULL,
    currency           TEXT NOT NULL DEFAULT 'USD',
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_orders_user ON orders (user_id);
CREATE INDEX idx_orders_status ON orders (status);

CREATE TABLE order_items (
    id               BIGSERIAL PRIMARY KEY,
    order_id         BIGINT NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id       BIGINT NOT NULL REFERENCES products (id),
    name             TEXT NOT NULL,
    sku              TEXT NOT NULL,
    quantity         BIGINT NOT NULL,
    unit_price_cents BIGINT NOT NULL,
    total_cents      BIGINT NOT NULL
);

CREATE INDEX idx_order_items_order ON order_items (order_id);

CREATE TABLE payments (
    id           BIGSERIAL PRIMARY KEY,
    order_id     BIGINT NOT NULL UNIQUE REFERENCES orders (id),
    amount_cents BIGINT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    provider     TEXT NOT NULL DEFAULT 'mock',
    reference    TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_payments_order ON payments (order_id);
