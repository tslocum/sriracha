package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	. "codeberg.org/tslocum/sriracha/model"
	"golang.org/x/net/publicsuffix"
)

func (s *Server) smokeTest(emptyDir bool) {
	fmt.Println("Running smoke test...")

	fatal := func(err error) {
		fmt.Println()
		fmt.Println(`  ⠀⠀⠐⠒⠴⡀⡄⡠⠀⠠⠀⡀⢀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠁⠁⠁⠁⠂⢚⠰⠷⠃⢒⣐⡄⠀⠀⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣠⣔⢴⡡⣅⢖⣊⣉⣁⠀⠀⠀⠀⠀⠀⠀⠀⠀
  ⠒⠎⠬⠌⡈⢀⠁⠈⠉⡉⠅⡭⠏⠊⠃⠋⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠠⠯⠥⠍⠖⣛⠋⣚⣓⣿⣎⣉⢉⡽⠀⠀⠀⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⣤⣔⡂⡈⣈⢉⠉⠉⢂⡶⠢⠯⣬⡍⠀⠀⠀⠀⠀⠀⠀⠀
  ⠀⢀⠀⠄⠠⠀⠀⠂⠀⢐⣀⣐⡳⠿⠽⠯⢗⡿⠛⠁⠀⠀⠀⠀⠀⠀⠀⠀
  ⠛⠒⠂⠄⠄⠄⠄⠠⠀⠄⠍⡉⡁⢂⢤⢮⣥⣤⣤⣤⣤⠀⠀⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⠤⠐⠐⠾⠯⠿⠀⠀⠀⠀⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⣉⣭⣬⣊⡷⡭⠿⠟⣀⡠⠤⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠐⠋⠛⠁⢡⣥⡓⣋⣡⡤⠶⠛⠛⠀⡀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠉⠑⠒⠐⣒⡾⠝⠋⠁
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⣔⣂⡁⠁⠁⢁⣀⡀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠒⠒⠒⠿⠽⠮⠗⠷⠽⡭⠭⠉⠉⠉⠉⠁⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣖⡊⣁⣁⣀⡄⠀⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠤⠮⢥⡵⠂⠀⠀⠀⠀⠀
  ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠷⠖⠉`)
		log.Fatal(err)
	}

	db := s.begin()
	foundLog := db.LogCount() > 0
	var foundThread bool
	for _, b := range db.AllBoards() {
		if len(db.AllThreads(FilterAny, b)) > 0 {
			foundThread = true
			break
		}
	}
	if foundLog || foundThread {
		fatal(fmt.Errorf("an empty database is required"))
	} else if !emptyDir {
		fatal(fmt.Errorf("an empty root directory is required"))
	}

	// Enable writing news to news.html
	s.opt.News = NewsWriteToNews
	db.SaveInt("news", int(s.opt.News))
	db.Commit()

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		fatal(fmt.Errorf("failed to initialize cookie jar: %s", err))
	}
	client := &http.Client{
		Jar: jar,
	}

	u := "http://"
	if s.config.HTTP == "" {
		u += "localhost"
	} else if strings.HasPrefix(s.config.HTTP, ":") {
		u += "localhost" + s.config.HTTP
	} else {
		u += s.config.HTTP
	}
	u += "/"

	getRequest := func(p string) ([]byte, error) {
		fmt.Printf("GET  /%s\n", p)
		wrapErr := func(err error) error {
			return fmt.Errorf("failed to GET %s: %s", u+p, err)
		}
		resp, err := client.Get(u + p)
		if err != nil {
			return nil, wrapErr(err)
		} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, wrapErr(fmt.Errorf("expected OK status gode, got %d", resp.StatusCode))
		}
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, wrapErr(fmt.Errorf("failed to read response body: %s", err))
		} else if len(buf) == 0 {
			return nil, wrapErr(fmt.Errorf("got empty response body"))
		} else if bytes.Contains(buf, []byte("Error:")) || bytes.Contains(buf, []byte("error:")) {
			return nil, wrapErr(fmt.Errorf("found error in page: %s", string(buf)))
		}
		return buf, nil
	}

	postRequest := func(p string, data url.Values) error {
		fmt.Printf("POST /%s %+v\n", p, data)
		wrapErr := func(err error) error {
			return fmt.Errorf("failed to POST %s %+v: %s", u+p, data, err)
		}
		resp, err := client.PostForm(u+p, data)
		if err != nil {
			return wrapErr(err)
		} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return wrapErr(fmt.Errorf("expected OK status gode, got %d", resp.StatusCode))
		}
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			return wrapErr(fmt.Errorf("failed to read response body: %s", err))
		} else if len(buf) == 0 {
			return wrapErr(fmt.Errorf("got empty response body"))
		} else if bytes.Contains(buf, []byte("Error:")) || bytes.Contains(buf, []byte("error:")) {
			return wrapErr(fmt.Errorf("found error in page: %s", string(buf)))
		}
		return nil
	}

	_, err = getRequest("sriracha")
	if err != nil {
		fatal(err)
	}

	err = postRequest("sriracha", nil)
	if err != nil {
		fatal(err)
	}

	// Verify managepent panel pages are forbidden when logged out.
	managePages := []string{"account", "ban", "banner", "board", "category", "keyword", "log", "news", "page", "preference", "setting", "threshold"}
	for _, managePage := range managePages {
		buf, err := getRequest("sriracha/" + managePage)
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(`<input type="hidden" name="login" value="1">`)) {
			fatal(fmt.Errorf("accessed forbidden page %s without being logged in", managePage))
		}
	}

	// Verify incorrect passwords fail authentication.
	err = postRequest("sriracha", map[string][]string{
		"login":    {"1"},
		"username": {"admin"},
		"password": {"wrong"},
	})
	if err == nil {
		fatal(fmt.Errorf("logged in with wrong username and password"))
	}
	pageURL, err := url.Parse(u)
	if err != nil {
		fatal(err)
	}
	numCookies := len(jar.Cookies(pageURL))
	if numCookies > 0 {
		fatal(fmt.Errorf("expected 0 cookies, got %d", numCookies))
	}

	// Verify correct passwords succeed authentication.
	err = postRequest("sriracha", map[string][]string{
		"login":    {"1"},
		"username": {"admin"},
		"password": {"admin"},
	})
	if err != nil {
		fatal(err)
	}
	numCookies = len(jar.Cookies(pageURL))
	if numCookies != 1 {
		fatal(fmt.Errorf("expected 1 cookie, got %d", numCookies))
	}

	// Verify management panel pages are accessible when logged in.
	for _, managePage := range managePages {
		buf, err := getRequest("sriracha/" + managePage)
		if err != nil {
			fatal(err)
		} else if bytes.Contains(buf, []byte(`<input type="hidden" name="login" value="1">`)) {
			fatal(fmt.Errorf("failed to access page %s while logged in", managePage))
		}
	}

	// Verify Accounts page.
	for i := 0; i < 3; i++ {
		username := fmt.Sprintf("username%d", i)
		password := fmt.Sprintf("password%d", i)
		err = postRequest("sriracha/account", map[string][]string{
			"username": {username},
			"password": {password},
			"role":     {strconv.Itoa(int(RoleMod))},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/account")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(username)) {
			fatal(fmt.Errorf("failed to add account: username was not found in accounts table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/account/%d", i+2))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(username)) {
			fatal(fmt.Errorf("failed to add account: username was not found in update page"))
		}
	}
	for i := 0; i < 3; i++ {
		username := fmt.Sprintf("newusername%d", i)
		password := fmt.Sprintf("newpassword%d", i)
		err = postRequest(fmt.Sprintf("sriracha/account/%d", i+2), map[string][]string{
			"username": {username},
			"password": {password},
			"role":     {strconv.Itoa(int(RoleDisabled))},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/account")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(username)) {
			fatal(fmt.Errorf("failed to update account: username was not found in accounts table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/account/%d", i+2))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(username)) {
			fatal(fmt.Errorf("failed to update account: username was not found in update page"))
		}
	}

	// Verify Bans page.
	for i := 0; i < 3; i++ {
		ip := fmt.Sprintf("192.0.2.%d", i)
		reason := fmt.Sprintf("reason%d", i)
		err = postRequest("sriracha/ban", map[string][]string{
			"ip":     {ip},
			"reason": {reason},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/ban")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(reason)) {
			fatal(fmt.Errorf("failed to add ban: reason was not found in bans table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/ban/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(reason)) {
			fatal(fmt.Errorf("failed to add ban: reason was not found in update page"))
		}
	}
	for i := 0; i < 3; i++ {
		reason := fmt.Sprintf("newreason%d", i)
		err = postRequest(fmt.Sprintf("sriracha/ban/%d", i+1), map[string][]string{
			"reason": {reason},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/ban")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(reason)) {
			fatal(fmt.Errorf("failed to update ban: reason was not found in bans table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/ban/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(reason)) {
			fatal(fmt.Errorf("failed to update ban: reason was not found in update page"))
		}

		err = postRequest(fmt.Sprintf("sriracha/ban/lift/%d", i+1), map[string][]string{
			"reason": {"reason"},
		})
		if err != nil {
			fatal(err)
		}

		buf, err = getRequest("sriracha/ban")
		if err != nil {
			fatal(err)
		} else if bytes.Contains(buf, []byte(reason)) {
			fatal(fmt.Errorf("failed to lift ban: reason still present in bans table"))
		}
	}

	// Verify Boards page.
	for i := 0; i < 3; i++ {
		dir := fmt.Sprintf("dir%d", i)
		name := fmt.Sprintf("name%d", i)
		description := fmt.Sprintf("description%d", i)
		err = postRequest("sriracha/board", map[string][]string{
			"dir":         {dir},
			"name":        {name},
			"description": {description},
			"style":       {s.config.Styles[0]},
			"approval":    {strconv.Itoa(int(ApprovalAll))},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/board")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(dir)) {
			fatal(fmt.Errorf("failed to add board: board dir was not found in boards table"))
		} else if !bytes.Contains(buf, []byte(name)) {
			fatal(fmt.Errorf("failed to add board: board name was not found in boards table"))
		} else if !bytes.Contains(buf, []byte(description)) {
			fatal(fmt.Errorf("failed to add board: board description was not found in boards table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/board/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(dir)) {
			fatal(fmt.Errorf("failed to add board: board dir was not found in update page"))
		} else if !bytes.Contains(buf, []byte(name)) {
			fatal(fmt.Errorf("failed to add board: board name was not found in update page"))
		} else if !bytes.Contains(buf, []byte(description)) {
			fatal(fmt.Errorf("failed to add board: board description was not found in update page"))
		}
	}
	for i := 0; i < 3; i++ {
		dir := fmt.Sprintf("newdir%d", i)
		name := fmt.Sprintf("newname%d", i)
		description := fmt.Sprintf("newdescription%d", i)
		err = postRequest(fmt.Sprintf("sriracha/board/%d", i+1), map[string][]string{
			"dir":         {dir},
			"name":        {name},
			"description": {description},
			"style":       {s.config.Styles[0] + "/flex"},
			"approval":    {strconv.Itoa(int(ApprovalNone))},
			"threads":     {strconv.Itoa(DefaultBoardThreads)},
			"replies":     {strconv.Itoa(DefaultBoardReplies)},
			"maxname":     {strconv.Itoa(DefaultBoardMaxName)},
			"maxemail":    {strconv.Itoa(DefaultBoardMaxEmail)},
			"maxsubject":  {strconv.Itoa(DefaultBoardMaxSubject)},
			"maxmessage":  {strconv.Itoa(DefaultBoardMaxMessage)},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/board")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(dir)) {
			fatal(fmt.Errorf("failed to add board: updated board dir was not found in boards table"))
		} else if !bytes.Contains(buf, []byte(name)) {
			fatal(fmt.Errorf("failed to add board: updated board name was not found in boards table"))
		} else if !bytes.Contains(buf, []byte(description)) {
			fatal(fmt.Errorf("failed to add board: updated board description was not found in boards table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/board/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(dir)) {
			fatal(fmt.Errorf("failed to add board: updated board dir was not found in update page"))
		} else if !bytes.Contains(buf, []byte(name)) {
			fatal(fmt.Errorf("failed to add board: updated board name was not found in update page"))
		} else if !bytes.Contains(buf, []byte(description)) {
			fatal(fmt.Errorf("failed to add board: updated board description was not found in update page"))
		}
	}

	// Verify Keywords page.
	for i := 0; i < 3; i++ {
		text := fmt.Sprintf("text%d", i)
		action := "hide"
		boards := []string{"1", "2", "3"}
		err = postRequest("sriracha/keyword", map[string][]string{
			"text":   {text},
			"action": {action},
			"boards": boards,
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/keyword")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to add keyword: text was not found in keywords table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/keyword/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to add keyword: text was not found in update page"))
		}
	}
	for i := 0; i < 3; i++ {
		text := fmt.Sprintf("text%d", i)
		action := "delete"
		boards := []string{"1"}
		err = postRequest(fmt.Sprintf("sriracha/keyword/%d", i+1), map[string][]string{
			"text":   {text},
			"action": {action},
			"boards": boards,
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/keyword")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to update keyword: text was not found in keywords table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/keyword/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to update keyword: text was not found in update page"))
		}

		err = postRequest(fmt.Sprintf("sriracha/keyword/delete/%d", i+1), nil)
		if err != nil {
			fatal(err)
		}

		buf, err = getRequest("sriracha/keyword")
		if err != nil {
			fatal(err)
		} else if bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to delete keyword: text still present in keywords table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/keyword/%d", i+1))
		if err == nil {
			fatal(fmt.Errorf("failed to delete keyword: update page is still accessible"))
		}
	}

	// Verify News page.
	for i := 0; i < 3; i++ {
		timestamp := "2026/01/01 00:00"
		name := fmt.Sprintf("name%d", i)
		subject := fmt.Sprintf("subject%d", i)
		message := fmt.Sprintf("message%d", i)
		share := "1"
		err = postRequest("sriracha/news", map[string][]string{
			"timestamp": {timestamp},
			"name":      {name},
			"subject":   {subject},
			"message":   {message},
			"share":     {share},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/news")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(subject)) {
			fatal(fmt.Errorf("failed to add news: subject was not found in news table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/news/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(subject)) {
			fatal(fmt.Errorf("failed to add news: subject was not found in update page"))
		}
	}
	for i := 0; i < 3; i++ {
		timestamp := "2026/06/01 00:00"
		name := fmt.Sprintf("newname%d", i)
		subject := fmt.Sprintf("newsubject%d", i)
		message := fmt.Sprintf("newmessage%d", i)
		share := "0"
		err = postRequest(fmt.Sprintf("sriracha/news/%d", i+1), map[string][]string{
			"timestamp": {timestamp},
			"name":      {name},
			"subject":   {subject},
			"message":   {message},
			"share":     {share},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/news")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(subject)) {
			fatal(fmt.Errorf("failed to update news: subject was not found in news table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/news/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(subject)) {
			fatal(fmt.Errorf("failed to update news: subject was not found in update page"))
		}

		err = postRequest(fmt.Sprintf("sriracha/news/delete/%d", i+1), nil)
		if err != nil {
			fatal(err)
		}

		buf, err = getRequest("sriracha/news")
		if err != nil {
			fatal(err)
		} else if bytes.Contains(buf, []byte(subject)) {
			fatal(fmt.Errorf("failed to delete news: subject still present in news table"))
		}

		_, err = getRequest(fmt.Sprintf("sriracha/news/%d", i+1))
		if err == nil {
			fatal(fmt.Errorf("failed to delete news: update page is still accessible"))
		}
	}

	// Verify Pages page.
	for i := 0; i < 3; i++ {
		pagePath := fmt.Sprintf("path%d", i)
		message := fmt.Sprintf("message%d", i)
		err = postRequest("sriracha/page", map[string][]string{
			"path":    {pagePath},
			"message": {message},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/page")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(pagePath)) {
			fatal(fmt.Errorf("failed to add page: path was not found in pages table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/page/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(pagePath)) {
			fatal(fmt.Errorf("failed to add page: path was not found in pages page"))
		}
	}
	for i := 0; i < 3; i++ {
		pagePath := fmt.Sprintf("newpath%d", i)
		message := fmt.Sprintf("newmessage%d", i)
		err = postRequest(fmt.Sprintf("sriracha/page/%d", i+1), map[string][]string{
			"path":    {pagePath},
			"message": {message},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("sriracha/page")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(pagePath)) {
			fatal(fmt.Errorf("failed to add page: path was not found in pages table"))
		}

		buf, err = getRequest(fmt.Sprintf("sriracha/page/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(pagePath)) {
			fatal(fmt.Errorf("failed to add page: path was not found in pages page"))
		}

		err = postRequest(fmt.Sprintf("sriracha/page/delete/%d", i+1), nil)
		if err != nil {
			fatal(err)
		}

		buf, err = getRequest("sriracha/page")
		if err != nil {
			fatal(err)
		} else if bytes.Contains(buf, []byte(pagePath)) {
			fatal(fmt.Errorf("failed to delete page: path still present in pages table"))
		}

		_, err = getRequest(fmt.Sprintf("sriracha/page/%d", i+1))
		if err == nil {
			fatal(fmt.Errorf("failed to delete page: update page is still accessible"))
		}
	}

	// Verify thread creation.
	const postDelay = 15 * time.Second
	boardDir := "newdir0"
	for j := 0; j < 2; j++ {
		for i := 0; i < 3; i++ {
			if j == 1 {
				time.Sleep(postDelay)
			}
			postName := fmt.Sprintf("threadname%d", i)
			postSubject := fmt.Sprintf("threadsubject%d", i)
			postMessage := fmt.Sprintf("threadmessage%d", i)
			err = postRequest("sriracha", map[string][]string{
				"action":   {"post"},
				"board":    {boardDir},
				"parent":   {"0"},
				"name":     {postName},
				"subject":  {postSubject},
				"message":  {postMessage},
				"password": {"password"},
			})
			if err != nil {
				fatal(err)
			}
			body, err := getRequest(boardDir + fmt.Sprintf("/res/%d.html", i+1))
			if err != nil {
				fatal(err)
			} else if !bytes.Contains(body, []byte(postName)) {
				fatal(fmt.Errorf("thread name was not found in res page"))
			} else if !bytes.Contains(body, []byte(postSubject)) {
				fatal(fmt.Errorf("thread subject was not found in res page"))
			} else if !bytes.Contains(body, []byte(postMessage)) {
				fatal(fmt.Errorf("thread message was not found in res page"))
			}
			body, err = getRequest(boardDir + "/")
			if err != nil {
				fatal(err)
			} else if !bytes.Contains(body, []byte(postName)) {
				fatal(fmt.Errorf("thread name was not found in index page"))
			} else if !bytes.Contains(body, []byte(postSubject)) {
				fatal(fmt.Errorf("thread subject was not found in index page"))
			} else if !bytes.Contains(body, []byte(postMessage)) {
				fatal(fmt.Errorf("thread message was not found in index page"))
			}
		}

		// Verify reply creation.
		for i := 0; i < 3; i++ {
			if j == 1 {
				time.Sleep(postDelay)
			}
			postName := fmt.Sprintf("replyname%d", i)
			postSubject := fmt.Sprintf("replysubject%d", i)
			postMessage := fmt.Sprintf("replymessage%d", i)
			err = postRequest("sriracha", map[string][]string{
				"action":  {"post"},
				"board":   {boardDir},
				"parent":  {"1"},
				"name":    {postName},
				"subject": {postSubject},
				"message": {postMessage},
			})
			if err != nil {
				fatal(err)
			}
			body, err := getRequest(boardDir + "/res/1.html")
			if err != nil {
				fatal(err)
			} else if !bytes.Contains(body, []byte(postName)) {
				fatal(fmt.Errorf("reply name was not found in res page"))
			} else if !bytes.Contains(body, []byte(postSubject)) {
				fatal(fmt.Errorf("reply subject was not found in res page"))
			} else if !bytes.Contains(body, []byte(postMessage)) {
				fatal(fmt.Errorf("reply message was not found in res page"))
			}
			body, err = getRequest(boardDir + "/")
			if err != nil {
				fatal(err)
			} else if !bytes.Contains(body, []byte(postName)) {
				fatal(fmt.Errorf("reply name was not found in index page"))
			} else if !bytes.Contains(body, []byte(postSubject)) {
				fatal(fmt.Errorf("reply subject was not found in index page"))
			} else if !bytes.Contains(body, []byte(postMessage)) {
				fatal(fmt.Errorf("reply message was not found in index page"))
			}
		}
	}

	// Verify log out page.
	_, err = getRequest("sriracha/logout")
	if err != nil {
		fatal(err)
	}
	var foundCookie bool
	for _, c := range jar.Cookies(pageURL) {
		if c.Value != "" {
			foundCookie = true
			break
		}
	}
	if foundCookie {
		fatal(fmt.Errorf("failed to log out: session cookie was not cleared"))
	}

	// Verify post deletion.
	err = postRequest("sriracha", map[string][]string{
		"action":       {"delete"},
		"board":        {boardDir},
		"delete[]":     {"1"},
		"confirmation": {"1"},
		"password":     {"wrong"},
	})
	if err == nil {
		fatal(fmt.Errorf("expected failure when deleting post with wrong password"))
	}

	deleteData := map[string][]string{
		"action":       {"delete"},
		"board":        {boardDir},
		"delete[]":     {"1"},
		"confirmation": {"1"},
		"password":     {"password"},
	}
	err = postRequest("sriracha", deleteData)
	if err != nil {
		fatal(err)
	}

	_, err = getRequest("sriracha/post/1")
	if err == nil {
		fatal(fmt.Errorf("expected failure when browsing a previously deleted post"))
	}

	fmt.Println("✔️ All tests passed. Smoke test complete.")
	s.Stop()
}
