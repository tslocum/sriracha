package database

var dbSchema = []string{ // Version 1.
	`CREATE TABLE account (
	id smallserial PRIMARY KEY,
	username varchar(255) NOT NULL,
	password text NOT NULL,
	role integer NOT NULL,
	lastactive bigint NOT NULL,
	session varchar(64) NOT NULL
	-- v2: style varchar(64) NOT NULL DEFAULT ''
	-- v6: locale varchar(64) NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX ON account (username);
CREATE UNIQUE INDEX ON account (session);

CREATE TABLE ban (
	id serial PRIMARY KEY,
	ip varchar(64) NOT NULL,
	timestamp bigint NOT NULL,
	expire bigint NOT NULL,
	reason text NOT NULL
);
CREATE UNIQUE INDEX ON ban (ip);

-- v9: CREATE TABLE banfile (
-- v9: 	hash char(64) PRIMARY KEY
-- v9: );

-- v13: CREATE TABLE banner (
-- v13: 	id serial PRIMARY KEY,
-- v13: 	name text NOT NULL,
-- v13: 	width smallint NOT NULL,
-- v13: 	height smallint NOT NULL,
-- v13: 	overboard smallint NOT NULL,
-- v13: 	news smallint NOT NULL,
-- v13: 	pages smallint NOT NULL
-- v13: );
-- v13: CREATE UNIQUE INDEX ON banner (name);

-- v13: CREATE TABLE banner_board (
-- v13: 	banner smallint NOT NULL REFERENCES banner (id) ON DELETE CASCADE,
-- v13: 	board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
-- v13: 	PRIMARY KEY	(banner, board)
-- v13: );

CREATE TABLE board (
	id smallserial PRIMARY KEY,
	dir varchar(255) NOT NULL,
	name varchar(255) NOT NULL,
	description text NOT NULL,
	type smallint NOT NULL,
	lock smallint NOT NULL,
	approval smallint NOT NULL,
	reports smallint NOT NULL,
	style varchar(64) NOT NULL,
	locale varchar(64) NOT NULL,
	delay integer NOT NULL,
	minname smallint NOT NULL,
	maxname smallint NOT NULL,
	minemail smallint NOT NULL,
	maxemail smallint NOT NULL,
	minsubject smallint NOT NULL,
	maxsubject smallint NOT NULL,
	minmessage smallint NOT NULL,
	maxmessage smallint NOT NULL,
	minsizethread bigint NOT NULL,
	maxsizethread bigint NOT NULL,
	minsizereply bigint NOT NULL,
	maxsizereply bigint NOT NULL,
	thumbwidth smallint NOT NULL,
	thumbheight smallint NOT NULL,
	defaultname varchar(255) NOT NULL,
	wordbreak smallint NOT NULL,
	truncate smallint NOT NULL,
	threads smallint NOT NULL,
	replies smallint NOT NULL,
	maxthreads smallint NOT NULL,
	maxreplies smallint NOT NULL
	-- v3: oekaki smallint NOT NULL DEFAULT 0
	-- v4: rules text NOT NULL DEFAULT ''
	-- v7: hide smallint NOT NULL DEFAULT 0
	-- v8: backlinks smallint NOT NULL DEFAULT 0
	-- v8: instances smallint NOT NULL DEFAULT 1
	-- v9: identifiers smallint NOT NULL DEFAULT 0
	-- v10: files smallint NOT NULL DEFAULT 1
	-- v11: gallery smallint NOT NULL DEFAULT 1
	-- v17: require smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX ON board (dir);

CREATE TABLE board_embed (
	board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
	embed varchar(64) NOT NULL,
	PRIMARY KEY	(board, embed)
);

CREATE TABLE board_upload (
	board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
	upload varchar(64) NOT NULL,
	PRIMARY KEY	(board, upload)
);

CREATE TABLE captcha (
	ip varchar(64) PRIMARY KEY,
	timestamp bigint NOT NULL,
	refresh smallint NOT NULL,
	image varchar(64) NOT NULL,
	text varchar(5) NOT NULL
);

-- v15: CREATE TABLE category (
-- v15: 	id smallserial PRIMARY KEY,
-- v15: 	parent smallint REFERENCES category (id) ON DELETE CASCADE,
-- v15: 	sort smallint NOT NULL,
-- v15: 	name varchar(255) NOT NULL,
-- v15: 	description varchar(255) NOT NULL
-- v15: );

-- v15: CREATE TABLE category_board (
-- v15: 	category smallint NOT NULL REFERENCES category (id) ON DELETE CASCADE,
-- v15: 	board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
-- v15: 	sort smallint NOT NULL,
-- v15: 	PRIMARY KEY (category, board)
-- v15: );

CREATE TABLE config (
	name  text PRIMARY KEY,
	value text NOT NULL
);
INSERT INTO config VALUES ('version', 1);

CREATE TABLE keyword (
	id smallserial PRIMARY KEY,
	text varchar(255) NOT NULL,
	action varchar(255) NOT NULL
);
CREATE UNIQUE INDEX ON keyword (text);

CREATE TABLE keyword_board (
	keyword smallint NOT NULL REFERENCES keyword (id) ON DELETE CASCADE,
	board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
	PRIMARY KEY	(keyword, board)
);

CREATE TABLE log (
	id serial PRIMARY KEY,
	account smallint NULL REFERENCES account (id) ON DELETE SET NULL,
	board smallint NULL REFERENCES board (id) ON DELETE SET NULL,
	timestamp bigint NOT NULL,
	message text NOT NULL,
	changes text NOT NULL
);

-- v4: CREATE TABLE news (
-- v4: 	id serial PRIMARY KEY,
-- v4: 	account smallint NULL REFERENCES account (id) ON DELETE SET NULL,
-- v4: 	timestamp bigint NOT NULL,
-- v4: 	modified bigint NOT NULL,
-- v4: 	share smallint NOT NULL,
-- v4: 	name varchar(255) NOT NULL,
-- v4: 	subject varchar(255) NOT NULL,
-- v4: 	message text NOT NULL
-- v4: );

-- v12: CREATE TABLE page (
-- v12: 	id serial PRIMARY KEY,
-- v12: 	path text NOT NULL,
-- v12: 	content text NOT NULL
-- v12: );
-- v13: CREATE UNIQUE INDEX ON page (path);

CREATE TABLE post (
	id serial PRIMARY KEY,
	parent integer REFERENCES post (id) ON DELETE CASCADE,
	board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
	timestamp bigint NOT NULL,
	bumped bigint NOT NULL,
	ip varchar(64) NOT NULL,
	name varchar(75) NOT NULL,
	tripcode varchar(24) NOT NULL,
	email varchar(75) NOT NULL,
	nameblock varchar(255) NOT NULL,
	subject varchar(75) NOT NULL,
	message text NOT NULL,
	password varchar(255) NOT NULL,
	file text NOT NULL,
	filehash text NULL,
	fileoriginal varchar(255) NOT NULL,
	filesize integer NOT NULL default '0',
	filewidth smallint NOT NULL default '0',
	fileheight smallint NOT NULL default '0',
	thumb varchar(255) NOT NULL,
	thumbwidth smallint NOT NULL default '0',
	thumbheight smallint NOT NULL default '0',
	moderated smallint NOT NULL default '1',
	stickied smallint NOT NULL default '0',
	locked smallint NOT NULL default '0'
	-- v5: filemime varchar(64) NOT NULL default ''
	-- v16: backlinks integer[] NOT NULL DEFAULT array[]::integer[];
);
CREATE INDEX ON post (board);
CREATE INDEX ON post (parent);
CREATE INDEX ON post (moderated);
CREATE INDEX ON post (stickied);
CREATE INDEX ON post (bumped);
CREATE UNIQUE INDEX ON post (filehash);
-- v8: CREATE INDEX ON post (filehash);

CREATE TABLE report (
	id serial PRIMARY KEY,
	board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
	post integer NOT NULL REFERENCES post (id) ON DELETE CASCADE,
	timestamp bigint NOT NULL,
	ip varchar(64) NOT NULL
);
CREATE UNIQUE INDEX ON report (board, post, ip);

-- v14: CREATE TABLE subscription (
-- v14: 	id serial PRIMARY KEY,
-- v14: 	ip text NOT NULL,
-- v14: 	confirm bigint NOT NULL,
-- v14: 	email text NOT NULL,
-- v14: 	board int NOT NULL,
-- v14: 	target int NOT NULL
-- v14: );`,
	// Version 2.
	`ALTER TABLE account ADD COLUMN style varchar(64) NOT NULL DEFAULT '';
	UPDATE config SET value = '2' WHERE name = 'version';`,
	// Version 3.
	`ALTER TABLE board ADD COLUMN oekaki smallint NOT NULL DEFAULT 0;
	UPDATE config SET value = '3' WHERE name = 'version';`,
	// Version 4.
	`ALTER TABLE board ADD COLUMN rules text NOT NULL DEFAULT '';
	CREATE TABLE news (
		id serial PRIMARY KEY,
		account smallint NULL REFERENCES account (id) ON DELETE SET NULL,
		timestamp bigint NOT NULL,
		modified bigint NOT NULL,
		share smallint NOT NULL,
		name varchar(255) NOT NULL,
		subject varchar(255) NOT NULL,
		message text NOT NULL
	);
	UPDATE config SET value = '4' WHERE name = 'version';`,
	// Version 5.
	`ALTER TABLE post ADD COLUMN filemime varchar(64) NOT NULL default '';
	UPDATE config SET value = '5' WHERE name = 'version';`,
	// Version 6.
	`ALTER TABLE account ADD COLUMN locale varchar(64) NOT NULL default '';
	UPDATE config SET value = '6' WHERE name = 'version';`,
	// Version 7.
	`ALTER TABLE board ADD COLUMN hide smallint NOT NULL DEFAULT 0;
	UPDATE config SET value = '7' WHERE name = 'version';`,
	// Version 8.
	`ALTER TABLE board ADD COLUMN backlinks smallint NOT NULL DEFAULT 0;
	ALTER TABLE board ADD COLUMN instances smallint NOT NULL DEFAULT 1;
	DROP INDEX post_filehash_idx;
	CREATE INDEX ON post (filehash);
	UPDATE config SET value = '8' WHERE name = 'version';`,
	// Version 9.
	`ALTER TABLE board ADD COLUMN identifiers smallint NOT NULL DEFAULT 0;
	CREATE TABLE banfile (
		hash char(64) PRIMARY KEY
	);
	UPDATE config SET value = '9' WHERE name = 'version';`,
	// Version 10.
	`ALTER TABLE board ADD COLUMN files smallint NOT NULL DEFAULT 1;
	UPDATE config SET value = '10' WHERE name = 'version';`,
	// Version 11.
	`ALTER TABLE board ADD COLUMN gallery smallint NOT NULL DEFAULT 1;
	UPDATE config SET value = '11' WHERE name = 'version';`,
	// Version 12.
	`CREATE TABLE page (
		id serial PRIMARY KEY,
		path text NOT NULL,
		content text NOT NULL
	);
	UPDATE config SET value = '12' WHERE name = 'version';`,
	// Version 13.
	`CREATE UNIQUE INDEX ON page (path);
	CREATE TABLE banner (
		id serial PRIMARY KEY,
		name text NOT NULL,
		width smallint NOT NULL,
		height smallint NOT NULL,
		overboard smallint NOT NULL,
		news smallint NOT NULL,
		pages smallint NOT NULL
	);
	CREATE UNIQUE INDEX ON banner (name);
	CREATE TABLE banner_board (
		banner smallint NOT NULL REFERENCES banner (id) ON DELETE CASCADE,
		board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
		PRIMARY KEY	(banner, board)
	);
	UPDATE config SET value = '13' WHERE name = 'version';`,
	// Version 14.
	`CREATE TABLE subscription (
		id serial PRIMARY KEY,
		ip text NOT NULL,
		confirm bigint NOT NULL,
		email text NOT NULL,
		board int NOT NULL,
		target int NOT NULL
	);
	UPDATE config SET value = '14' WHERE name = 'version';`,
	// Version 15.
	`CREATE TABLE category (
		id smallserial PRIMARY KEY,
		parent smallint REFERENCES category (id) ON DELETE CASCADE,
		sort smallint NOT NULL,
		name varchar(255) NOT NULL,
		description varchar(255) NOT NULL
	);
	CREATE TABLE category_board (
		category smallint NOT NULL REFERENCES category (id) ON DELETE CASCADE,
		board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
		sort smallint NOT NULL,
		PRIMARY KEY (category, board)
	);
	UPDATE config SET value = '15' WHERE name = 'version';`,
	// Version 16.
	`ALTER TABLE post ADD COLUMN backlinks integer[] NOT NULL DEFAULT array[]::integer[];
	UPDATE config SET value = '16' WHERE name = 'version';`,
	// Version 17.
	`ALTER TABLE board ADD COLUMN require smallint NOT NULL DEFAULT 0;
	UPDATE config SET value = '17' WHERE name = 'version';`,
}
