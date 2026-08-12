package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/tslocum/gotext"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

const (
	initialBufferSize = 500000  // 500 KB.
	maxBufferSize     = 4000000 // 4 MB.
)

type buildType int

const (
	buildBoardIndex buildType = iota
	buildBoardCatalog
	buildBoardThread
	buildNewsIndex
	buildNewsEntry
	buildPage
	buildSiteIndex
	buildStatistics
	queueBoardIndexes
)

const newsCount = 10

// buildInfo contains information used to request building a page.
type buildInfo struct {
	build      buildType
	board      *Board
	archive    bool
	overboards []*Board
	threads    [][2]int
	post       int
	news       []*News
	page       int
	customPage *Page
	db         serverDB
	postIDs    [][]int
	buf        *bytes.Buffer
	wg         *sync.WaitGroup
	delta      *atomic.Uint32
}

func (s *Server) _buildBoardIndex(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if s.opt.trace {
		traceT = time.Now()
	}
	db := info.db
	board := info.board
	cacheID := board.ID
	if info.archive {
		cacheID = math.MaxInt32 - board.ID
	}

	data := s.newTemplateData(db, info.buf)
	data.Board = board
	data.Boards = db.AllBoards()

	data.ReplyMode = 0
	data.ArchiveMode = info.archive
	data.Template = "board_page"
	data.Pages = pageCount(len(info.threads), board.Threads)
	checkCache := board.Type == TypeImageboard && len(s.indexCache[cacheID]) > 0
	page := info.page

	existingIDs := func(page int) []int {
		if info.postIDs == nil || page < 0 || page > len(info.postIDs)-1 {
			return nil
		}
		return info.postIDs[page]
	}

	var fileName string
	if !info.archive {
		if page == 0 {
			fileName = "index.html"
		} else {
			fileName = fmt.Sprintf("%d.html", page)
		}
	} else {
		if page == 0 {
			fileName = "archive.html"
		} else {
			fileName = fmt.Sprintf("archive-%d.html", page)
		}
	}

	start := page * board.Threads
	end := len(info.threads)
	if board.Threads != 0 && end > start+board.Threads {
		end = start + board.Threads
	}

	data.Threads = data.Threads[:0]
	threadIDs := make([]int, len(info.threads[start:end]))
	for i, threadInfo := range info.threads[start:end] {
		threadIDs[i] = threadInfo[0]
	}
	threadPosts := db.PostsByID(threadIDs)
	var postIDs []int
	for i, threadInfo := range info.threads[start:end] {
		threadPosts[i].Replies = threadInfo[1]
		posts := []*Post{threadPosts[i]}
		if board.Type == TypeImageboard {
			posts = append(posts, db.AllReplies(FilterVisible, threadPosts[i].ID, board.Replies)...)
		}
		for i := range posts {
			postIDs = append(postIDs, posts[i].ID)
		}
		data.Threads = append(data.Threads, posts)
	}
	s.indexCacheLock.Lock()
	if cap(s.indexCache[cacheID][page]) != len(postIDs) {
		s.indexCache[cacheID][page] = postIDs
	} else {
		copy(s.indexCache[cacheID][page], postIDs)
	}
	if checkCache && len(s.indexCache[cacheID][page]) > 0 && slices.Equal(s.indexCache[cacheID][page], existingIDs(page)) {
		s.indexCacheLock.Unlock()
		return
	}
	s.indexCacheLock.Unlock()
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

	if s.opt.trace {
		traceD = time.Since(traceT)
		traceLog(board.Path()+fileName, traceD)
		info.delta.Add(uint32(traceD.Milliseconds()))
	}
}

