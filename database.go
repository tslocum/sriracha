// Package sriracha is a plugin interface for the Sriracha imageboard and forum server.
package sriracha

import . "codeberg.org/tslocum/sriracha/model"

// DB is an interface to the database used by plugins. This allows plugins to
// avoid importing pgx and its dependencies redundantly. See database package
// for method documentation.
type DB interface {
	// Config.
	HaveConfig(key string) bool
	GetString(key string) string
	SaveString(key string, value string)
	GetMultiString(key string) []string
	SaveMultiString(key string, value []string)
	GetBool(key string) bool
	SaveBool(key string, value bool)
	GetInt(key string) int
	SaveInt(key string, value int)
	GetMultiInt(key string) []int
	SaveMultiInt(key string, values []int)
	GetInt64(key string) int64
	SaveInt64(key string, value int64)
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
	UpdateAccountPassword(a *Account, password string)
	UpdateAccountLastActive(id int)
	UpdateAccountStyle(id int, style string)
	UpdateAccountLocale(id int, locale string)
	LoginAccount(username string, password string, newSession bool) (*Account, string)
	CheckAccountPassword(username string, password string) *Account
	ExpireAccountSessions()
	DeleteAccountSession(key string)

	// Ban.
	AddBan(b *Ban)
	BanByID(id int) *Ban
	BanByIP(ip string) *Ban
	AllActiveBans(rangeOnly bool) []*Ban
	LiftedBansByIP(ip string) []*Ban
	UpdateBan(b *Ban)
	LiftBan(id int, reason string)
	LiftExpiredBans() int

	// Banner.
	AddBanner(b *Banner)
	BannerByID(id int) *Banner
	BannerByName(name string) *Banner
	AllBanners() []*Banner
	UpdateBanner(b *Banner)
	DeleteBanner(id int)

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
	ClearBoardCache()

	// CAPTCHA.
	AddCAPTCHA(c *CAPTCHA)
	GetCAPTCHA(ip string) *CAPTCHA
	AllCAPTCHAs() []*CAPTCHA
	UpdateCAPTCHA(c *CAPTCHA)
	ExpiredCAPTCHAs() []*CAPTCHA
	DeleteCAPTCHA(ip string)
	NewCAPTCHAImage() string

	// Category.
	AddCategory(c *Category)
	CategoryByID(id int) *Category
	ChildCategories(id int) []*Category
	AllCategories() []*Category
	UpdateCategory(c *Category)
	DeleteCategory(id int)

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

	// Page.
	AddPage(p *Page)
	PageByID(id int) *Page
	PageByPath(path string) *Page
	AllPages() []*Page
	UpdatePage(p *Page)
	DeletePage(id int)

	// Post.
	AddPost(p *Post)
	AllThreads(filter PostFilter, board ...*Board) [][2]int
	PruneThreads(board *Board) []*Post
	AllPostsInThread(filter PostFilter, postID int) []*Post
	AllReplies(filter PostFilter, threadID int, limit int) []*Post
	PendingPosts() []*Post
	PrunedThreads() []int
	PostByID(postID int) *Post
	PostsByID(postIDs []int) []*Post
	PostsByIP(hash string) []*Post
	PostsByFileHash(hash string, filterBoard *Board) []*Post
	PostByField(b *Board, field string, value any) *Post
	LastPostByIP(board *Board, ip string) *Post
	LastPostByBoard(board *Board) *Post
	SearchPosts(filter PostFilter, query string, board ...*Board) []int
	HighlightPosts(query string, posts []*Post)
	NumPosts(filterBoard *Board, since int64) int
	ReplyCount(threadID int) int
	MaxPostID() int
	BumpThread(threadID int, timestamp int64)
	UnBumpThread(threadID int, timestamp int64)
	ModeratePost(postID int, moderated PostModerated)
	StickyPost(postID int, sticky bool)
	SpoilerPost(postID int, spoiler bool)
	LockPost(postID int, lock bool)
	UpdatePostBoard(postID int, board *Board)
	UpdatePostNameblock(postID int, nameblock string)
	UpdatePostMessage(postID int, message string)
	DeletePostAttachment(p *Post, staff bool)
	DeletePost(postID int)
	AddPostBacklink(target *Post, sourceID int)
	AddPostBacklinks(p *Post)
	HavePostBacklinks() bool

	// Report.
	AddReport(r *Report)
	AllReports() []*Report
	NumReports(p *Post) int
	PostReported(p *Post, ipHash string) bool
	DeleteReports(p *Post)

	// Subscription.
	AddSubscription(s *Subscription)
	SubscriptionByID(id int) *Subscription
	SubscriptionByIP(ip string) *Subscription
	SubscriptionsByEmail(email string) []*Subscription
	SubscriptionsByPost(p *Post, distinct bool, includeBoard bool) []*Subscription
	UpdateSubscription(s *Subscription)
	DeleteSubscription(s *Subscription)
	DeleteSubscriptionsByBoard(boardID int)
	DeleteSubscriptionsByPost(postID int)
	DeleteExpiredSubscriptions() int

	// Threshold.
	AddThreshold(t *Threshold)
	ThresholdByID(id int) *Threshold
	AllThresholds() []*Threshold
	UpdateThreshold(t *Threshold)
	ThresholdTimeout(t *Threshold, ipHash string, now int64) int
	DeleteThreshold(id int)

	// Two-factor.
	AddTwoFactor(t *TwoFactor)
	TwoFactorByID(id int) *TwoFactor
	TwoFactorsByAccount(accountID int) []*TwoFactor
	UpdateTwoFactor(t *TwoFactor)
	DeleteTwoFactor(id int)
}
