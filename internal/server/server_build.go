package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"time"

	"codeberg.org/tslocum/gotext"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

type buildType int

const (
	buildBoardIndex buildType = iota
	buildBoardCatalog
	buildBoardThread
	buildPage
	buildSiteIndex
	buildStatistics
)

// buildInfo contains information used to request building a page.
type buildInfo struct {
	build      buildType
	board      *Board
	threads    [][2]int
	post       int
	page       int
	customPage *Page
	db         serverDB
	postIDs    [][]int
	wg         *sync.WaitGroup
}

func (s *Server) _buildBoardIndex(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if trace {
		traceT = time.Now()
	}
	db := info.db
	board := info.board

	data := s.newTemplateData()
	data.Board = board
	data.Boards = db.AllBoards()

	data.ReplyMode = 0
	data.Template = "board_page"
	data.Pages = pageCount(len(info.threads), board.Threads)
	checkCache := board.Type == TypeImageboard && len(s.indexCache[board.ID]) > 0
	page := info.page

	existingIDs := func(page int) []int {
		if info.postIDs == nil || page < 0 || page > len(info.postIDs)-1 {
			return nil
		}
		return info.postIDs[page]
	}

	fileName := "index.html"
	if page > 0 {
		fileName = fmt.Sprintf("%d.html", page)
	}

	start := page * board.Threads
	end := len(info.threads)
	if board.Threads != 0 && end > start+board.Threads {
		end = start + board.Threads
	}

	data.Threads = data.Threads[:0]
	for _, threadInfo := range info.threads[start:end] {
		thread := db.PostByID(threadInfo[0])
		thread.Replies = threadInfo[1]
		posts := []*Post{thread}
		if board.Type == TypeImageboard {
			posts = append(posts, db.AllReplies(thread.ID, board.Replies, true)...)
		}
		for i := range posts {
			s.indexCache[board.ID][page] = append(s.indexCache[board.ID][page], posts[i].ID)
		}
		data.Threads = append(data.Threads, posts)
	}
	if checkCache && len(s.indexCache[board.ID][page]) > 0 && slices.Equal(s.indexCache[board.ID][page], existingIDs(page)) {
		if trace {
			traceD = time.Since(traceT)
			traceLog(board.Path()+fileName+" (skipped)", traceD)
		}
		return
	}
	data.Page = page

	writePath := filepath.Join(s.config.Root, board.Dir, "_"+fileName)
	filePath := filepath.Join(s.config.Root, board.Dir, fileName)

	indexFile, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}
	data.execute(indexFile)

	indexFile.Close()
	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if trace {
		traceD = time.Since(traceT)
		traceLog(board.Path()+fileName, traceD)
	}
}

func (s *Server) _buildBoardCatalog(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if trace {
		traceT = time.Now()
	}
	db := info.db
	board := info.board

	if board.Type != TypeImageboard {
		return
	}
	data := s.newTemplateData()
	data.Board = board
	data.Boards = db.AllBoards()
	data.ReplyMode = 1
	data.Template = "board_catalog"

	writePath := filepath.Join(s.config.Root, board.Dir, "_catalog.html")
	filePath := filepath.Join(s.config.Root, board.Dir, "catalog.html")

	catalogFile, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	for _, threadInfo := range info.threads {
		thread := db.PostByID(threadInfo[0])
		thread.Replies = threadInfo[1]
		data.Threads = append(data.Threads, []*Post{thread})
	}
	data.execute(catalogFile)

	catalogFile.Close()
	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if trace {
		traceD = time.Since(traceT)
		traceLog(board.Path()+"catalog.html", traceD)
	}
}

