-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id INTEGER NOT NULL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    two_factor_secret BLOB NULL,
    two_factor_recovery BLOB NULL,
    gpg_public_key TEXT NULL,
    token VARCHAR(255) unique,
    role VARCHAR(30) default 'member' not null,
    lang VARCHAR(10) default '' not null,
    chat_color VARCHAR(20) default '#000000' not null,
    api_key VARCHAR(50) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL,
    updated_at DATETIME NULL DEFAULT CURRENT_TIMESTAMP,
    is_admin TINYINT(1) NOT NULL DEFAULT 0,
    is_hellbanned TINYINT(1) NOT NULL DEFAULT 0,
    verified TINYINT(1) NOT NULL DEFAULT 0);

CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER NOT NULL PRIMARY KEY,
    token VARCHAR(255) NOT NULL,
    expires_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id INTEGER NOT NULL,
    client_ip VARCHAR(45) NULL,
    user_agent VARCHAR(255) NULL,
    CONSTRAINT sessions_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_token_idx ON sessions (token);

CREATE TABLE IF NOT EXISTS invitations (
    id INTEGER NOT NULL PRIMARY KEY,
    token VARCHAR(255) NOT NULL,
    owner_user_id INTEGER NOT NULL,
    invitee_user_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT invitations_owner_user_id_fk
        FOREIGN KEY (owner_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT invitations_invitee_user_id_fk
        FOREIGN KEY (invitee_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX invitations_owner_user_id_idx ON invitations (owner_user_id);
CREATE INDEX invitations_invitee_user_id_idx ON invitations (invitee_user_id);
CREATE INDEX invitations_token_idx ON invitations (token);

CREATE TABLE IF NOT EXISTS chat_rooms (
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);

INSERT INTO chat_rooms (id, name) VALUES (1, 'general');
INSERT INTO chat_rooms (id, name) VALUES (2, 'suggestions');
INSERT INTO chat_rooms (id, name) VALUES (3, 'announcements');
INSERT INTO chat_rooms (id, name) VALUES (4, 'moderators');
INSERT INTO chat_rooms (id, name) VALUES (5, 'werewolf');

CREATE TABLE IF NOT EXISTS chat_messages (
    id INTEGER NOT NULL PRIMARY KEY,
    message VARCHAR(255) NOT NULL,
    room_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chat_messages_room_id_fk
        FOREIGN KEY (room_id)
            REFERENCES chat_rooms (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chat_messages_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX chat_messages_room_id_idx ON chat_messages (room_id);

-- +migrate Down
