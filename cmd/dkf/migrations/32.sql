-- +migrate Up
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    log VARCHAR(255) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT audit_logs_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX audit_logs_user_id_idx ON audit_logs (user_id);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at);

-- +migrate Down
