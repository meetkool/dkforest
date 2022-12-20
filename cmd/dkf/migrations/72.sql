-- +migrate Up
CREATE TABLE IF NOT EXISTS pm_whitelisted_users (
    user_id INTEGER NOT NULL,
    whitelisted_user_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, whitelisted_user_id)
    CONSTRAINT pm_whitelisted_users_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT pm_whitelisted_users_whitelisted_user_id_fk
        FOREIGN KEY (whitelisted_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

ALTER TABLE users ADD COLUMN pm_mode INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
