ALTER TABLE posts ADD COLUMN repost_kind TEXT CHECK(repost_kind IN ('post','reply'));
ALTER TABLE posts ADD COLUMN repost_post_id INTEGER REFERENCES posts(id) ON DELETE SET NULL
 CHECK(repost_post_id IS NULL OR (repost_kind IS 'post' AND repost_post_id<>id));
ALTER TABLE posts ADD COLUMN repost_reply_id INTEGER REFERENCES replies(id) ON DELETE SET NULL
 CHECK(repost_reply_id IS NULL OR repost_kind IS 'reply');
CREATE UNIQUE INDEX reposts_source_post ON posts(repost_post_id,user_id) WHERE repost_post_id IS NOT NULL;
CREATE UNIQUE INDEX reposts_source_reply ON posts(repost_reply_id,user_id) WHERE repost_reply_id IS NOT NULL;

ALTER TABLE photo_previews RENAME TO photo_previews_v1;
ALTER TABLE photos RENAME TO photos_v1;
CREATE TABLE photos(
 id TEXT PRIMARY KEY, post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
 position INTEGER NOT NULL CHECK(position BETWEEN 0 AND 9), alt TEXT NOT NULL,
 data BLOB NOT NULL, width INTEGER NOT NULL CHECK(width>0), height INTEGER NOT NULL CHECK(height>0),
 UNIQUE(post_id,position)
);
CREATE TABLE photo_previews(
 photo_id TEXT NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
 width INTEGER NOT NULL CHECK(width IN (320,640,960)), data BLOB NOT NULL,
 PRIMARY KEY(photo_id,width)
);
INSERT INTO photos SELECT * FROM photos_v1;
INSERT INTO photo_previews SELECT * FROM photo_previews_v1;
DROP TABLE photo_previews_v1;
DROP TABLE photos_v1;

DROP TABLE schema_version;
CREATE TABLE schema_version(version INTEGER PRIMARY KEY CHECK(version=2));
INSERT INTO schema_version VALUES(2);
