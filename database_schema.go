package sriracha

var dbSchema = []string{`
CREATE TABLE account (
	id smallserial PRIMARY KEY,
	username varchar(255) NOT NULL,
	password text NOT NULL,
	role integer NOT NULL,
	lastactive bigint NOT NULL,
	session varchar(64) NOT NULL
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
);
CREATE INDEX ON post (board);
CREATE INDEX ON post (parent);
CREATE INDEX ON post (moderated);
CREATE INDEX ON post (stickied);
CREATE INDEX ON post (bumped);
CREATE UNIQUE INDEX ON post (filehash);

CREATE TABLE report (
	id serial PRIMARY KEY,
	board smallint NOT NULL REFERENCES board (id) ON DELETE CASCADE,
	post integer NOT NULL REFERENCES post (id) ON DELETE CASCADE,
	timestamp bigint NOT NULL,
	ip varchar(64) NOT NULL
);
CREATE UNIQUE INDEX ON report (board, post, ip);
`}
