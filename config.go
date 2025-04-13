package sriracha

type Config struct {
	Root     string // Directory where board files are written to.
	Serve    string // Address:Port to listen for HTTP connections on.
	SaltData string // Long random string of text used when one-way hashing data. Must not change once set.
	SaltPass string // Long random string of text used when two-way hashing data. Must not change once set.

	Min      int    // Minimum number of database connections to maintain in the pool.
	Max      int    // Maximum number of database connections to maintain in the pool.
	Address  string // Address:Port to connect to the database.
	Username string // Database username.
	Password string // Database password.
	Schema   string // Database schema.
}
