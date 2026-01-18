package sriracha

import . "codeberg.org/tslocum/sriracha/model"

type DB interface {
	AddLog(l *Log)

	HaveConfig(key string) bool

	GetString(key string) string
	SaveString(key string, value string)
	GetMultiString(key string) []string

	GetBool(key string) bool

	SaveBool(key string, value bool)
	SaveMultiString(key string, value []string)
	GetInt(key string) int

	GetInt64(key string) int64
	GetMultiInt(key string) []int

	SaveInt(key string, value int)

	GetFloat(key string) float64

	SaveFloat(key string, value float64)

	AccountByID(id int) *Account

	AllBoards() []*Board
	UniqueUserPosts(b *Board) int

	AllThreads(board *Board, moderated bool) [][2]int
	AllPostsInThread(postID int, moderated bool) []*Post
	PostByField(b *Board, field string, value any) *Post
}
