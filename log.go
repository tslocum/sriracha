package sriracha

import "time"

type Log struct {
	ID        int
	Board     *Board
	Timestamp int64
	Account   *Account
	Message   string
}

func (l *Log) TimestampDate() string {
	return time.Unix(l.Timestamp, 0).Format("2006-01-02 15:04:05 MST")
}
