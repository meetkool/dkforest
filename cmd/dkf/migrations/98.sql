-- +migrate Up
CREATE TABLE IF NOT EXISTS snippets (
    name VARCHAR(20) NOT NULL,
    user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    PRIMARY KEY (user_id, name),
    CONSTRAINT snippets_user_id_fk
    FOREIGN KEY (user_id)
        REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