func (s *Server) _buildBoardCatalog(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if s.opt.trace {
		traceT = time.Now()
	}
	db := info.db
	board := info.board

	if board.Type != TypeImageboard {
		return
	}
	data := s.newTemplateData(db, info.buf)
	data.Board = board
	data.Boards = db.AllBoards()
	data.ReplyMode = 1
	data.ArchiveMode = info.archive
	data.Template = "board_catalog"

	var fileName string
	if !info.archive {
		fileName = "catalog.html"
	} else {
		fileName = "archive-catalog.html"
	}
	writePath := filepath.Join(s.config.Root, board.Dir, "_"+fileName)
	filePath := filepath.Join(s.config.Root, board.Dir, fileName)

	catalogFile, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	ids := make([]int, len(info.threads))
	for i, threadInfo := range info.threads {
		ids[i] = threadInfo[0]
	}
	posts := db.PostsByID(ids)
	data.Threads = make([][]*Post, len(info.threads))
	for i, threadInfo := range info.threads {
		posts[i].Replies = threadInfo[1]
		data.Threads[i] = append(data.Threads[i], posts[i])
	}

	data.execute(catalogFile)

	catalogFile.Close()

	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if s.opt.trace {
		traceD = time.Since(traceT)
		traceLog(board.Path()+fileName, traceD)
		info.delta.Add(uint32(traceD.Milliseconds()))
	}
}

func (s *Server) _queueBoardIndexes(info *buildInfo) {
	db := info.db
	board := info.board
	wg := info.wg
	cacheID := board.ID
	if info.archive {
		cacheID = math.MaxInt32 - cacheID
	}

	if board.Unique == 0 {
		board.Unique = db.UniqueUserPosts(board)
		board.ArchiveSize = len(db.AllThreads(FilterArchived, board))
	}

	postFilter := FilterActive
	if info.archive {
		postFilter = FilterArchived
	}

	var threads [][2]int
	if board.ID > 0 {
		threads = db.AllThreads(postFilter, board)
	} else {
		threads = db.AllThreads(postFilter, info.overboards...)
	}

	pages := pageCount(len(threads), board.Threads)
	s.indexCacheLock.Lock()
	allPostIDs := make([][]int, len(s.indexCache[cacheID]))
	for i := range s.indexCache[cacheID] {
		allPostIDs[i] = make([]int, len(s.indexCache[cacheID][i]))
		copy(allPostIDs[i], s.indexCache[cacheID][i])
	}
	if len(s.indexCache[cacheID]) < pages {
		s.indexCache[cacheID] = make([][]int, pages)
		for i := range allPostIDs {
			s.indexCache[cacheID][i] = make([]int, len(allPostIDs[i]))
			copy(s.indexCache[cacheID][i], allPostIDs[i])
		}
	}
	s.indexCacheLock.Unlock()
	var buf *bytes.Buffer
	for build := buildBoardIndex; build <= buildBoardCatalog; build++ {
		info := &buildInfo{
			build:   build,
			board:   board,
			archive: info.archive,
			threads: threads,
			postIDs: allPostIDs,
			wg:      wg,
			delta:   info.delta,
		}
		wg.Add(1)
		select {
		case s.buildQueue <- info:
		default:
			if buf == nil {
				buf = bytes.NewBuffer(make([]byte, initialBufferSize))
			}
			buf.Reset()
			info.buf = buf
			info.db = db
			if build == buildBoardIndex {
				s._buildBoardIndex(info)
			} else {
				s._buildBoardCatalog(info)
			}
			wg.Done()
		}
		if build == buildBoardIndex {
			for page := 1; page < pages; page++ {
				info := &buildInfo{
					build:   build,
					board:   board,
					archive: info.archive,
					page:    page,
					threads: threads,
					postIDs: allPostIDs,
					wg:      wg,
					delta:   info.delta,
				}
				wg.Add(1)
				select {
				case s.buildQueue <- info:
				default:
					if buf == nil {
						buf = bytes.NewBuffer(make([]byte, initialBufferSize))
					}
					buf.Reset()
					info.buf = buf
					info.db = db
					s._buildBoardIndex(info)
					wg.Done()
				}
			}
		}
	}
}

