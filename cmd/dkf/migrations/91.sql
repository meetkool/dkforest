-- +migrate Up

CREATE TABLE IF NOT EXISTS forum_categories (
    id INTEGER NOT NULL PRIMARY KEY,
    idx INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE);

CREATE TABLE IF NOT EXISTS forum_threads_tmp (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    user_id INTEGER NOT NULL,
    category_id INTEGER NOT NULL,
    is_club TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT forum_threads_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT forum_threads_category_id_fk
        FOREIGN KEY (category_id)
            REFERENCES forum_categories (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

INSERT INTO forum_categories (id, name, slug) VALUES (1, 'General', 'general');
INSERT INTO forum_categories (id, name, slug) VALUES (2, 'Random', 'random');

INSERT INTO forum_threads_tmp (id, uuid, name, user_id, category_id, is_club, created_at)
SELECT id, uuid, name, user_id, 1, is_club, created_at FROM forum_threads;

DROP TABLE forum_threads;

ALTER TABLE forum_threads_tmp RENAME TO forum_threads;

CREATE INDEX forum_threads_user_id_idx ON forum_threads (user_id);
CREATE INDEX forum_threads_category_id_idx ON forum_threads (category_id);
CREATE INDEX forum_threads_is_club_idx ON forum_threads (is_club);

-- +migrate Down