func (s *Server) _buildBoardThread(info *buildInfo) {
	db := info.db
	board := info.board
	postID := info.post

	posts := db.AllPostsInThread(true, postID)
	if len(posts) == 0 {
		return
	}

	writePath := filepath.Join(s.config.Root, board.Dir, "res", fmt.Sprintf("_%d.html", postID))
	filePath := filepath.Join(s.config.Root, board.Dir, "res", fmt.Sprintf("%d.html", postID))

	f, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	data := s.newTemplateData()
	data.Board = board
	data.Boards = db.AllBoards()
	data.Threads = [][]*Post{posts}
	data.ReplyMode = postID
	data.Template = "board_page"
	data.execute(f)

	f.Close()
	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) _buildPage(info *buildInfo) {
	db := info.db

	data := s.newTemplateData()
	data.Boards = db.AllBoards()
	data.Template = "page"
	p := info.customPage

	writePath := filepath.Join(s.config.Root, p.Path+"_.html")
	filePath := filepath.Join(s.config.Root, p.Path+".html")

	pageFile, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	err = s.writePage(db, data, p, pageFile)
	pageFile.Close()
	if err != nil {
		log.Printf("warning: skipped invalid page %s: %s", p.Path, err)
		return
	}
	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Printf("warning: skipped invalid page %s: %s", p.Path, err)
	}
}

func (s *Server) _buildSiteIndex(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if trace {
		traceT = time.Now()
	}
	db := info.db

	if db.BoardByDir("") != nil {
		return
	}

	allBoards := db.AllBoards()
	var keep []*Board
	for _, board := range allBoards {
		if board.Hide == HideIndex || board.Hide == HideEverywhere {
			continue
		}
		keep = append(keep, board)
	}
	allBoards = keep
	if len(allBoards) == 0 {
		return
	}
	data := s.newTemplateData()
	data.Template = "index"

	data.Boards = allBoards

	if s.opt.News != NewsDisable {
		allNews := db.AllNews(true)
		var latest *News
		if len(allNews) > 0 {
			latest = allNews[0]
		}
		data.News = latest
	}

	s.refreshRecentPosts(db)

	writePath := filepath.Join(s.config.Root, "_index.html")
	filePath := filepath.Join(s.config.Root, "index.html")

	indexFile, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}
	data.execute(indexFile)
	indexFile.Close()
	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if trace {
		traceD = time.Since(traceT)
		traceLog("Site index", traceD)
	}
}

func (s *Server) _buildStatistics(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if trace {
		traceT = time.Now()
	}
	db := info.db

	serverStats := &ServerStats{
		Name:      s.opt.SiteName,
		About:     s.opt.SiteDescription,
		Generated: time.Now().Unix(),
	}
	thirtyDays := serverStats.Generated - 2592000
	for _, c := range s.opt.Categories {
		for _, b := range c.Boards {
			boardStats := BoardStats{
				Dir:   b.Dir,
				Name:  b.Name,
				About: b.Description,
				Month: db.NumPosts(b, thirtyDays),
				Total: db.NumPosts(b, 0),
			}
			recent := db.LastPostByBoard(b)
			if recent != nil {
				boardStats.Recent = recent.URL(s.opt.SiteHome)
			}
			serverStats.Boards = append(serverStats.Boards, boardStats)

			serverStats.Month += boardStats.Month
			serverStats.Total += boardStats.Total
		}
	}

	if s.statsCache != nil && reflect.DeepEqual(serverStats, s.statsCache) {
		if trace {
			traceD = time.Since(traceT)
			traceLog("/stats.json (skipped)", traceD)
		}
		return
	}

	writePath := filepath.Join(s.config.Root, "stats_.json")
	filePath := filepath.Join(s.config.Root, "stats.json")

	file, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	statsJSON, err := json.MarshalIndent(serverStats, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	_, err = file.Write(statsJSON)
	if err != nil {
		log.Fatal(err)
	}

	file.Close()

	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}
	s.statsCache = serverStats

	if trace {
		traceD = time.Since(traceT)
		traceLog("/stats.json", traceD)
	}
}

func (s *Server) _build() {
	for {
		info := <-s.buildQueue
		db := s.begin()
		info.db = db
		switch info.build {
		case buildBoardIndex:
			s._buildBoardIndex(info)
		case buildBoardCatalog:
			s._buildBoardCatalog(info)
		case buildBoardThread:
			s._buildBoardThread(info)
		case buildPage:
			s._buildPage(info)
		case buildSiteIndex:
			s._buildSiteIndex(info)
		case buildStatistics:
			s._buildStatistics(info)
		}
		db.Commit()
		info.wg.Done()
	}
}

// rebuildBoard rebuilds all pages in a board.
func (s *Server) rebuildBoard(db serverDB, wg *sync.WaitGroup, board *Board) {
	s.indexCache[board.ID] = nil

	threads := s.writeBoardIndexes(db, wg, board)
	for _, threadInfo := range threads {
		s.writeThread(db, wg, board, threadInfo[0])
	}
}

