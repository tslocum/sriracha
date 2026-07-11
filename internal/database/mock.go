package database

import (
	"codeberg.org/tslocum/sriracha"
	. "codeberg.org/tslocum/sriracha/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func newTestThread(board *Board, size int) []*Post {
	var id int
	newPost := func() *Post {
		id++
		return &Post{
			ID:      id,
			Subject: "Subject",
			Name:    "Anonymous",
			Message: "Message",
		}
	}
	posts := make([]*Post, size)
	for i := 0; i < size; i++ {
		posts[i] = newPost()
		if i != 0 {
			posts[i].Parent = 1
		}
		posts[i].Board = board
		posts[i].SetNameBlock("Anonymous", "", false)
	}
	return posts
}

// mockDB represents a mock database.
type mockDB struct {
	boards     []*Board
	categories []*Category
	posts      []*Post
}

func newMockDB() *mockDB {
	db := &mockDB{}

	img := NewBoard()
	img.ID = 1
	img.Dir = "img"
	img.Name = "Imageboard"
	img.Type = TypeImageboard
	db.boards = append(db.boards, img)

	forum := NewBoard()
	forum.ID = 1
	forum.Dir = "forum"
	forum.Name = "Forum"
	forum.Type = TypeForum
	db.boards = append(db.boards, forum)

	db.categories = []*Category{
		{
			ID:     1,
			Boards: db.boards,
		},
	}

	db.posts = append(db.posts, newTestThread(img, 100)...)
	db.posts = append(db.posts, newTestThread(img, 10)...)
	db.posts = append(db.posts, newTestThread(img, 1)...)
	db.posts = append(db.posts, newTestThread(forum, 100)...)
	db.posts = append(db.posts, newTestThread(forum, 10)...)
	db.posts = append(db.posts, newTestThread(forum, 1)...)

	return db
}

