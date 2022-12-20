-- +migrate Up
CREATE TABLE IF NOT EXISTS user_room_subscriptions (
    user_id INTEGER NOT NULL,
    room_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, room_id),
    CONSTRAINT user_room_subscriptions_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT user_room_subscriptions_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
