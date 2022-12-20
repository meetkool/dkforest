-- +migrate Up
CREATE TABLE IF NOT EXISTS forum_threads_tmp (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    user_id INTEGER NOT NULL,
    is_club TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT forum_threads_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

INSERT INTO forum_threads_tmp (id, uuid, name, user_id, is_club, created_at)
SELECT id, lower(
            hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || '4' ||
            substr(hex( randomblob(2)), 2) || '-' ||
            substr('AB89', 1 + (abs(random()) % 4) , 1)  ||
            substr(hex(randomblob(2)), 2) || '-' ||
            hex(randomblob(6))
    ), name, user_id, 1, created_at FROM forum_threads;

DROP TABLE forum_threads;

ALTER TABLE forum_threads_tmp RENAME TO forum_threads;

CREATE INDEX forum_threads_user_id_idx ON forum_threads (user_id);
CREATE INDEX forum_threads_is_club_idx ON forum_threads (is_club);
-- +migrate Down