func (db *mockDB) TestConn()             {}
func (db *mockDB) SetPlugin(name string) {}
func (db *mockDB) Exec(sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (db *mockDB) QueryRow(sql string, arguments ...any) pgx.Row { return nil }
func (db *mockDB) RollBack()                                     {}
func (db *mockDB) SoftRollBack()                                 {}
func (db *mockDB) Commit()                                       {}
func (db *mockDB) CommitWithErr() error                          { return nil }

// Config.
func (db *mockDB) HaveConfig(key string) bool                 { return false }
func (db *mockDB) GetString(key string) string                { return "" }
func (db *mockDB) SaveString(key string, value string)        {}
func (db *mockDB) GetMultiString(key string) []string         { return nil }
func (db *mockDB) SaveMultiString(key string, value []string) {}
func (db *mockDB) GetBool(key string) bool                    { return false }
func (db *mockDB) SaveBool(key string, value bool)            {}
func (db *mockDB) GetInt(key string) int                      { return 0 }
func (db *mockDB) SaveInt(key string, value int)              {}
func (db *mockDB) GetMultiInt(key string) []int               { return nil }
func (db *mockDB) SaveMultiInt(key string, values []int)      {}
func (db *mockDB) GetInt64(key string) int64                  { return 0 }
func (db *mockDB) SaveInt64(key string, value int64)          {}
func (db *mockDB) GetFloat(key string) float64                { return 0 }
func (db *mockDB) SaveFloat(key string, value float64)        {}

// Account.
func (db *mockDB) AddAccount(a *Account, password string)                 {}
func (db *mockDB) AccountByID(id int) *Account                            { return nil }
func (db *mockDB) AccountByUsername(username string) *Account             { return nil }
func (db *mockDB) AccountBySessionKey(sessionKey string) *Account         { return nil }
func (db *mockDB) AllAccounts() []*Account                                { return nil }
func (db *mockDB) UpdateAccountUsername(a *Account)                       {}
func (db *mockDB) UpdateAccountRole(a *Account)                           {}
func (db *mockDB) UpdateAccountPassword(id int, password string)          {}
func (db *mockDB) UpdateAccountLastActive(id int)                         {}
func (db *mockDB) UpdateAccountStyle(id int, style string)                {}
func (db *mockDB) UpdateAccountLocale(id int, locale string)              {}
func (db *mockDB) LoginAccount(username string, password string) *Account { return nil }

// Ban.
func (db *mockDB) AddBan(b *Ban)                 {}
func (db *mockDB) BanByID(id int) *Ban           { return nil }
func (db *mockDB) BanByIP(ip string) *Ban        { return nil }
func (db *mockDB) AllBans(rangeOnly bool) []*Ban { return nil }
func (db *mockDB) UpdateBan(b *Ban)              {}
func (db *mockDB) DeleteExpiredBans() int        { return 0 }
func (db *mockDB) DeleteBan(id int)              {}

// Banner.
func (db *mockDB) AddBanner(b *Banner)              {}
func (db *mockDB) BannerByID(id int) *Banner        { return nil }
func (db *mockDB) BannerByName(name string) *Banner { return nil }
func (db *mockDB) AllBanners() []*Banner            { return nil }
func (db *mockDB) UpdateBanner(b *Banner)           {}
func (db *mockDB) DeleteBanner(id int)              {}

// File ban.
func (db *mockDB) AddFileBan(fileHash string)      {}
func (db *mockDB) FileBanned(fileHash string) bool { return false }
func (db *mockDB) LiftFileBan(fileHash string)     {}

// Board.
func (db *mockDB) AddBoard(b *Board) {}
func (db *mockDB) BoardByID(id int) *Board {
	for _, b := range db.boards {
		if b.ID == id {
			return b
		}
	}
	return nil
}
func (db *mockDB) BoardByDir(dir string) *Board {
	for _, b := range db.boards {
		if b.Dir == dir {
			return b
		}
	}
	return nil
}
func (db *mockDB) UniqueUserPosts(b *Board) int { return 2 }
func (db *mockDB) AllBoards() []*Board          { return db.boards }
func (db *mockDB) DeleteBoard(id int)           {}
func (db *mockDB) UpdateBoard(b *Board)         {}
func (db *mockDB) ClearBoardCache()             {}

// CAPTCHA.
func (db *mockDB) AddCAPTCHA(c *CAPTCHA)         {}
func (db *mockDB) GetCAPTCHA(ip string) *CAPTCHA { return nil }
func (db *mockDB) AllCAPTCHAs() []*CAPTCHA       { return nil }
func (db *mockDB) UpdateCAPTCHA(c *CAPTCHA)      {}
func (db *mockDB) ExpiredCAPTCHAs() []*CAPTCHA   { return nil }
func (db *mockDB) DeleteCAPTCHA(ip string)       {}
func (db *mockDB) NewCAPTCHAImage() string       { return "" }

// Category.
func (db *mockDB) AddCategory(c *Category)            {}
func (db *mockDB) CategoryByID(id int) *Category      { return nil }
func (db *mockDB) ChildCategories(id int) []*Category { return nil }
func (db *mockDB) AllCategories() []*Category         { return db.categories }
func (db *mockDB) UpdateCategory(c *Category)         {}
func (db *mockDB) DeleteCategory(id int)              {}

// Keyword.
func (db *mockDB) AddKeyword(k *Keyword)              {}
func (db *mockDB) KeywordByID(id int) *Keyword        { return nil }
func (db *mockDB) KeywordByText(text string) *Keyword { return nil }
func (db *mockDB) AllKeywords() []*Keyword            { return nil }
func (db *mockDB) UpdateKeyword(k *Keyword)           {}
func (db *mockDB) DeleteKeyword(id int)               {}

// Log.
func (db *mockDB) AddLog(l *Log)              {}
func (db *mockDB) LogCount() int              { return 0 }
func (db *mockDB) LogsByPage(page int) []*Log { return nil }

// News.
func (db *mockDB) AddNews(n *News)                    {}
func (db *mockDB) NewsByID(id int) *News              { return nil }
func (db *mockDB) AllNews(onlyPublished bool) []*News { return nil }
func (db *mockDB) UpdateNews(n *News)                 {}
func (db *mockDB) DeleteNews(id int)                  {}

// Page.
func (db *mockDB) AddPage(p *Page)              {}
func (db *mockDB) PageByID(id int) *Page        { return nil }
func (db *mockDB) PageByPath(path string) *Page { return nil }
func (db *mockDB) AllPages() []*Page            { return nil }
func (db *mockDB) UpdatePage(p *Page)           {}
func (db *mockDB) DeletePage(id int)            {}

// Post.
func (db *mockDB) AddPost(p *Post) {}
func (db *mockDB) AllThreads(board *Board, moderated bool) [][2]int {
	var threads [][2]int
	for _, post := range db.posts {
		if post.Board != board || post.Parent == 0 {
			threads = append(threads, [2]int{post.ID, db.ReplyCount(post.ID)})
		}
	}
	return threads
}
func (db *mockDB) TrimThreads(board *Board) []*Post { return nil }
func (db *mockDB) AllPostsInThread(postID int, moderated bool) []*Post {
	var posts []*Post
	for _, post := range db.posts {
		if post.ID == postID || post.Parent == postID {
			posts = append(posts, post)
		}
	}
	return posts
}
func (db *mockDB) AllReplies(threadID int, limit int, moderated bool) []*Post { return nil }
func (db *mockDB) PendingPosts() []*Post                                      { return nil }
func (db *mockDB) PostByID(postID int) *Post                                  { return nil }
func (db *mockDB) PostsByIP(hash string) []*Post                              { return nil }
func (db *mockDB) PostsByFileHash(hash string, filterBoard *Board) []*Post    { return nil }
func (db *mockDB) PostByField(b *Board, field string, value any) *Post        { return nil }
func (db *mockDB) LastPostByIP(board *Board, ip string) *Post                 { return nil }
func (db *mockDB) LastPostByBoard(board *Board) *Post                         { return nil }
func (db *mockDB) NumPosts(filterBoard *Board, since int64) int               { return 0 }
func (db *mockDB) ReplyCount(threadID int) int {
	var replies int
	for _, post := range db.posts {
		if post.Parent == threadID {
			replies++
		}
	}
	return replies
}
func (db *mockDB) MaxPostID() int                                   { return 0 }
func (db *mockDB) BumpThread(threadID int, timestamp int64)         {}
func (db *mockDB) ModeratePost(postID int, moderated PostModerated) {}
func (db *mockDB) StickyPost(postID int, sticky bool)               {}
func (db *mockDB) LockPost(postID int, lock bool)                   {}
func (db *mockDB) UpdatePostBoard(postID int, board *Board)         {}
func (db *mockDB) UpdatePostNameblock(postID int, nameblock string) {}
func (db *mockDB) UpdatePostMessage(postID int, message string)     {}
func (db *mockDB) DeletePost(postID int)                            {}
func (db *mockDB) AddPostBacklink(target *Post, sourceID int)       {}
func (db *mockDB) AddPostBacklinks(p *Post)                         {}
func (db *mockDB) HavePostBacklinks() bool                          { return false }

// Report.
func (db *mockDB) AddReport(r *Report)    {}
func (db *mockDB) AllReports() []*Report  { return nil }
func (db *mockDB) NumReports(p *Post) int { return 0 }
func (db *mockDB) DeleteReports(p *Post)  {}

// Subscription.
func (db *mockDB) AddSubscription(s *Subscription)                   {}
func (db *mockDB) SubscriptionByID(id int) *Subscription             { return nil }
func (db *mockDB) SubscriptionByIP(ip string) *Subscription          { return nil }
func (db *mockDB) SubscriptionsByEmail(email string) []*Subscription { return nil }
func (db *mockDB) SubscriptionsByPost(p *Post, distinct bool, includeBoard bool) []*Subscription {
	return nil
}
func (db *mockDB) UpdateSubscription(s *Subscription)     {}
func (db *mockDB) DeleteSubscription(s *Subscription)     {}
func (db *mockDB) DeleteSubscriptionsByBoard(boardID int) {}
func (db *mockDB) DeleteSubscriptionsByPost(postID int)   {}
func (db *mockDB) DeleteExpiredSubscriptions() int        { return 0 }

// Threshold.
func (db *mockDB) AddThreshold(t *Threshold)       {}
func (db *mockDB) ThresholdByID(id int) *Threshold { return nil }
func (db *mockDB) AllThresholds() []*Threshold     { return nil }
func (db *mockDB) UpdateThreshold(t *Threshold)    {}
func (db *mockDB) DeleteThreshold(id int)          {}

var MockDB = newMockDB()

// Validate mock database interface during compilation.
var (
	_ sriracha.DB = &mockDB{}
)
