-- +migrate Up
CREATE TABLE chat_room_whitelisted_users (
    room_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (room_id, user_id),
    CONSTRAINT chat_room_whitelisted_users_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_room_whitelisted_users_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

ALTER TABLE chat_rooms ADD COLUMN mode INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
