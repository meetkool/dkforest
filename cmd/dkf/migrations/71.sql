-- +migrate Up
CREATE TABLE IF NOT EXISTS links (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) UNIQUE NOT NULL,
    url VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    visited_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL,
    updated_at DATETIME NULL DEFAULT CURRENT_TIMESTAMP);

CREATE TABLE IF NOT EXISTS links_categories(
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE);

CREATE TABLE IF NOT EXISTS links_categories_links(
    link_id INTEGER NOT NULL,
    category_id INTEGER NOT NULL,
    PRIMARY KEY (link_id, category_id),
    CONSTRAINT links_categories_links_link_id_fk
        FOREIGN KEY (link_id)
            REFERENCES links (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT links_categories_links_category_id_fk
        FOREIGN KEY (category_id)
            REFERENCES links_categories (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

CREATE TABLE IF NOT EXISTS links_tags(
    id INTEGER NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE);

CREATE TABLE IF NOT EXISTS links_tags_links(
    link_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (link_id, tag_id),
    CONSTRAINT links_tags_links_link_id_fk
        FOREIGN KEY (link_id)
            REFERENCES links (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT links_tags_links_tag_id_fk
        FOREIGN KEY (tag_id)
            REFERENCES links_tags (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);


CREATE VIRTUAL TABLE fts5_links USING fts5(id UNINDEXED, uuid UNINDEXED, url UNINDEXED, title, description, created_at UNINDEXED, visited_at UNINDEXED, content='links', content_rowid='id');

-- +migrate StatementBegin
CREATE TRIGGER links_before_update
    BEFORE UPDATE ON links BEGIN
    DELETE FROM fts5_links WHERE id=old.id;
END;

CREATE TRIGGER links_before_delete
    BEFORE DELETE ON links BEGIN
    DELETE FROM fts5_links WHERE id=old.id;
END;

CREATE TRIGGER links_after_update
    AFTER UPDATE ON links BEGIN
    INSERT INTO fts5_links(id, uuid, url, title, description, created_at, visited_at)
    SELECT id, uuid, url, title, description, created_at, visited_at
    FROM links
    WHERE new.id = links.id;
END;

CREATE TRIGGER links_after_insert
    AFTER INSERT ON links BEGIN
    INSERT INTO fts5_links(id, uuid, url, title, description, created_at, visited_at)
    SELECT id, uuid, url, title, description, created_at, visited_at
    FROM links
    WHERE new.id = links.id;
END;
-- +migrate StatementEnd

INSERT INTO fts5_links SELECT id, uuid, url, title, description, created_at, visited_at FROM links;
INSERT INTO fts5_links(fts5_links) VALUES('rebuild');

-- +migrate Down