// writeBoardIndexes writes board index pages to disk.
func (s *Server) writeBoardIndexes(db serverDB, wg *sync.WaitGroup, board *Board, overboards ...*Board) [][2]int {
	if board.Unique == 0 {
		board.Unique = db.UniqueUserPosts(board)
	}

	var threads [][2]int
	if board.ID > 0 {
		threads = db.AllThreads(true, board)
	} else {
		threads = db.AllThreads(true, overboards...)
	}

	pages := pageCount(len(threads), board.Threads)
	allPostIDs := make([][]int, len(s.indexCache[board.ID]))
	for i := range s.indexCache[board.ID] {
		allPostIDs[i] = make([]int, len(s.indexCache[board.ID][i]))
		copy(allPostIDs[i], s.indexCache[board.ID][i])
	}
	if len(s.indexCache[board.ID]) < pages {
		s.indexCache[board.ID] = make([][]int, pages)
		for i := range allPostIDs {
			s.indexCache[board.ID][i] = make([]int, len(allPostIDs[i]))
			copy(s.indexCache[board.ID][i], allPostIDs[i])
		}
	}
	for build := buildBoardIndex; build <= buildBoardCatalog; build++ {
		info := &buildInfo{
			build:   build,
			board:   board,
			threads: threads,
			postIDs: allPostIDs,
			wg:      wg,
		}
		wg.Add(1)
		s.buildQueue <- info
		if build == buildBoardIndex {
			for page := 1; page < pages; page++ {
				info := &buildInfo{
					build:   build,
					board:   board,
					page:    page,
					threads: threads,
					postIDs: allPostIDs,
					wg:      wg,
				}
				wg.Add(1)
				s.buildQueue <- info
			}
		}
	}
	return threads
}

// writeThread writes a thread res page to disk.
func (s *Server) writeThread(db serverDB, wg *sync.WaitGroup, board *Board, postID int) {
	if board.Unique == 0 {
		board.Unique = db.UniqueUserPosts(board)
	}

	info := &buildInfo{
		build: buildBoardThread,
		board: board,
		post:  postID,
		wg:    wg,
	}
	wg.Add(1)
	s.buildQueue <- info
}

// writeOverboard writes overboard pages to disk.
func (s *Server) writeOverboard(db serverDB, wg *sync.WaitGroup, c *categoryInfo) {
	var id int
	if c != nil {
		id = c.ID * -1
	}

	dir := s.opt.Overboard
	if c != nil {
		dir = c.Overboard
	}

	name := gotext.Get("Overboard")
	if c != nil && c.Name != "" {
		name = c.Name
	}

	var overboardDir string
	overboardPath := "/"
	if dir != "/" {
		overboardDir = dir
		overboardPath += dir + "/"
	}

	board := &Board{
		ID:      id,
		Type:    s.opt.OverboardType,
		Name:    name,
		Dir:     overboardDir,
		Threads: s.opt.OverboardThreads,
		Replies: s.opt.OverboardReplies,
	}

	var boards []*Board
	if c != nil {
		boards = c.Boards
	}
	s.writeBoardIndexes(db, wg, board, boards...)
}

// writePages writes custom pages to disk.
func (s *Server) writePages(db serverDB, wg *sync.WaitGroup, pages []*Page) {
	wg.Add(len(pages))
	for _, p := range pages {
		info := &buildInfo{
			build:      buildPage,
			customPage: p,
			wg:         wg,
		}
		s.buildQueue <- info
	}
}

// writeSiteIndex writes the site index page to disk.
func (s *Server) writeSiteIndex(wg *sync.WaitGroup) {
	if !s.opt.SiteIndex || s.opt.News == NewsWriteToIndex || s.opt.Overboard == "/" {
		return
	}
	wg.Add(1)
	info := &buildInfo{
		build: buildSiteIndex,
		wg:    wg,
	}
	s.buildQueue <- info
}

func (s *Server) writeStatistics(wg *sync.WaitGroup) {
	if !s.opt.Statistics {
		return
	}
	wg.Add(1)
	info := &buildInfo{
		build: buildStatistics,
		wg:    wg,
	}
	s.buildQueue <- info
}
