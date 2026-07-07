package server

import (
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadGlobalBoardSettings(db serverDB, b *Board) {
	var haveGlobal bool
	for _, setting := range s.opt.Global {
		if strings.HasPrefix(setting, "board.") {
			haveGlobal = true
			break
		}
	}
	if !haveGlobal {
		return
	}

	var first *Board
	allBoards := db.AllBoards()
	if len(allBoards) > 0 {
		first = allBoards[0]
	}

	if slices.Contains(s.opt.Global, "board.type") {
		if first != nil {
			b.Type = first.Type
		} else {
			b.Type = TypeImageboard
		}
	}
	if slices.Contains(s.opt.Global, "board.hide") {
		if first != nil {
			b.Hide = first.Hide
		} else {
			b.Hide = HideNowhere
		}
	}
	if slices.Contains(s.opt.Global, "board.locale") {
		if first != nil {
			b.Locale = first.Locale
		} else {
			b.Locale = ""
		}
	}
	if slices.Contains(s.opt.Global, "board.style") {
		if first != nil {
			b.Style = first.Style
		} else {
			b.Style = ""
		}
	}
	if slices.Contains(s.opt.Global, "board.identifiers") {
		if first != nil {
			b.Identifiers = first.Identifiers
		} else {
			b.Identifiers = IdentifiersDisable
		}
	}
	if slices.Contains(s.opt.Global, "board.backlinks") {
		if first != nil {
			b.Backlinks = first.Backlinks
		} else {
			b.Backlinks = false
		}
	}
	if slices.Contains(s.opt.Global, "board.defaultname") {
		if first != nil {
			b.DefaultName = first.DefaultName
		} else {
			b.DefaultName = DefaultBoardDefaultName
		}
	}
	if slices.Contains(s.opt.Global, "board.threads") {
		if first != nil {
			b.Threads = first.Threads
		} else {
			b.Threads = DefaultBoardThreads
		}
	}
	if slices.Contains(s.opt.Global, "board.replies") {
		if first != nil {
			b.Replies = first.Replies
		} else {
			b.Replies = DefaultBoardReplies
		}
	}
	if slices.Contains(s.opt.Global, "board.truncate") {
		if first != nil {
			b.Truncate = first.Truncate
		} else {
			b.Truncate = DefaultBoardTruncate
		}
	}
	if slices.Contains(s.opt.Global, "board.rules") {
		if first != nil {
			b.Rules = first.Rules
		} else {
			b.Rules = nil
		}
	}
	if slices.Contains(s.opt.Global, "board.lock") {
		if first != nil {
			b.Lock = first.Lock
		} else {
			b.Lock = LockNone
		}
	}
	if slices.Contains(s.opt.Global, "board.approval") {
		if first != nil {
			b.Approval = first.Approval
		} else {
			b.Approval = ApprovalNone
		}
	}
	if slices.Contains(s.opt.Global, "board.reports") {
		if first != nil {
			b.Reports = first.Reports
		} else {
			b.Reports = false
		}
	}
	if slices.Contains(s.opt.Global, "board.maxthreads") {
		if first != nil {
			b.MaxThreads = first.MaxThreads
		} else {
			b.MaxThreads = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.maxreplies") {
		if first != nil {
			b.MaxReplies = first.MaxReplies
		} else {
			b.MaxReplies = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.minname") {
		if first != nil {
			b.MinName = first.MinName
		} else {
			b.MinName = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.maxname") {
		if first != nil {
			b.MaxName = first.MaxName
		} else {
			b.MaxName = DefaultBoardMaxName
		}
	}
	if slices.Contains(s.opt.Global, "board.minemail") {
		if first != nil {
			b.MinEmail = first.MinEmail
		} else {
			b.MinEmail = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.maxemail") {
		if first != nil {
			b.MaxEmail = first.MaxEmail
		} else {
			b.MaxEmail = DefaultBoardMaxEmail
		}
	}
	if slices.Contains(s.opt.Global, "board.minsubject") {
		if first != nil {
			b.MinSubject = first.MinSubject
		} else {
			b.MinSubject = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.maxsubject") {
		if first != nil {
			b.MaxSubject = first.MaxSubject
		} else {
			b.MaxSubject = DefaultBoardMaxSubject
		}
	}
	if slices.Contains(s.opt.Global, "board.minmessage") {
		if first != nil {
			b.MinMessage = first.MinMessage
		} else {
			b.MinMessage = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.maxmessage") {
		if first != nil {
			b.MaxMessage = first.MaxMessage
		} else {
			b.MaxMessage = DefaultBoardMaxMessage
		}
	}
	if slices.Contains(s.opt.Global, "board.wordbreak") {
		if first != nil {
			b.WordBreak = first.WordBreak
		} else {
			b.WordBreak = DefaultBoardWordBreak
		}
	}
	if slices.Contains(s.opt.Global, "board.delay") {
		if first != nil {
			b.Delay = first.Delay
		} else {
			b.Delay = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.files") {
		if first != nil {
			b.Files = first.Files
		} else {
			b.Files = DefaultBoardFiles
		}
	}
	if slices.Contains(s.opt.Global, "board.instances") {
		if first != nil {
			b.Instances = first.Instances
		} else {
			b.Instances = DefaultBoardInstances
		}
	}
	if slices.Contains(s.opt.Global, "board.minsizethread") {
		if first != nil {
			b.MinSizeThread = first.MinSizeThread
		} else {
			b.MinSizeThread = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.maxsizethread") {
		if first != nil {
			b.MaxSizeThread = first.MaxSizeThread
		} else {
			b.MaxSizeThread = DefaultBoardMaxSize
		}
	}
	if slices.Contains(s.opt.Global, "board.minsizereply") {
		if first != nil {
			b.MinSizeReply = first.MinSizeReply
		} else {
			b.MinSizeReply = 0
		}
	}
	if slices.Contains(s.opt.Global, "board.maxsizereply") {
		if first != nil {
			b.MaxSizeReply = first.MaxSizeReply
		} else {
			b.MaxSizeReply = DefaultBoardMaxSize
		}
	}
	if slices.Contains(s.opt.Global, "board.thumbwidth") {
		if first != nil {
			b.ThumbWidth = first.ThumbWidth
		} else {
			b.ThumbWidth = DefaultBoardThumbWidth
		}
	}
	if slices.Contains(s.opt.Global, "board.thumbheight") {
		if first != nil {
			b.ThumbHeight = first.ThumbHeight
		} else {
			b.ThumbHeight = DefaultBoardThumbHeight
		}
	}
	if slices.Contains(s.opt.Global, "board.uploads") {
		if first != nil {
			b.Uploads = first.Uploads
		} else {
			b.Uploads = nil
		}
	}
	if slices.Contains(s.opt.Global, "board.embeds") {
		if first != nil {
			b.Embeds = first.Embeds
		} else {
			b.Embeds = nil
		}
	}
	if slices.Contains(s.opt.Global, "board.oekaki") {
		if first != nil {
			b.Oekaki = first.Oekaki
		} else {
			b.Oekaki = false
		}
	}
	if slices.Contains(s.opt.Global, "board.gallery") {
		if first != nil {
			b.Gallery = first.Gallery
		} else {
			b.Gallery = DefaultBoardGallery
		}
	}
	if slices.Contains(s.opt.Global, "board.require") {
		if first != nil {
			b.Require = first.Require
		} else {
			b.Require = RequireNever
		}
	}
}

func (s *Server) loadBoardForm(db serverDB, r *http.Request, b *Board) {
	b.Dir = FormString(r, "dir")
	b.Name = FormString(r, "name")
	b.Description = FormString(r, "description")
	b.Type = FormRange(r, "type", TypeImageboard, TypeForum)
	b.Hide = FormRange(r, "hide", HideNowhere, HideEverywhere)
	b.Lock = FormRange(r, "lock", LockNone, LockStaff)
	b.Approval = FormRange(r, "approval", ApprovalNone, ApprovalAll)
	b.Reports = FormBool(r, "reports")
	b.Style = FormString(r, "style")
	b.Locale = FormString(r, "locale")
	b.Delay = FormInt(r, "delay")
	b.MinName = FormInt(r, "minname")
	b.MaxName = FormInt(r, "maxname")
	b.MinEmail = FormInt(r, "minemail")
	b.MaxEmail = FormInt(r, "maxemail")
	b.MinSubject = FormInt(r, "minsubject")
	b.MaxSubject = FormInt(r, "maxsubject")
	b.MinMessage = FormInt(r, "minmessage")
	b.MaxMessage = FormInt(r, "maxmessage")
	b.MinSizeThread = FormInt64(r, "minsizethread")
	b.MaxSizeThread = FormInt64(r, "maxsizethread")
	b.MinSizeReply = FormInt64(r, "minsizereply")
	b.MaxSizeReply = FormInt64(r, "maxsizereply")
	b.ThumbWidth = FormInt(r, "thumbwidth")
	b.ThumbHeight = FormInt(r, "thumbheight")
	b.DefaultName = FormString(r, "defaultname")
	b.WordBreak = FormInt(r, "wordbreak")
	b.Truncate = FormInt(r, "truncate")
	b.Threads = FormInt(r, "threads")
	b.Replies = FormInt(r, "replies")
	b.MaxThreads = FormInt(r, "maxthreads")
	b.MaxReplies = FormInt(r, "maxreplies")
	b.Oekaki = FormBool(r, "oekaki")
	b.Rules = FormMultiString(r, "rules")
	b.Backlinks = FormBool(r, "backlinks")
	b.Instances = FormNegInt(r, "instances")
	b.Identifiers = FormRange(r, "identifiers", IdentifiersDisable, IdentifiersGlobal)
	b.Files = FormInt(r, "files")
	b.Gallery = FormBool(r, "gallery")
	b.Require = FormRange(r, "require", RequireNever, RequireAll)

	if b.Locale != "" && !slices.Contains(s.opt.LocalesSorted, b.Locale) {
		b.Locale = ""
	}

	if b.Files < 0 {
		b.Files = 0
	}

	b.Uploads = nil
	uploads := r.Form["uploads"]
	availableUploads := s.config.UploadTypes()
	for _, upload := range uploads {
		var found bool
		for _, u := range availableUploads {
			if u.MIME == upload {
				found = true
				break
			}
		}
		if found {
			b.Uploads = append(b.Uploads, upload)
		}
	}

	b.Embeds = nil
	embeds := r.Form["embeds"]
	for _, embed := range embeds {
		var found bool
		for _, info := range s.opt.Embeds {
			if info[0] == embed {
				found = true
				break
			}
		}
		if found {
			b.Embeds = append(b.Embeds, embed)
		}
	}
}

func (s *Server) saveGlobalBoardSettings(db serverDB, b *Board) []*Board {
	var haveGlobal bool
	for _, setting := range s.opt.Global {
		if strings.HasPrefix(setting, "board.") {
			haveGlobal = true
			break
		}
	}
	if !haveGlobal {
		return nil
	}

	allBoards := db.AllBoards()
	var modified bool
	var modifiedBoards []*Board
	for _, board := range allBoards {
		if board.ID == b.ID {
			continue
		}
		modified = false
		if slices.Contains(s.opt.Global, "board.type") && board.Type != b.Type {
			board.Type = b.Type
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.hide") && board.Hide != b.Hide {
			board.Hide = b.Hide
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.locale") && board.Locale != b.Locale {
			board.Locale = b.Locale
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.style") && board.Style != b.Style {
			board.Style = b.Style
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.identifiers") && board.Identifiers != b.Identifiers {
			board.Identifiers = b.Identifiers
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.backlinks") && board.Backlinks != b.Backlinks {
			board.Backlinks = b.Backlinks
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.defaultname") && board.DefaultName != b.DefaultName {
			board.DefaultName = b.DefaultName
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.threads") && board.Threads != b.Threads {
			board.Threads = b.Threads
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.replies") && board.Replies != b.Replies {
			board.Replies = b.Replies
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.truncate") && board.Truncate != b.Truncate {
			board.Truncate = b.Truncate
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.rules") && !slices.Equal(board.Rules, b.Rules) {
			board.Rules = b.Rules
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.lock") && board.Lock != b.Lock {
			board.Lock = b.Lock
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.approval") && board.Approval != b.Approval {
			board.Approval = b.Approval
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.reports") && board.Reports != b.Reports {
			board.Reports = b.Reports
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.maxthreads") && board.MaxThreads != b.MaxThreads {
			board.MaxThreads = b.MaxThreads
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.maxreplies") && board.MaxReplies != b.MaxReplies {
			board.MaxReplies = b.MaxReplies
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.minname") && board.MinName != b.MinName {
			board.MinName = b.MinName
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.maxname") && board.MaxName != b.MaxName {
			board.MaxName = b.MaxName
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.minemail") && board.MinEmail != b.MinEmail {
			board.MinEmail = b.MinEmail
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.maxemail") && board.MaxEmail != b.MaxEmail {
			board.MaxEmail = b.MaxEmail
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.minsubject") && board.MinSubject != b.MinSubject {
			board.MinSubject = b.MinSubject
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.maxsubject") && board.MaxSubject != b.MaxSubject {
			board.MaxSubject = b.MaxSubject
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.minmessage") && board.MinMessage != b.MinMessage {
			board.MinMessage = b.MinMessage
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.maxmessage") && board.MaxMessage != b.MaxMessage {
			board.MaxMessage = b.MaxMessage
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.wordbreak") && board.WordBreak != b.WordBreak {
			board.WordBreak = b.WordBreak
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.delay") && board.Delay != b.Delay {
			board.Delay = b.Delay
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.files") && board.Files != b.Files {
			board.Files = b.Files
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.instances") && board.Instances != b.Instances {
			board.Instances = b.Instances
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.minsizethread") && board.MinSizeThread != b.MinSizeThread {
			board.MinSizeThread = b.MinSizeThread
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.maxsizethread") && board.MaxSizeThread != b.MaxSizeThread {
			board.MaxSizeThread = b.MaxSizeThread
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.minsizereply") && board.MinSizeReply != b.MinSizeReply {
			board.MinSizeReply = b.MinSizeReply
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.maxsizereply") && board.MaxSizeReply != b.MaxSizeReply {
			board.MaxSizeReply = b.MaxSizeReply
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.thumbwidth") && board.ThumbWidth != b.ThumbWidth {
			board.ThumbWidth = b.ThumbWidth
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.thumbheight") && board.ThumbHeight != b.ThumbHeight {
			board.ThumbHeight = b.ThumbHeight
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.uploads") && !slices.Equal(board.Uploads, b.Uploads) {
			board.Uploads = b.Uploads
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.embeds") && !slices.Equal(board.Embeds, b.Embeds) {
			board.Embeds = b.Embeds
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.oekaki") && board.Oekaki != b.Oekaki {
			board.Oekaki = b.Oekaki
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.gallery") && board.Gallery != b.Gallery {
			board.Gallery = b.Gallery
			modified = true
		}
		if slices.Contains(s.opt.Global, "board.require") && board.Require != b.Require {
			board.Require = b.Require
			modified = true
		}
		if !modified {
			continue
		}
		db.UpdateBoard(board)
		modifiedBoards = append(modifiedBoards, board)
	}
	return modifiedBoards
}

func (s *Server) serveBoard(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) (skipExecute bool) {
	data.Template = "manage_board"

	boardID := PathInt(r, "/sriracha/board/rebuild/")
	if boardID > 0 {
		if data.forbidden(w, RoleAdmin) {
			return false
		}
		b := db.BoardByID(boardID)
		if b == nil {
			data.ManageError("Board not found")
			return false
		}
		s.rebuildBoard(db, b)
		data.Info = fmt.Sprintf("Rebuilt %s", b.Path())
	}

	modBoard := PathString(r, "/sriracha/board/mod/")
	if modBoard != "" {
		var postID int
		var page int
		split := strings.Split(modBoard, "/")
		if len(split) == 2 {
			boardID, _ = strconv.Atoi(split[0])
			if strings.HasPrefix(split[1], "p") {
				page = ParseInt(split[1][1:])
			} else {
				postID = ParseInt(split[1])
			}
		} else if len(split) == 1 {
			boardID, _ = strconv.Atoi(split[0])
		}

		b := db.BoardByID(boardID)
		if b == nil {
			data.ManageError("Invalid or deleted board or post")
			return false
		}

		data.Template = "board_page"
		data.Board = b
		data.Boards = db.AllBoards()
		data.ModMode = true
		if postID > 0 {
			data.Threads = [][]*Post{db.AllPostsInThread(postID, true)}
			data.ReplyMode = postID
		} else {
			allThreads := db.AllThreads(b, true)

			data.Page = page
			data.Pages = pageCount(len(allThreads), b.Threads)

			start := page * b.Threads
			end := len(allThreads)
			if b.Threads != 0 && end > start+b.Threads {
				end = start + b.Threads
			}
			for _, threadInfo := range allThreads[start:end] {
				thread := db.PostByID(threadInfo[0])
				thread.Replies = threadInfo[1]
				posts := []*Post{thread}
				if b.Type == TypeImageboard {
					posts = append(posts, db.AllReplies(threadInfo[0], b.Replies, true)...)
				}
				data.Threads = append(data.Threads, posts)
			}
		}
		if len(data.Threads) == 0 || len(data.Threads[0]) == 0 {
			data.ManageError("Invalid or deleted post")
			return false
		}
		return false
	}

	resetBoardID := PathInt(r, "/sriracha/board/reset/")
	if resetBoardID > 0 {
		if s.forbidden(w, data, "board.update") {
			return
		}

		b := db.BoardByID(resetBoardID)
		if b == nil {
			data.ManageError("Invalid board.")
			return
		}

		bb := NewBoard()
		bb.ID = b.ID
		bb.Dir = b.Dir
		bb.Name = b.Name
		bb.Description = b.Description
		db.UpdateBoard(bb)
		updated := s.saveGlobalBoardSettings(db, bb)

		s.refreshMaxRequestSize(db)
		s.refreshBannerCache(db)
		s.refreshRulesCache(db)
		s.refreshCategoryCache(db)
		s.refreshKeywordCache(db)
		s.rebuildBoard(db, bb)
		for _, board := range updated {
			s.rebuildBoard(db, board)
		}
		s.writeSiteIndex(db)

		changes := printChanges(*b, *bb)
		s.log(db, data.Account, nil, fmt.Sprintf("Reset >>/board/%d", bb.ID), changes)

		data.Redirect(w, r, fmt.Sprintf("/sriracha/board/%d", bb.ID))
		return
	}

	deleteBoardID := PathInt(r, "/sriracha/board/delete/")
	if deleteBoardID > 0 {
		if s.forbidden(w, data, "board.delete") {
			return
		}

		b := db.BoardByID(deleteBoardID)
		if b == nil {
			data.ManageError("Invalid board.")
			return
		}

		allThreads := db.AllThreads(b, false)
		if !FormBool(r, "confirmation") {
			data.Template = "manage_info"
			data.Message = template.HTML(`<h2 class="managetitle">` + Get(b, data.Account, "Boards") + `</h2>
			[<a href="/sriracha/board/">` + Get(b, data.Account, "Return") + `</a>]<br>
			<form method="post">
			<input type="hidden" name="confirmation" value="1">
			<fieldset>
				<legend>
					Delete ` + b.Path() + ` ` + html.EscapeString(b.Name) + `
				</legend>
				<div>
					<h1 style="margin: 0;">WARNING!</h1>
					You are about to <b>PERMANENTLY DELETE <a href="` + b.Path() + `">` + b.Path() + ` ` + html.EscapeString(b.Name) + `</a>!</b><br>
					` + strconv.Itoa(len(allThreads)) + ` threads in ` + b.Path() + ` will be <b>PERMANENTLY DELETED!</b><br>
					<b>THIS OPERATION CANNOT BE UNDONE!</b><br><br>
					Type <b>` + b.Path() + `</b> to confirm:
					<input type="text" name="path"> <input type="submit" value="Delete ` + b.Path() + ` forever" style="margin-top: 5px;">
				</div>
			</fieldset>
			</form><br>`)
			return
		} else if FormString(r, "path") != b.Path() {
			data.ManageError(fmt.Sprintf("Type the board path %s to confirm deletion.", b.Path()))
			return
		}
		for _, threadInfo := range allThreads {
			s.deletePost(db, db.PostByID(threadInfo[0]))
		}
		db.DeleteBoard(b.ID)

		if b.Dir != "" {
			var skipDeleteDir bool
			boardPath := filepath.Join(s.config.Root, b.Dir)
			pattern := regexp.MustCompile(`^(index|catalog|[0-9]+).html$`)
			filepath.WalkDir(boardPath, func(path string, d fs.DirEntry, err error) error {
				if !d.IsDir() && !pattern.MatchString(d.Name()) && err == nil {
					skipDeleteDir = true
					return filepath.SkipAll
				}
				return nil
			})
			if !skipDeleteDir {
				os.RemoveAll(boardPath)
			}
		}

		s.refreshMaxRequestSize(db)
		s.refreshBannerCache(db)
		s.refreshRulesCache(db)
		s.refreshCategoryCache(db)
		s.refreshKeywordCache(db)
		s.writeSiteIndex(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted board #%d", b.ID), "")

		data.Template = "manage_info"
		data.Redirect(w, r, "/sriracha/board/")
		return
	}

	boardID = PathInt(r, "/sriracha/board/")
	if boardID > 0 {
		data.Manage.Board = db.BoardByID(boardID)
		if data.Manage.Board == nil {
			data.ManageError("Board not found")
			return false
		}

		if data.Manage.Board != nil && r.Method == http.MethodPost {
			if s.forbidden(w, data, "board.update") {
				return false
			}
			oldBoard := *data.Manage.Board

			oldDir := data.Manage.Board.Dir
			oldPath := data.Manage.Board.Path()
			s.loadBoardForm(db, r, data.Manage.Board)

			err := data.Manage.Board.Validate()
			if err != nil {
				data.ManageError(err.Error())
				return false
			}

			if data.Manage.Board.Dir != "" && data.Manage.Board.Dir != oldDir {
				_, err := os.Stat(filepath.Join(s.config.Root, data.Manage.Board.Dir))
				if err != nil {
					if !os.IsNotExist(err) {
						log.Fatal(err)
					}
				} else {
					data.ManageError("New directory already exists")
					return false
				}
			}

			db.UpdateBoard(data.Manage.Board)
			updated := s.saveGlobalBoardSettings(db, data.Manage.Board)

			if data.Manage.Board.Dir != oldDir {
				subDirs := []string{"src", "thumb", "res"}
				for _, subDir := range subDirs {
					newPath := filepath.Join(s.config.Root, data.Manage.Board.Dir, subDir)
					_, err := os.Stat(newPath)
					if err == nil {
						data.ManageError(fmt.Sprintf("New board directory %s already exists", newPath))
						return false
					}
				}
				moveSubDirs := func() error {
					for _, subDir := range subDirs {
						oldPath := filepath.Join(s.config.Root, oldDir, subDir)
						newPath := filepath.Join(s.config.Root, data.Manage.Board.Dir, subDir)
						err := os.Rename(oldPath, newPath)
						if err != nil {
							return fmt.Errorf("Failed to rename board directory %s to %s: %s", oldPath, newPath, err)
						}
					}
					return nil
				}
				if data.Manage.Board.Dir == "" {
					err = moveSubDirs()
					if err != nil {
						data.ManageError(err.Error())
						return false
					}
				} else {
					if oldDir == "" {
						err := os.Mkdir(filepath.Join(s.config.Root, data.Manage.Board.Dir), NewDirPermission)
						if err != nil {
							data.ManageError(fmt.Sprintf("Failed to create board directory: %s", err))
							return false
						}
						err = moveSubDirs()
						if err != nil {
							data.ManageError(err.Error())
							return false
						}
					} else {
						err := os.Rename(filepath.Join(s.config.Root, oldDir), filepath.Join(s.config.Root, data.Manage.Board.Dir))
						if err != nil {
							data.ManageError(fmt.Sprintf("Failed to rename board directory: %s", err))
							return false
						}
					}
				}

				for _, info := range db.AllThreads(data.Manage.Board, false) {
					for _, post := range db.AllPostsInThread(info[0], false) {
						var modified bool
						resPattern, err := regexp.Compile(`<a href="` + regexp.QuoteMeta(oldPath) + `res\/([0-9]+).html#([0-9]+)"`)
						if err != nil {
							log.Fatalf("failed to compile res pattern: %s", err)
						}
						post.Message = resPattern.ReplaceAllStringFunc(post.Message, func(s string) string {
							modified = true
							match := resPattern.FindStringSubmatch(s)
							return fmt.Sprintf(`<a href="%sres/%s.html#%s"`, data.Manage.Board.Path(), match[1], match[2])
						})
						if modified {
							db.UpdatePostMessage(post.ID, post.Message)
						}
					}
				}
			}

			s.refreshMaxRequestSize(db)
			s.refreshBannerCache(db)
			s.refreshRulesCache(db)
			s.refreshCategoryCache(db)
			s.refreshKeywordCache(db)
			s.rebuildBoard(db, data.Manage.Board)
			for _, board := range updated {
				s.rebuildBoard(db, board)
			}
			s.writeSiteIndex(db)

			changes := printChanges(oldBoard, *data.Manage.Board)
			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/board/%d", data.Manage.Board.ID), changes)

			data.Redirect(w, r, "/sriracha/board/")
			return true
		}
		return false
	}

	if r.Method == http.MethodPost {
		if s.forbidden(w, data, "board.add") {
			return
		}
		b := &Board{}
		s.loadBoardForm(db, r, b)
		s.loadGlobalBoardSettings(db, b)

		if FormBool(r, "duplicate") {
			duplicateID := FormInt(r, "board")
			d := db.BoardByID(duplicateID)
			if d == nil {
				data.ManageError("Board not found")
				return false
			}
			b.Type = d.Type
			b.Hide = d.Hide
			b.Lock = d.Lock
			b.Approval = d.Approval
			b.Reports = d.Reports
			b.Style = d.Style
			b.Locale = d.Locale
			b.Delay = d.Delay
			b.MinName = d.MinName
			b.MaxName = d.MaxName
			b.MinEmail = d.MinEmail
			b.MaxEmail = d.MaxEmail
			b.MinSubject = d.MinSubject
			b.MaxSubject = d.MaxSubject
			b.MinMessage = d.MinMessage
			b.MaxMessage = d.MaxMessage
			b.MinSizeThread = d.MinSizeThread
			b.MaxSizeThread = d.MaxSizeThread
			b.MinSizeReply = d.MinSizeReply
			b.MaxSizeReply = d.MaxSizeReply
			b.ThumbWidth = d.ThumbWidth
			b.ThumbHeight = d.ThumbHeight
			b.DefaultName = d.DefaultName
			b.WordBreak = d.WordBreak
			b.Truncate = d.Truncate
			b.Threads = d.Threads
			b.Replies = d.Replies
			b.MaxThreads = d.MaxThreads
			b.MaxReplies = d.MaxReplies
			b.Oekaki = d.Oekaki
			b.Backlinks = d.Backlinks
			b.Files = d.Files
			b.Instances = d.Instances
			b.Identifiers = d.Identifiers
			b.Gallery = d.Gallery
			b.Require = d.Require
			b.Uploads = d.Uploads
			b.Embeds = d.Embeds
			b.Rules = d.Rules
		}

		err := b.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return false
		}

		dirs := []string{"", "src", "thumb", "res"}
		for _, boardDir := range dirs {
			if b.Dir == "" && boardDir == "" {
				continue
			}
			boardPath := filepath.Join(s.config.Root, b.Dir, boardDir)
			err = os.Mkdir(boardPath, NewDirPermission)
			if err != nil {
				if os.IsExist(err) {
					data.ManageError(fmt.Sprintf("Board directory %s already exists.", boardPath))
				} else {
					data.ManageError(fmt.Sprintf("Failed to create board directory %s: %s", boardPath, err))
				}
				return false
			}
		}

		db.AddBoard(b)

		s.refreshMaxRequestSize(db)
		s.refreshBannerCache(db)
		s.refreshRulesCache(db)
		s.refreshCategoryCache(db)
		s.refreshKeywordCache(db)
		s.rebuildBoard(db, b)
		s.writeSiteIndex(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/board/%d", b.ID), "")

		data.Redirect(w, r, "/sriracha/board/")
		return true
	}

	data.Manage.Board = NewBoard()

	data.Manage.Boards = db.AllBoards()
	return false
}
