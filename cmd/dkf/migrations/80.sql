-- +migrate Up
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    message TEXT NOT NULL,
    is_read TINYINT(1) NOT NULL DEFAULT 0,
    read_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT notifications_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX notifications_user_id_idx ON notifications (user_id);
CREATE INDEX notifications_is_read_idx ON notifications (is_read);
CREATE INDEX notifications_read_at_idx ON notifications (read_at);

-- +migrate Down
