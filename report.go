package sriracha

type Report struct {
	ID        int
	Board     *Board
	Post      *Post
	Timestamp int64
	IP        string
}
