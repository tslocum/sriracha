package util

import (
	"strings"
	"time"
)

type UploadType struct {
	Ext   string
	MIME  string
	Thumb string
}

// ImportConfig represents a board import configuration.
type ImportConfig struct {
	Address  string // Address:Port to connect to the database.
	Username string // Database username.
	Password string // Database password.
	DBName   string // Database name.

	Posts    string // Posts table.
	Keywords string // Keywords table.
}

func (c ImportConfig) Enabled() bool {
	return c != ImportConfig{}
}

// Config represents the server configuration.
type Config struct {
	Locale string // Default locale. See locale directory for available languages.
	Root   string // Directory where board files are written to.
	Serve  string // Address:Port to listen for HTTP connections on.
	Header string // Client IP address header.

	SaltData string // Long random string of text used when one-way hashing data. Must not change once set.
	SaltPass string // Long random string of text used when two-way hashing data. Must not change once set.
	SaltTrip string // Long random string of text used when generating secure tripcodes. Must not change once set.

	Address  string // Address:Port to connect to the database.
	Username string // Database username.
	Password string // Database password.
	DBName   string // Database name.
	DBURL    string // Database connection URL.

	MailAddress  string // SMTP server Address:Port.
	MailTLS      bool   // Whether TLS is used to connect to the server.
	MailInsecure bool   // Whether TLS certificate verification is skipped.
	MailUsername string // SMTP server username.
	MailPassword string // SMTP server password.
	MailAuth     string // SMTP server authentication mechanism. May be challenge / plain / none.
	MailFrom     string // "From" email address.
	MailReplyTo  string // "Reply-To" email address.

	Mentions      int // Duration (in minutes) mention notifications are batched together.
	Notifications int // Duration (in minutes) non-mention notifications are batched together.

	Template string // Custom template directory.

	Identifiers bool // Whether staff may browse posts by IP hashes and boards may display identifier hashes.

	Uploads []string // Supported upload file types.

	Access map[string]string // Specifies which roles may perform each management or moderation action.

	Import ImportConfig // Board import configuration.

	// Calculated fields.
	cachedUploads  []*UploadType
	ImportMode     bool
	ImportComplete bool
	StartTime      time.Time
}

func (c *Config) UploadTypes() []*UploadType {
	if c.cachedUploads != nil {
		return c.cachedUploads
	}
	uploads := []*UploadType{}
	for _, upload := range c.Uploads {
		fields := strings.Fields(upload)
		if len(fields) < 2 {
			continue
		}
		u := &UploadType{
			Ext:  strings.ToLower(fields[0]),
			MIME: strings.ToLower(fields[1]),
		}
		if len(fields) > 2 {
			u.Thumb = fields[2]
		}
		uploads = append(uploads, u)
	}
	c.cachedUploads = uploads
	return uploads
}
