-- +migrate Up
CREATE TABLE IF NOT EXISTS captcha_requests (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    captcha_img TEXT NOT NULL,
    answer VARCHAR(50) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT captcha_requests_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);
CREATE INDEX captcha_requests_created_at_idx ON captcha_requests (created_at);

-- +migrate Down
