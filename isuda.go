package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/Songmu/strrand"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/unrolled/render"
)

const (
	sessionName   = "isuda_session"
	sessionSecret = "tonymoris"
)

var (
	isutarEndpoint string
	isupamEndpoint string

	baseUrl *url.URL
	db      *sql.DB
	re      *render.Render
	store   *sessions.CookieStore

	errInvalidUser = errors.New("Invalid User")
)

func setName(c *gin.Context) error {
	session := getSession(c)
	userID, ok := session.Values["user_id"]
	if !ok {
		return nil
	}
	setContext(c.Request, "user_id", userID)
	row := db.QueryRow(`SELECT name FROM user WHERE id = ?`, userID)
	user := User{}
	err := row.Scan(&user.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return errInvalidUser
		}
		panicIf(err)
	}
	setContext(c.Request, "user_name", user.Name)
	return nil
}

func authenticate(c *gin.Context) error {
	if u := getContext(c, "user_id"); u != nil {
		return nil
	}
	return errInvalidUser
}

func initializedHandler(c *gin.Context) {
	_, err := db.Exec(`DELETE FROM entry WHERE id > 7101`)
	panicIf(err)

	resp, err := http.Get(fmt.Sprintf("%s/initialize", isutarEndpoint))
	panicIf(err)
	defer resp.Body.Close()

	c.JSON(200, gin.H{
		"result": "ok",
	})
	//re.JSON(w, http.StatusOK, map[string]string{"result": "ok"})
}

func topHandler(c *gin.Context) {
	if err := setName(c); err != nil {
		forbidden(c.Writer)
		return
	}

	perPage := 10
	p := c.Query("page")
	if p == "" {
		p = "1"
	}
	page, _ := strconv.Atoi(p)

	rows, err := db.Query(fmt.Sprintf(
		"SELECT * FROM entry ORDER BY updated_at DESC LIMIT %d OFFSET %d",
		perPage, perPage*(page-1),
	))
	if err != nil && err != sql.ErrNoRows {
		log.Printf("error rows")
		panicIf(err)
	}
	entries := make([]*Entry, 0, 10)
	for rows.Next() {
		e := Entry{}
		err := rows.Scan(&e.ID, &e.AuthorID, &e.Keyword, &e.Description, &e.UpdatedAt, &e.CreatedAt)
		panicIf(err)
		e.Html = htmlify(c, e.Description)
		e.Stars = loadStars(e.Keyword)
		entries = append(entries, &e)
	}
	rows.Close()

	var totalEntries int
	row := db.QueryRow(`SELECT COUNT(*) FROM entry`)
	err = row.Scan(&totalEntries)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("error rows2")
		panicIf(err)
	}

	lastPage := int(math.Ceil(float64(totalEntries) / float64(perPage)))
	pages := make([]int, 0, 10)
	start := int(math.Max(float64(1), float64(page-5)))
	end := int(math.Min(float64(lastPage), float64(page+5)))
	for i := start; i <= end; i++ {
		pages = append(pages, i)
	}

	c.HTML(http.StatusOK, "/views/index.tmpl", gin.H{
		"Context"  : c,
		"Entries"  : entries,
		"Page"     : page,
		"LastPage" : lastPage,
		"Pages"    : pages,
	})

	//re.HTML(w, http.StatusOK, "index", struct {
	//	Context  context.Context
	//	Entries  []*Entry
	//	Page     int
	//	LastPage int
	//	Pages    []int
	//}{
	//	r.Context(), entries, page, lastPage, pages,
	//})
}

func robotsHandler(c *gin.Context) {
	notFound(c.Writer)
}

func keywordPostHandler(c *gin.Context) {
	if err := setName(c); err != nil {
		forbidden(c.Writer)
		return
	}
	if err := authenticate(c); err != nil {
		forbidden(c.Writer)
		return
	}
	keyword := c.PostForm("keyword")
	if keyword == "" {
		badRequest(c.Writer)
		return
	}
	userID := getContext(c, "user_id").(int)
	description := c.PostForm("description")

	if isSpamContents(description) || isSpamContents(keyword) {
		//c.Error("SPAM!")
		http.Error(c.Writer, "SPAM!", http.StatusBadRequest)
		return
	}
	_, err := db.Exec(`
		INSERT INTO entry (author_id, keyword, description, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
		author_id = ?, keyword = ?, description = ?, updated_at = NOW()
	`, userID, keyword, description, userID, keyword, description)
	panicIf(err)
	c.Redirect(302, "/")
	//http.Redirect(w, r, "/", http.StatusFound)
}

