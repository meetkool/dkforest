-- +migrate Up
DROP TRIGGER links_after_insert;
DROP TRIGGER links_before_update;
DROP TRIGGER links_before_update_soft_delete;
DROP INDEX links_shorthand_uniq;

create table links_tmp (
    id          INTEGER                            not null primary key,
    uuid        VARCHAR(100)                       not null unique,
    url         VARCHAR(255)                       not null unique,
    title       VARCHAR(255)                       not null,
    description TEXT                               not null,
    signed_certificate TEXT,
    owner_user_id INTEGER NULL,
    visited_at  DATETIME,
    created_at  DATETIME default CURRENT_TIMESTAMP not null,
    deleted_at  DATETIME,
    updated_at  DATETIME default CURRENT_TIMESTAMP,
    shorthand   VARCHAR(50) UNIQUE,
    CONSTRAINT links_owner_user_id_fk
        FOREIGN KEY (owner_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

INSERT INTO links_tmp (id, uuid, url, title, description, visited_at, created_at, deleted_at, updated_at, shorthand)
SELECT id, uuid, url, title, description, visited_at, created_at, deleted_at, updated_at, shorthand FROM links;

DROP TABLE links;

ALTER TABLE links_tmp RENAME TO links;

CREATE INDEX links_owner_user_id_idx ON links(owner_user_id);

-- +migrate StatementBegin
CREATE TRIGGER links_after_insert
    AFTER INSERT ON links BEGIN
    INSERT INTO fts5_links(rowid, uuid, url, title, description, created_at, visited_at) VALUES
        (new.id, new.uuid, new.url, new.title, new.description, new.created_at, new.visited_at);
END;

CREATE TRIGGER links_after_update
    AFTER UPDATE ON links WHEN old.deleted_at IS NULL AND new.deleted_at IS NULL BEGIN
    INSERT INTO fts5_links(fts5_links, rowid, uuid, url, title, description, created_at, visited_at) VALUES
        ('delete', old.id, old.uuid, old.url, old.title, old.description, old.created_at, old.visited_at);
    INSERT INTO fts5_links(rowid, uuid, url, title, description, created_at, visited_at) VALUES
        (new.id, new.uuid, new.url, new.title, new.description, new.created_at, new.visited_at);
END;

CREATE TRIGGER links_after_update_restore
    AFTER UPDATE ON links WHEN old.deleted_at IS NOT NULL AND new.deleted_at IS NULL BEGIN
    INSERT INTO fts5_links(fts5_links, rowid, uuid, url, title, description, created_at, visited_at) VALUES
        ('delete', old.id, old.uuid, old.url, old.title, old.description, old.created_at, old.visited_at);
    INSERT INTO fts5_links(rowid, uuid, url, title, description, created_at, visited_at) VALUES
        (new.id, new.uuid, new.url, new.title, new.description, new.created_at, new.visited_at);
END;

CREATE TRIGGER links_after_update_soft_delete
    AFTER UPDATE ON links WHEN old.deleted_at IS NULL AND new.deleted_at IS NOT NULL BEGIN
    INSERT INTO fts5_links(fts5_links, rowid, uuid, url, title, description, created_at, visited_at) VALUES
        ('delete', old.id, old.uuid, old.url, old.title, old.description, old.created_at, old.visited_at);
END;

CREATE TRIGGER links_after_delete
    AFTER DELETE ON links BEGIN
    INSERT INTO fts5_links(fts5_links, rowid, uuid, url, title, description, created_at, visited_at) VALUES
        ('delete', old.id, old.uuid, old.url, old.title, old.description, old.created_at, old.visited_at);
END;

-- +migrate StatementEnd

-- +migrate Down
