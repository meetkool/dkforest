-- +migrate Up

CREATE TABLE IF NOT EXISTS sessions_tmp (
    token VARCHAR(255) PRIMARY KEY NOT NULL,
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

INSERT INTO sessions_tmp (token, expires_at, deleted_at, created_at, user_id, client_ip, user_agent)
SELECT token, expires_at, deleted_at, created_at, user_id, client_ip, user_agent FROM sessions;

DROP INDEX sessions_user_id_idx;
DROP INDEX sessions_token_idx;
DROP TABLE sessions;

ALTER TABLE sessions_tmp RENAME TO sessions;

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE IF NOT EXISTS session_notifications (
    id INTEGER NOT NULL PRIMARY KEY,
    session_token VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    is_read TINYINT(1) NOT NULL DEFAULT 0,
    read_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT session_notifications_session_token_fk
        FOREIGN KEY (session_token)
            REFERENCES sessions (token)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX session_notifications_session_token_idx ON session_notifications (session_token);
CREATE INDEX session_notifications_is_read_idx ON session_notifications (is_read);
CREATE INDEX session_notifications_read_at_idx ON session_notifications (read_at);

-- +migrate Down
