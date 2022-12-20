-- +migrate Up
CREATE TABLE IF NOT EXISTS links_pgps (
    id INTEGER NOT NULL PRIMARY KEY,
    idx INTEGER NOT NULL,
    link_id INTEGER NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    pgp_public_key TEXT NOT NULL,
    CONSTRAINT links_pgps_link_id_fk
        FOREIGN KEY (link_id)
            REFERENCES links (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

CREATE TABLE IF NOT EXISTS links_mirrors (
    id INTEGER NOT NULL PRIMARY KEY,
    link_id INTEGER NOT NULL,
    idx INTEGER NOT NULL,
    mirror_url VARCHAR(255) NOT NULL,
    CONSTRAINT links_pgps_link_id_fk
        FOREIGN KEY (link_id)
            REFERENCES links (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