func (s *Server) _buildBoardThread(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if s.opt.trace {
		traceT = time.Now()
	}
	db := info.db
	board := info.board
	postID := info.post

	posts := db.AllPostsInThread(FilterVisible, postID)
	if len(posts) == 0 {
		return
	}

	writePath := filepath.Join(s.config.Root, board.Dir, "res", fmt.Sprintf("_%d.html", postID))
	filePath := filepath.Join(s.config.Root, board.Dir, "res", fmt.Sprintf("%d.html", postID))

	f, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	data := s.newTemplateData(db, info.buf)
	data.Board = board
	data.Boards = db.AllBoards()
	data.Threads = [][]*Post{posts}
	data.ReplyMode = postID
	data.ArchiveMode = posts[0].Archived()
	data.Template = "board_page"

	data.execute(f)

	f.Close()

	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if s.opt.trace {
		traceD = time.Since(traceT)
		traceLog(board.Path()+fmt.Sprintf("res/%d.html", postID), traceD)
		info.delta.Add(uint32(traceD.Milliseconds()))
	}
}

func (s *Server) _buildNewsIndex(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if s.opt.trace {
		traceT = time.Now()
	}
	db := info.db
	page := info.page

	data := s.newTemplateData(db, info.buf)
	data.Boards = db.AllBoards()
	data.Template = "news"

	data.Pages = pageCount(len(info.news), newsCount)

	fileName := "news.html"
	if s.opt.News == NewsWriteToIndex {
		fileName = "index.html"
	}
	if page > 0 {
		fileName = fmt.Sprintf("news-p%d.html", page)
	}

	writePath := filepath.Join(s.config.Root, "_"+fileName)
	filePath := filepath.Join(s.config.Root, fileName)

	indexFile, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	start := page * newsCount
	end := len(info.news)
	if newsCount != 0 && end > start+newsCount {
		end = start + newsCount
	}

	data.AllNews = info.news[start:end]
	data.Page = page

	subData := s.newTemplateData(db, info.buf)
	buf := &bytes.Buffer{}
	for _, n := range data.AllNews {
		subData.Boards = data.Boards
		subData.Template = "line"
		subData.tpl, err = s.tplOriginal.Clone()
		if err != nil {
			log.Fatal(err)
		}
		subData.tpl, err = subData.tpl.New("line").Parse(n.Message)
		if err != nil {
			log.Printf("warning: skipped invalid news entry %d: %s", n.ID, err)
			continue
		}
		err = subData.executeWithError(buf)
		if err != nil {
			log.Printf("warning: skipped invalid news entry %d: %s", n.ID, err)
			continue
		}
		n.Message = buf.String()
		buf.Reset()
	}

	data.execute(indexFile)

	indexFile.Close()

	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if s.opt.trace {
		traceD = time.Since(traceT)
		traceLog("/"+fileName, traceD)
		info.delta.Add(uint32(traceD.Milliseconds()))
	}
}

func (s *Server) _buildNewsEntry(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if s.opt.trace {
		traceT = time.Now()
	}
	db := info.db
	n := info.news[0]
	if n.ID <= 0 {
		return
	}

	data := s.newTemplateData(db, info.buf)
	data.Boards = db.AllBoards()
	data.Template = "news"
	data.AllNews = []*News{n}
	data.Pages = 1
	data.Extra = "view"

	writePath := filepath.Join(s.config.Root, fmt.Sprintf("_news-%d.html", n.ID))
	filePath := filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", n.ID))

	itemFile, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	subData := s.newTemplateData(db, info.buf)
	buf := &bytes.Buffer{}
	subData.Boards = data.Boards
	subData.Template = "line"
	subData.tpl, err = s.tplOriginal.Clone()
	if err != nil {
		log.Fatal(err)
	}
	subData.tpl, err = subData.tpl.New("line").Parse(n.Message)
	if err != nil {
		log.Printf("warning: skipped invalid news entry %d: %s", n.ID, err)
		return
	}
	err = subData.executeWithError(buf)
	if err != nil {
		log.Printf("warning: skipped invalid news entry %d: %s", n.ID, err)
		return
	}
	n.Message = buf.String()
	buf.Reset()

	data.execute(itemFile)

	itemFile.Close()

	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if s.opt.trace {
		traceD = time.Since(traceT)
		traceLog(fmt.Sprintf("/news-%d.html", n.ID), traceD)
		info.delta.Add(uint32(traceD.Milliseconds()))
	}
}

