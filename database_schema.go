package sriracha

var dbSchema = []string{`
CREATE TABLE account (
	id smallserial UNIQUE,
	username varchar(255) NOT NULL,
	password text NOT NULL,
	role integer NOT NULL,
	lastactive integer NOT NULL,
	session varchar(64) NOT NULL
);
CREATE UNIQUE INDEX ON account (username);
CREATE UNIQUE INDEX ON account (session);

CREATE TABLE ban (
	id bigserial UNIQUE,
	ip varchar(255) NOT NULL,
	timestamp integer NOT NULL,
	expire integer NOT NULL,
	reason text NOT NULL
);
CREATE UNIQUE INDEX ON ban (ip);

CREATE TABLE board (
	id smallserial UNIQUE,
	dir varchar(255) NOT NULL,
	name varchar(255) NOT NULL,
	description text NOT NULL,
	type smallint NOT NULL,
	approval smallint NOT NULL,
	maxsize integer NOT NULL,
	thumbwidth smallint NOT NULL,
	thumbheight smallint NOT NULL
);
CREATE UNIQUE INDEX ON board (dir);

CREATE TABLE config (
	name  text NOT NULL,
	value text NOT NULL,
	PRIMARY KEY	(name)
);
INSERT INTO config VALUES ('version', 1);

CREATE TABLE keyword (
	id smallserial UNIQUE,
	text varchar(255) NOT NULL,
	action varchar(255) NOT NULL
);
CREATE UNIQUE INDEX ON keyword (text);

CREATE TABLE keyword_board (
	keyword smallint NOT NULL REFERENCES keyword (id),
	board smallint NOT NULL REFERENCES board (id),
	PRIMARY KEY	(keyword, board)
);

CREATE TABLE log (
	id bigserial UNIQUE,
	board smallint NULL REFERENCES board (id),
	timestamp integer NOT NULL,
	account smallint NULL REFERENCES account (id),
	message text NOT NULL
);

CREATE TABLE post (
	board smallint NOT NULL REFERENCES board (id),
	id bigserial UNIQUE,
	parent integer NOT NULL,
	timestamp integer NOT NULL,
	bumped integer NOT NULL,
	ip varchar(255) NOT NULL,
	name varchar(75) NOT NULL,
	tripcode varchar(24) NOT NULL,
	email varchar(75) NOT NULL,
	nameblock varchar(255) NOT NULL,
	subject varchar(75) NOT NULL,
	message text NOT NULL,
	password varchar(255) NOT NULL,
	file text NOT NULL,
	file_hex varchar(75) NOT NULL,
	file_original varchar(255) NOT NULL,
	file_size integer NOT NULL default '0',
	file_size_formatted varchar(75) NOT NULL,
	image_width smallint NOT NULL default '0',
	image_height smallint NOT NULL default '0',
	thumb varchar(255) NOT NULL,
	thumb_width smallint NOT NULL default '0',
	thumb_height smallint NOT NULL default '0',
	moderated smallint NOT NULL default '1',
	stickied smallint NOT NULL default '0',
	locked smallint NOT NULL default '0',
	PRIMARY KEY	(board, id)
);
CREATE INDEX ON post (parent);
CREATE INDEX ON post (bumped);
CREATE INDEX ON post (stickied);
CREATE INDEX ON post (moderated);

CREATE TABLE report (
	id bigserial UNIQUE,
	board smallint NOT NULL REFERENCES board (id),
	ip varchar(255) NOT NULL,
	post integer NOT NULL
);`,
}
