-- +migrate Up
CREATE TABLE IF NOT EXISTS products(
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT NOT NULL,
    price INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);

CREATE TABLE IF NOT EXISTS xmr_invoices(
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    product_id INTEGER NOT NULL,
    address VARCHAR(255) NOT NULL,
    amount_requested INTEGER NOT NULL,
    amount_received INTEGER NULL,
    confirmations INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT xmr_invoices_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT xmr_invoices_product_id_fk
        FOREIGN KEY (product_id)
            REFERENCES products (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX xmr_invoices_user_id_idx ON xmr_invoices (user_id);
CREATE INDEX xmr_invoices_address_idx ON xmr_invoices (address);

-- +migrate Down
