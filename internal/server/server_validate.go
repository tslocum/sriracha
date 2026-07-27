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
		if len(db.AllThreads(false, b)) > 0 {
			foundThread = true
			break
		}
	}
	db.Commit()
	if foundLog || foundThread {
		fatal(fmt.Errorf("an empty database is required"))
	} else if !emptyDir {
		fatal(fmt.Errorf("an empty root directory is required"))
	}

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
	u += "/sriracha/"

	getRequest := func(p string) ([]byte, error) {
		fmt.Printf("GET  %s\n", u+p)
		wrapErr := func(err error) error {
			return fmt.Errorf("failed to GET %s: %s", u, err)
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
		}
		return buf, nil
	}

	postRequest := func(p string, data url.Values) error {
		fmt.Printf("POST %s %+v\n", u+p, data)
		wrapErr := func(err error) error {
			return fmt.Errorf("failed to POST %s %+v: %s", u, data, err)
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
		}
		return nil
	}

	_, err = getRequest("")
	if err != nil {
		fatal(err)
	}

	err = postRequest("", nil)
	if err != nil {
		fatal(err)
	}

	// Verify managepent panel pages are forbidden when logged out.
	managePages := []string{"account", "ban", "banner", "board", "category", "keyword", "log", "news", "page", "preference", "setting", "threshold"}
	for _, managePage := range managePages {
		buf, err := getRequest(managePage)
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(`<input type="hidden" name="login" value="1">`)) {
			fatal(fmt.Errorf("accessed forbidden page %s without being logged in", managePage))
		}
	}

	// Verify incorrect passwords fail authentication.
	err = postRequest("", map[string][]string{
		"login":    {"1"},
		"username": {"admin"},
		"password": {"wrong"},
	})
	if err != nil {
		fatal(err)
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
	err = postRequest("", map[string][]string{
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
		buf, err := getRequest(managePage)
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
		err = postRequest("account", map[string][]string{
			"username": {username},
			"password": {password},
			"role":     {strconv.Itoa(int(RoleMod))},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("account")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(username)) {
			fatal(fmt.Errorf("failed to add account: username was not found in accounts table"))
		}

		buf, err = getRequest(fmt.Sprintf("account/%d", i+2))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(username)) {
			fatal(fmt.Errorf("failed to add account: username was not found in update page"))
		}
	}
	for i := 0; i < 3; i++ {
		username := fmt.Sprintf("newusername%d", i)
		password := fmt.Sprintf("newpassword%d", i)
		err = postRequest(fmt.Sprintf("account/%d", i+2), map[string][]string{
			"username": {username},
			"password": {password},
			"role":     {strconv.Itoa(int(RoleDisabled))},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("account")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(username)) {
			fatal(fmt.Errorf("failed to update account: username was not found in accounts table"))
		}

		buf, err = getRequest(fmt.Sprintf("account/%d", i+2))
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
		err = postRequest("ban", map[string][]string{
			"ip":     {ip},
			"reason": {reason},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("ban")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(reason)) {
			fatal(fmt.Errorf("failed to add ban: reason was not found in bans table"))
		}

		buf, err = getRequest(fmt.Sprintf("ban/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(reason)) {
			fatal(fmt.Errorf("failed to add ban: reason was not found in update page"))
		}
	}
	for i := 0; i < 3; i++ {
		reason := fmt.Sprintf("newreason%d", i)
		err = postRequest(fmt.Sprintf("ban/%d", i+1), map[string][]string{
			"reason": {reason},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("ban")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(reason)) {
			log.Println(string(buf))
			fatal(fmt.Errorf("failed to update ban: reason was not found in bans table"))
		}

		buf, err = getRequest(fmt.Sprintf("ban/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(reason)) {
			fatal(fmt.Errorf("failed to update ban: reason was not found in update page"))
		}

		err = postRequest(fmt.Sprintf("ban/lift/%d", i+1), map[string][]string{
			"reason": {"reason"},
		})
		if err != nil {
			fatal(err)
		}

		buf, err = getRequest("ban")
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
		err = postRequest("board", map[string][]string{
			"dir":         {dir},
			"name":        {name},
			"description": {description},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("board")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(dir)) {
			fatal(fmt.Errorf("failed to add board: board dir was not found in boards table"))
		} else if !bytes.Contains(buf, []byte(name)) {
			fatal(fmt.Errorf("failed to add board: board name was not found in boards table"))
		} else if !bytes.Contains(buf, []byte(description)) {
			fatal(fmt.Errorf("failed to add board: board description was not found in boards table"))
		}

		buf, err = getRequest(fmt.Sprintf("board/%d", i+1))
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
		err = postRequest(fmt.Sprintf("board/%d", i+1), map[string][]string{
			"dir":         {dir},
			"name":        {name},
			"description": {description},
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("board")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(dir)) {
			fatal(fmt.Errorf("failed to add board: updated board dir was not found in boards table"))
		} else if !bytes.Contains(buf, []byte(name)) {
			fatal(fmt.Errorf("failed to add board: updated board name was not found in boards table"))
		} else if !bytes.Contains(buf, []byte(description)) {
			fatal(fmt.Errorf("failed to add board: updated board description was not found in boards table"))
		}

		buf, err = getRequest(fmt.Sprintf("board/%d", i+1))
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
		err = postRequest("keyword", map[string][]string{
			"text":   {text},
			"action": {action},
			"boards": boards,
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("keyword")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to add keyword: text was not found in keywords table"))
		}

		buf, err = getRequest(fmt.Sprintf("keyword/%d", i+1))
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
		err = postRequest(fmt.Sprintf("keyword/%d", i+1), map[string][]string{
			"text":   {text},
			"action": {action},
			"boards": boards,
		})
		if err != nil {
			fatal(err)
		}

		buf, err := getRequest("keyword")
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to update keyword: text was not found in keywords table"))
		}

		buf, err = getRequest(fmt.Sprintf("keyword/%d", i+1))
		if err != nil {
			fatal(err)
		} else if !bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to update keyword: text was not found in update page"))
		}

		err = postRequest(fmt.Sprintf("keyword/delete/%d", i+1), nil)
		if err != nil {
			fatal(err)
		}

		buf, err = getRequest("keyword")
		if err != nil {
			fatal(err)
		} else if bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to delete keyword: text still present in keywords table"))
		}

		buf, err = getRequest(fmt.Sprintf("keyword/%d", i+1))
		if err != nil {
			fatal(err)
		} else if bytes.Contains(buf, []byte(text)) {
			fatal(fmt.Errorf("failed to delete keyword: update page is still accessible"))
		}
	}

	// Verify log out page.
	_, err = getRequest("logout")
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

	fmt.Println("All tests passed. Smoke test complete.")
	s.Stop()
}
