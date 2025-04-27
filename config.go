package sriracha

import "strings"

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

type uploadType struct {
	Ext   string
	MIME  string
	Thumb string
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

	Min      int    // Minimum number of database connections to maintain in the pool.
	Max      int    // Maximum number of database connections to maintain in the pool.
	Address  string // Address:Port to connect to the database.
	Username string // Database username.
	Password string // Database password.
	DBName   string // Database name.

	Uploads []string // Supported upload file types.

	Import ImportConfig // Board import configuration.

	cachedUploads  []*uploadType
	importMode     bool
	importComplete bool
}

func (c *Config) UploadTypes() []*uploadType {
	if c.cachedUploads != nil {
		return c.cachedUploads
	}
	uploads := []*uploadType{}
	for _, upload := range c.Uploads {
		fields := strings.Fields(upload)
		if len(fields) < 2 {
			continue
		}
		u := &uploadType{
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