func loginHandler(c *gin.Context) {
	if err := setName(c); err != nil {
		forbidden(c.Writer)
		return
	}
	c.HTML(200, "/views/authenticate.tmpl", gin.H{
		"Context" : c,
		"Action" : "login",
	})
	//re.HTML(w, http.StatusOK, "authenticate", struct {
	//	Context context.Context
	//	Action  string
	//}{
	//	r.Context(), "login",
	//})
}

func loginPostHandler(c *gin.Context) {
	name := c.Value("name")
	//r.FormValue("name")
	row := db.QueryRow(`SELECT * FROM user WHERE name = ?`, name)
	user := User{}
	err := row.Scan(&user.ID, &user.Name, &user.Salt, &user.Password, &user.CreatedAt)
	if err == sql.ErrNoRows || user.Password != fmt.Sprintf("%x", sha1.Sum([]byte(user.Salt+c.PostForm("password")))){
		forbidden(c.Writer)
		return
	}
	panicIf(err)
	session := getSession(c)
	session.Values["user_id"] = user.ID
	session.Save(c.Request, c.Writer)
	c.Redirect(302, "/")
	//http.Redirect(w, r, "/", http.StatusFound)
}

func logoutHandler(c *gin.Context) {
	session := getSession(c)
	session.Options = &sessions.Options{MaxAge: -1}
	session.Save(c.Request, c.Writer)
	c.Redirect(302, "/")
	//http.Redirect(w, r, "/", http.StatusFound)
}

func registerHandler(c *gin.Context) {
	if err := setName(c); err != nil {
		forbidden(c.Writer)
		return
	}

	c.HTML(200, "/views/authenticate.tmpl", gin.H{
		"Context" : c,
		"Action"  : "register",
	})
	//re.HTML(w, http.StatusOK, "authenticate", struct {
	//	Context context.Context
	//	Action  string
	//}{
	//	r.Context(), "register",
	//})
}

func registerPostHandler(c *gin.Context) {
	name := c.PostForm("name")
	pw := c.PostForm("password")
	if name == "" || pw == "" {
		badRequest(c.Writer)
		return
	}
	userID := register(name, pw)
	session := getSession(c)
	session.Values["user_id"] = userID
	session.Save(c.Request, c.Writer)
	c.Redirect(302, "/")
	//http.Redirect(w, r, "/", http.StatusFound)
}

func register(user string, pass string) int64 {
	salt, err := strrand.RandomString(`....................`)
	panicIf(err)
	res, err := db.Exec(`INSERT INTO user (name, salt, password, created_at) VALUES (?, ?, ?, NOW())`,
		user, salt, fmt.Sprintf("%x", sha1.Sum([]byte(salt+pass))))
	panicIf(err)
	lastInsertID, _ := res.LastInsertId()
	return lastInsertID
}

func keywordByKeywordHandler(c *gin.Context) {
	if err := setName(c); err != nil {
		forbidden(c.Writer)
		return
	}

	keyword := c.Param("keyword")
	row := db.QueryRow(`SELECT * FROM entry WHERE keyword = ?`, keyword)
	e := Entry{}
	err := row.Scan(&e.ID, &e.AuthorID, &e.Keyword, &e.Description, &e.UpdatedAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		notFound(c.Writer)
		return
	}
	e.Html = htmlify(c, e.Description)
	e.Stars = loadStars(e.Keyword)

	c.HTML(200, "/views/widget/keyword.tmpl", gin.H{
		"Context" : c,
		"Entry"  : e,
	})
	//re.HTML(w, http.StatusOK, "keyword", struct {
	//	Context context.Context
	//	Entry   Entry
	//}{
	//	r.Context(), e,
	//})
}

