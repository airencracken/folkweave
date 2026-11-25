CREATE TABLE reader_progress(
 user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 caught_up_post_id INTEGER NOT NULL DEFAULT 0 CHECK(caught_up_post_id>=0),
 seen_reply_id INTEGER NOT NULL DEFAULT 0 CHECK(seen_reply_id>=0)
);
CREATE TABLE discussion_follows(
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
 after_reply_id INTEGER NOT NULL CHECK(after_reply_id>=0),
 PRIMARY KEY(user_id,post_id)
);
CREATE INDEX discussion_follows_post ON discussion_follows(post_id);
DROP TABLE schema_version;
CREATE TABLE schema_version(version INTEGER PRIMARY KEY CHECK(version=4));
INSERT INTO schema_version VALUES(4);
