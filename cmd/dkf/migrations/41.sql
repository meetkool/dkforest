-- +migrate Up
CREATE TABLE IF NOT EXISTS chat_read_records (
    user_id INTEGER NOT NULL,
    room_id INTEGER NOT NULL,
    read_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, room_id),
    CONSTRAINT chat_read_records_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_read_records_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

CREATE INDEX chat_rooms_name_idx ON chat_rooms (name);

-- +migrate Down
