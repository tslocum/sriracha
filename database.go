package sriracha

import . "codeberg.org/tslocum/sriracha/model"

// DB is an interface used by plugins to query and interact with the database.
type DB interface {
	// Config.
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

	// Account.
	AddAccount(a *Account, password string)
	AccountByID(id int) *Account
	AccountByUsername(username string) *Account
	AccountBySessionKey(sessionKey string) *Account
	AllAccounts() []*Account
	UpdateAccountUsername(a *Account)
	UpdateAccountRole(a *Account)
	UpdateAccountPassword(id int, password string)
	UpdateAccountLastActive(id int)
	UpdateAccountStyle(id int, style string)
	UpdateAccountLocale(id int, locale string)
	LoginAccount(username string, password string) *Account

	// Ban.
	AddBan(b *Ban)
	BanByID(id int) *Ban
	BanByIP(ip string) *Ban
	AllBans(rangeOnly bool) []*Ban
	UpdateBan(b *Ban)
	DeleteExpiredBans() int
	DeleteBan(id int)

	// File ban.
	AddFileBan(fileHash string)
	FileBanned(fileHash string) bool
	LiftFileBan(fileHash string)

	// Board.
	AddBoard(b *Board)
	BoardByID(id int) *Board
	BoardByDir(dir string) *Board
	UniqueUserPosts(b *Board) int
	AllBoards() []*Board
	DeleteBoard(id int)
	UpdateBoard(b *Board)

	// CAPTCHA.
	AddCAPTCHA(c *CAPTCHA)
	GetCAPTCHA(ip string) *CAPTCHA
	UpdateCAPTCHA(c *CAPTCHA)
	ExpiredCAPTCHAs() []*CAPTCHA
	DeleteCAPTCHA(ip string)
	NewCAPTCHAImage() string

	// Keyword.
	AddKeyword(k *Keyword)
	KeywordByID(id int) *Keyword
	KeywordByText(text string) *Keyword
	AllKeywords() []*Keyword
	UpdateKeyword(k *Keyword)
	DeleteKeyword(id int)

	// Log.
	AddLog(l *Log)
	LogCount() int
	LogsByPage(page int) []*Log

	// News.
	AddNews(n *News)
	NewsByID(id int) *News
	AllNews(onlyPublished bool) []*News
	UpdateNews(n *News)
	DeleteNews(id int)

	// Post.
	AddPost(p *Post)
	AllThreads(board *Board, moderated bool) [][2]int
	TrimThreads(board *Board) []*Post
	AllPostsInThread(postID int, moderated bool) []*Post
	AllReplies(threadID int, limit int, moderated bool) []*Post
	PendingPosts() []*Post
	PostByID(postID int) *Post
	PostsByIP(hash string) []*Post
	PostsByFileHash(hash string, filterBoard *Board) []*Post
	PostByField(b *Board, field string, value any) *Post
	LastPostByIP(board *Board, ip string) *Post
	ReplyCount(threadID int) int
	BumpThread(threadID int, timestamp int64)
	ModeratePost(postID int, moderated PostModerated)
	StickyPost(postID int, sticky bool)
	LockPost(postID int, lock bool)
	UpdatePostNameblock(postID int, nameblock string)
	UpdatePostMessage(postID int, message string)
	DeletePost(postID int)

	// Report.
	AddReport(r *Report)
	AllReports() []*Report
	NumReports(p *Post) int
	DeleteReports(p *Post)
}
