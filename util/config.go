package util

import (
	"log"
	"strings"
	"time"
)

type UploadType struct {
	Ext   string
	MIME  string
	Thumb string
}

// Config represents the server configuration.
type Config struct {
	Locale      string // Default locale. See locale directory for available languages.
	Root        string // Directory where board files are written to.
	HTTP        string // Address:Port to listen for HTTP connections on.
	HTTPS       string // Address:Port to listen for HTTPS connections on.
	Header      string // Client IP address header.
	RejectHTTP1 bool   // Whether the server should reject HTTP/1 connections.

	HTTPSCert          string // Path to HTTPS certificate file.
	HTTPSKey           string // Path to HTTPS certificate private key file.
	InsecureSkipVerify bool   // Whether HTTPS certificate verification is skipped.

	Algorithm string // Hash algorithm used to generate sensitive one-way hashes. Supported algorithms are sha-2 and sha-3.
	SaltData  string // Long random string of text used when one-way hashing data. Must not change once set.
	SaltPass  string // Long random string of text used when two-way hashing data. Must not change once set.
	SaltTrip  string // Long random string of text used when generating secure tripcodes. Must not change once set.
	SaltIdent string // Long random string of text used when generating identifiers. Must not change once set.

	Address  string // Address:Port to connect to the database.
	Username string // Database username.
	Password string // Database password.
	DBName   string // Database name.
	DBURL    string // Database connection URL.

	Audit string // Audit database connection URL.

	MailAddress  string // SMTP server Address:Port.
	MailTLS      bool   // Whether TLS is used to connect to the server.
	MailInsecure bool   // Whether TLS certificate verification is skipped.
	MailUsername string // SMTP server username.
	MailPassword string // SMTP server password.
	MailAuth     string // SMTP server authentication mechanism. May be challenge / plain / none.
	MailFrom     string // "From" email address.
	MailReplyTo  string // "Reply-To" email address.
	MailDomains  string // Regular expression specifying allowed email address domains.

	Mentions      int // Duration (in minutes) mention notifications are batched together.
	Notifications int // Duration (in minutes) non-mention notifications are batched together.

	Identifiers bool // Whether staff may browse posts by IP hashes and boards may display identifier hashes.

	Styles []string // Available stylesheets.

	Template string // Custom template directory.

	Require2FA bool // Require two-factor authentication.

	SessionLimit int   // Account login session limit.
	SessionTime  int64 // Account login session expiration time, in seconds.

	NoIP bool // Do not record post author IP addresses.

	MinPageBuffer int // Initial static page buffer size, in bytes.
	MaxPageBuffer int // Maximum static page buffer size, in bytes.

	MaxFormBuffer int64 // Maximum multipart form buffer size, in bytes.

	MaxConns int // Maximum concurrent connections.

	Access map[string]string // Specifies which roles may perform each management or moderation action.

	WarnFree int64 // Free disk space warning threshold.
	MinFree  int64 // Minimum free disk space.

	Uploads []string // Supported upload file types.

	Import string // Import posts from DSN.

	// Calculated fields.
	cachedUploads  []*UploadType
	ImportMode     bool
	ImportComplete bool
	StartTime      time.Time

	// Obsolete fields.
	Serve string // Replaced by HTTP option.
}

func (c *Config) UploadTypes() []*UploadType {
	if c.cachedUploads != nil {
		return c.cachedUploads
	}
	uploads := []*UploadType{}
	for _, upload := range c.Uploads {
		fields := strings.Fields(upload)
		l := len(fields)
		if l != 2 && l != 3 {
			log.Fatalf("error: invalid entry in uploads configuration: expected 2 or 3 fields, found %d: %s\nexpected format: ext mime optional_thumb", l, upload)
		}
		u := &UploadType{
			Ext:  strings.ToLower(fields[0]),
			MIME: strings.ToLower(fields[1]),
		}
		if l == 3 {
			u.Thumb = fields[2]
		}
		uploads = append(uploads, u)
	}
	c.cachedUploads = uploads
	return uploads
}