func (s *Server) _buildPage(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if s.opt.trace {
		traceT = time.Now()
	}
	db := info.db

	data := s.newTemplateData(db, info.buf)
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
		log.Printf("%s content:", p.Path)
		for i, line := range strings.Split(p.Content, "\n") {
			log.Printf("%d %s", i+1, line)
		}
		log.Printf("warning: skipped invalid page %s: %s", p.Path, err)
		return
	}

	err = os.Rename(writePath, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if s.opt.trace {
		traceD = time.Since(traceT)
		traceLog(fmt.Sprintf("/%s.html", p.Path), traceD)
		info.delta.Add(uint32(traceD.Milliseconds()))
	}
}

func (s *Server) _buildSiteIndex(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if s.opt.trace {
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
	if len(keep) == 0 {
		return
	}
	data := s.newTemplateData(db, info.buf)
	data.Template = "index"

	data.Boards = keep

	if s.opt.News != NewsDisable {
		allNews := db.AllNews(true)
		if len(allNews) > 0 {
			n := allNews[0]
			subData := s.newTemplateData(db, info.buf)
			buf := &bytes.Buffer{}
			subData.Boards = allBoards
			subData.Template = "line"
			var err error
			subData.tpl, err = s.tplOriginal.Clone()
			if err != nil {
				log.Fatal(err)
			}
			subData.tpl, err = subData.tpl.New("line").Parse(n.Message)
			if err != nil {
				log.Printf("warning: skipped invalid news entry %d: %s", n.ID, err)
			} else {
				err = subData.executeWithError(buf)
				if err != nil {
					log.Printf("warning: skipped invalid news entry %d: %s", n.ID, err)
				} else {
					n.Message = buf.String()
					buf.Reset()
				}
			}
			data.News = n
		}
	}

	s.refreshRecentPosts(db)

	fileName := "index.html"
	if s.opt.SiteIndex == IndexWriteToNav {
		fileName = "nav.html"
	}
	writePath := filepath.Join(s.config.Root, "_"+fileName)
	filePath := filepath.Join(s.config.Root, fileName)

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

	if s.opt.trace {
		traceD = time.Since(traceT)
		traceLog("/"+fileName, traceD)
		info.delta.Add(uint32(traceD.Milliseconds()))
	}
}

func (s *Server) _buildStatistics(info *buildInfo) {
	var (
		traceT time.Time
		traceD time.Duration
	)
	if s.opt.trace {
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

	if s.opt.trace {
		traceD = time.Since(traceT)
		traceLog("/stats.json", traceD)
		info.delta.Add(uint32(traceD.Milliseconds()))
	}
}

// _build handles the static page build queue.
func (s *Server) _build() {
	// Initialize write buffer.
	buf := bytes.NewBuffer(make([]byte, initialBufferSize))

	for {
		info := <-s.buildQueue
		db := s.beginReadOnly()
		info.db = db
		info.buf = buf

		// Handle build request.
		switch info.build {
		case buildBoardIndex:
			s._buildBoardIndex(info)
		case buildBoardCatalog:
			s._buildBoardCatalog(info)
		case buildBoardThread:
			s._buildBoardThread(info)
		case buildNewsIndex:
			s._buildNewsIndex(info)
		case buildNewsEntry:
			s._buildNewsEntry(info)
		case buildPage:
			s._buildPage(info)
		case buildSiteIndex:
			s._buildSiteIndex(info)
		case buildStatistics:
			s._buildStatistics(info)
		case queueBoardIndexes:
			s._queueBoardIndexes(info)
		}

		db.Commit()
		info.wg.Done()

		// Resize write buffer.
		if buf.Cap() > maxBufferSize {
			buf = bytes.NewBuffer(make([]byte, initialBufferSize))
		}
	}
}

// rebuildBoard rebuilds all pages in a board.
func (s *Server) rebuildBoard(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32, board *Board) {
	s.indexCacheLock.Lock()
	s.indexCache[board.ID] = nil               // Clear active page cache.
	s.indexCache[math.MaxInt32-board.ID] = nil // Clear archive page cache.
	s.indexCacheLock.Unlock()

	s.writeBoardIndexes(db, wg, delta, false, board)
	s.writeBoardIndexes(db, wg, delta, true, board)
	for _, threadInfo := range db.AllThreads(FilterVisible, board) {
		s.writeBoardThread(db, wg, delta, board, threadInfo[0])
	}
}

// writeBoardIndexes writes board index pages to disk.
func (s *Server) writeBoardIndexes(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32, archive bool, board *Board, overboards ...*Board) {
	info := &buildInfo{
		build:      queueBoardIndexes,
		board:      board,
		archive:    archive,
		overboards: overboards,
		wg:         wg,
		delta:      delta,
	}
	wg.Add(1)
	s.buildQueue <- info
}

// writeBoardThread writes a thread res page to disk.
func (s *Server) writeBoardThread(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32, board *Board, postID int) {
	if board.Unique == 0 {
		board.Unique = db.UniqueUserPosts(board)
		board.ArchiveSize = len(db.AllThreads(FilterArchived, board))
	}

	info := &buildInfo{
		build: buildBoardThread,
		board: board,
		post:  postID,
		wg:    wg,
		delta: delta,
	}
	wg.Add(1)
	s.buildQueue <- info
}

// writeOverboard writes overboard pages to disk.
func (s *Server) writeOverboard(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32, c *categoryInfo) {
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
	s.writeBoardIndexes(db, wg, delta, false, board, boards...)
}

// writeNewsEntry writes a news entry page to disk.
func (s *Server) writeNewsEntry(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32, n *News) {
	if s.opt.News == NewsDisable {
		return
	}

	wg.Add(1)
	info := &buildInfo{
		build: buildNewsEntry,
		news:  []*News{n},
		wg:    wg,
		delta: delta,
	}
	s.buildQueue <- info
}

// writeNewsIndexes writes news index pages to disk.
func (s *Server) writeNewsIndexes(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32) []*News {
	if s.opt.News == NewsDisable {
		return nil
	}

	allNews := db.AllNews(true)
	pages := pageCount(len(allNews), newsCount)
	wg.Add(pages)
	for page := 0; page < pages; page++ {
		info := &buildInfo{
			build: buildNewsIndex,
			news:  allNews,
			page:  page,
			wg:    wg,
			delta: delta,
		}
		s.buildQueue <- info
	}
	return allNews
}

// rebuildNewsEntry rebuilds a news entry.
func (s *Server) rebuildNewsEntry(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32, n *News) {
	s.writeNewsIndexes(db, wg, delta)
	s.writeNewsEntry(db, wg, delta, n)
}

// rebuildNews rebuilds all news entries.
func (s *Server) rebuildNews(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32) {
	allNews := s.writeNewsIndexes(db, wg, delta)
	for _, n := range allNews {
		s.writeNewsEntry(db, wg, delta, n)
	}
}

// writePages writes custom pages to disk.
func (s *Server) writePages(db serverDB, wg *sync.WaitGroup, delta *atomic.Uint32, pages []*Page) {
	wg.Add(len(pages))
	for _, p := range pages {
		info := &buildInfo{
			build:      buildPage,
			customPage: p,
			wg:         wg,
			delta:      delta,
		}
		s.buildQueue <- info
	}
}

// writeSiteIndex writes the site index page to disk.
func (s *Server) writeSiteIndex(wg *sync.WaitGroup, delta *atomic.Uint32) {
	if s.opt.SiteIndex == IndexDisable {
		return
	}

	wg.Add(1)
	info := &buildInfo{
		build: buildSiteIndex,
		wg:    wg,
		delta: delta,
	}
	s.buildQueue <- info
}

func (s *Server) writeStatistics(wg *sync.WaitGroup, delta *atomic.Uint32) {
	if !s.opt.Statistics {
		return
	}

	wg.Add(1)
	info := &buildInfo{
		build: buildStatistics,
		wg:    wg,
		delta: delta,
	}
	s.buildQueue <- info
}
