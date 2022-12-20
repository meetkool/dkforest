-- +migrate Up
CREATE TABLE IF NOT EXISTS pm_blacklisted_users (
    user_id INTEGER NOT NULL,
    blacklisted_user_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, blacklisted_user_id)
    CONSTRAINT pm_blacklisted_users_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT pm_blacklisted_users_blacklisted_user_id_fk
        FOREIGN KEY (blacklisted_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
