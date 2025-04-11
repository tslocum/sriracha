package sriracha

type ServerConfig struct {
	Root  string
	Serve string
	Salt  string

	Address  string
	Username string
	Password string
	Schema   string
}
