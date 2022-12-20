-- +migrate Up
CREATE TABLE IF NOT EXISTS badges (
    id INTEGER PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);

INSERT INTO badges (name) VALUES ('RE challenge 1');
INSERT INTO badges (name) VALUES ('ByteRoad captcha challenge');

CREATE TABLE IF NOT EXISTS user_badges (
    user_id INTEGER NOT NULL,
    badge_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, badge_id),
    CONSTRAINT users_badges_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT users_badges_badge_id_fk
        FOREIGN KEY (badge_id)
            REFERENCES badges (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
