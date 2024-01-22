-- +migrate Up
INSERT INTO forum_categories (name, slug) VALUES ('News', 'news');

-- +migrate Down