func keywordByKeywordDeleteHandler(c *gin.Context) {
	if err := setName(c); err != nil {
		forbidden(c.Writer)
		return
	}
	if err := authenticate(c); err != nil {
		forbidden(c.Writer)
		return
	}

	keyword := c.Param("keyword")
	if keyword == "" {
		badRequest(c.Writer)
		return
	}
	if c.PostForm("delete") == "" {
		badRequest(c.Writer)
		return
	}
	row := db.QueryRow(`SELECT * FROM entry WHERE keyword = ?`, keyword)
	e := Entry{}
	err := row.Scan(&e.ID, &e.AuthorID, &e.Keyword, &e.Description, &e.UpdatedAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		notFound(c.Writer)
		return
	}
	_, err = db.Exec(`DELETE FROM entry WHERE keyword = ?`, keyword)
	panicIf(err)
	c.Redirect(302, "/")
	//http.Redirect(w, r, "/", http.StatusFound)
}

func htmlify(c *gin.Context, content string) string {
	if content == "" {
		return ""
	}
	rows, err := db.Query(`
		SELECT * FROM entry ORDER BY CHARACTER_LENGTH(keyword) DESC
	`)
	panicIf(err)
	entries := make([]*Entry, 0, 500)
	for rows.Next() {
		e := Entry{}
		err := rows.Scan(&e.ID, &e.AuthorID, &e.Keyword, &e.Description, &e.UpdatedAt, &e.CreatedAt)
		panicIf(err)
		entries = append(entries, &e)
	}
	rows.Close()

	keywords := make([]string, 0, 500)
	for _, entry := range entries {
		keywords = append(keywords, regexp.QuoteMeta(entry.Keyword))
	}
	re := regexp.MustCompile("("+strings.Join(keywords, "|")+")")
	kw2sha := make(map[string]string)
	content = re.ReplaceAllStringFunc(content, func(kw string) string {
		kw2sha[kw] = "isuda_" + fmt.Sprintf("%x", sha1.Sum([]byte(kw)))
		return kw2sha[kw]
	})
	content = html.EscapeString(content)
	for kw, hash := range kw2sha {
		u, err := c.Request.URL.Parse(baseUrl.String()+"/keyword/" + pathURIEscape(kw))
		panicIf(err)
		link := fmt.Sprintf("<a href=\"%s\">%s</a>", u, html.EscapeString(kw))
		content = strings.Replace(content, hash, link, -1)
	}
	return strings.Replace(content, "\n", "<br />\n", -1)
}

