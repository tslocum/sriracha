package sriracha

type Report struct {
	ID        int
	Board     *Board
	Post      *Post
	Timestamp int64
	IP        string

	count int
}

func (r *Report) Count() int {
	return r.count
}
