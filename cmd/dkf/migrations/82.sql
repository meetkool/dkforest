-- +migrate Up
CREATE TABLE IF NOT EXISTS security_logs (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    message TEXT NOT NULL,
    typ INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT security_logs_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX security_logs_user_id_idx ON security_logs (user_id);
CREATE INDEX security_logs_typ_idx ON security_logs (typ);

-- +migrate Down