func loadStars(keyword string) []*Star {
	v := url.Values{}
	v.Set("keyword", keyword)
	resp, err := http.Get(fmt.Sprintf("%s/stars", isutarEndpoint) + "?" + v.Encode())
	panicIf(err)
	defer resp.Body.Close()

	var data struct {
		Result []*Star `json:result`
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	panicIf(err)
	return data.Result
}

func isSpamContents(content string) bool {
	v := url.Values{}
	v.Set("content", content)
	resp, err := http.PostForm(isupamEndpoint, v)
	panicIf(err)
	defer resp.Body.Close()

	var data struct {
		Valid bool `json:valid`
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	panicIf(err)
	return !data.Valid
}

func getContext(c *gin.Context, key interface{}) interface{} {
	return c.Value(key)
}

func setContext(r *http.Request, key, val interface{}) {
	if val == nil {
		return
	}

	r2 := r.WithContext(context.WithValue(r.Context(), key, val))
	*r = *r2
}

func getSession(c *gin.Context) *sessions.Session {
	session, _ := store.Get(c.Request, sessionName)
	return session
}

func main() {
	host := os.Getenv("ISUDA_DB_HOST")
	if host == "" {
		host = "localhost"
	}
	portstr := os.Getenv("ISUDA_DB_PORT")
	if portstr == "" {
		portstr = "3306"
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		log.Fatalf("Failed to read DB port number from an environment variable ISUDA_DB_PORT.\nError: %s", err.Error())
	}
	user := os.Getenv("ISUDA_DB_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("ISUDA_DB_PASSWORD")
	dbname := os.Getenv("ISUDA_DB_NAME")
	if dbname == "" {
		dbname = "isuda"
	}

	db, err = sql.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?loc=Local&parseTime=true",
		user, password, host, port, dbname,
	))
	if err != nil {
		log.Fatalf("Failed to connect to DB: %s.", err.Error())
	}
	db.Exec("SET SESSION sql_mode='TRADITIONAL,NO_AUTO_VALUE_ON_ZERO,ONLY_FULL_GROUP_BY'")
	db.Exec("SET NAMES utf8mb4")

	isutarEndpoint = os.Getenv("ISUTAR_ORIGIN")
	if isutarEndpoint == "" {
		isutarEndpoint = "http://localhost:5001"
	}
	isupamEndpoint = os.Getenv("ISUPAM_ORIGIN")
	if isupamEndpoint == "" {
		isupamEndpoint = "http://localhost:5050"
	}

	store = sessions.NewCookieStore([]byte(sessionSecret))

	re = render.New(render.Options{
		Directory: "views",
		Funcs: []template.FuncMap{
			{
				"url_for": func(path string) string {
					return baseUrl.String() + path
				},
				"title": func(s string) string {
					return strings.Title(s)
				},
				"raw": func(text string) template.HTML {
					return template.HTML(text)
				},
				"add": func(a, b int) int { return a + b },
				"sub": func(a, b int) int { return a - b },
				"entry_with_ctx": func(entry Entry, ctx context.Context) *EntryWithCtx {
					return &EntryWithCtx{Context: ctx, Entry: entry}
				},
			},
		},
	})

	runtime.GOMAXPROCS(runtime.NumCPU())

	r := gin.New()

	//r.GET("/", getIndex)
	//r.GET("/initialize", getInitialize)
	//r.GET("/robots.txt", getRobots)
	//r.POST("/keyword", postKeyword)
	//
	//r.GET("/login", getLogin)
	//r.POST("/login", postLogin)
	//r.GET("/logout", getLogout)
	//
	//r.GET("/register", getRegister)
	//r.POST("/register", postRegister)
	//
	//r.GET("/keyword/:keyword", getKeyword)
	//r.GET("/keyword/:keyword", postKeyword)
	//r.Run(":80")
	r.GET("/", topHandler)
	r.GET("/initialize", initializedHandler)
	r.GET("/robots.txt", robotsHandler)
	r.POST("/keyword", keywordPostHandler)

	r.GET("/login", loginHandler)
	r.POST("/login", loginPostHandler)
	r.GET("/logout", logoutHandler)

	r.GET("/register", registerHandler)
	r.POST("/register", registerPostHandler)

	r.GET("/keyword/:keyword", keywordByKeywordHandler)
	r.GET("/keyword/:keyword", keywordByKeywordDeleteHandler)
	r.Static("/", "./public/")
	r.Run(":5000")

	//r := mux.NewRouter()
	//r.HandleFunc("/", myHandler(topHandler))
	//r.HandleFunc("/initialize", myHandler(initializeHandler)).Methods("GET")
	//r.HandleFunc("/robots.txt", myHandler(robotsHandler))
	//r.HandleFunc("/keyword", myHandler(keywordPostHandler)).Methods("POST")
	//
	//l := r.PathPrefix("/login").Subrouter()
	//l.Methods("GET").HandlerFunc(myHandler(loginHandler))
	//l.Methods("POST").HandlerFunc(myHandler(loginPostHandler))
	//r.HandleFunc("/logout", myHandler(logoutHandler))
	//
	//g := r.PathPrefix("/register").Subrouter()
	//g.Methods("GET").HandlerFunc(myHandler(registerHandler))
	//g.Methods("POST").HandlerFunc(myHandler(registerPostHandler))
	//
	//k := r.PathPrefix("/keyword/{keyword}").Subrouter()
	//k.Methods("GET").HandlerFunc(myHandler(keywordByKeywordHandler))
	//k.Methods("POST").HandlerFunc(myHandler(keywordByKeywordDeleteHandler))
	//
	//r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))
	//log.Fatal(http.ListenAndServe(":5000", r))
}
