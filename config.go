package sriracha

type Config struct {
	Root  string
	Serve string
	Salt  string

	Min      int
	Max      int
	Address  string
	Username string
	Password string
	Schema   string
}
