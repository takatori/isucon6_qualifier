package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/unrolled/render"
)

var (
	baseUrl *url.URL
	db      *sql.DB
	re      *render.Render
)

func initializeHandler(c *gin.Context) {
	_, err := db.Exec("TRUNCATE star")
	panicIf(err)
	c.JSON(200, gin.H{
		"result" : "ok",
	})
	//re.JSON(w, http.StatusOK, map[string]string{"result": "ok"})
}

func starsHandler(c *gin.Context) {
	keyword := c.Param("keyword")
	rows, err := db.Query(`SELECT * FROM star WHERE keyword = ?`, keyword)
	if err != nil && err != sql.ErrNoRows {
		panicIf(err)
		return
	}

	stars := make([]Star, 0, 10)
	for rows.Next() {
		s := Star{}
		err := rows.Scan(&s.ID, &s.Keyword, &s.UserName, &s.CreatedAt)
		panicIf(err)
		stars = append(stars, s)
	}
	rows.Close()

	c.JSON(200, gin.H{
		"result" : stars,
	})
	//re.JSON(w, http.StatusOK, map[string][]Star{
	//	"result": stars,
	//})
}

func starsPostHandler(c *gin.Context) {
	keyword := c.Query("keyword")

	origin := os.Getenv("ISUDA_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5000"
	}
	u, err := c.Request.URL.Parse(fmt.Sprintf("%s/keyword/%s", origin, pathURIEscape(keyword)))
	panicIf(err)
	resp, err := http.Get(u.String())
	panicIf(err)
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		notFound(c.Writer)
		return
	}

	user := c.Query("user")
	_, err = db.Exec(`INSERT INTO star (keyword, user_name, created_at) VALUES (?, ?, NOW())`, keyword, user)
	panicIf(err)

	c.JSON(200, gin.H{
		"result" : "ok",
	})
	//re.JSON(w, http.StatusOK, map[string][]Star{
	//	"result": "ok",
	//})
	//re.JSON(w, http.StatusOK, map[string]string{"result": "ok"})
}

func main() {
	host := os.Getenv("ISUTAR_DB_HOST")
	if host == "" {
		host = "localhost"
	}
	portstr := os.Getenv("ISUTAR_DB_PORT")
	if portstr == "" {
		portstr = "3306"
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		log.Fatalf("Failed to read DB port number from an environment variable ISUTAR_DB_PORT.\nError: %s", err.Error())
	}
	user := os.Getenv("ISUTAR_DB_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("ISUTAR_DB_PASSWORD")
	dbname := os.Getenv("ISUTAR_DB_NAME")
	if dbname == "" {
		dbname = "isutar"
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

	re = render.New(render.Options{Directory: "dummy"})

	r := gin.New()
	r.GET("/initialize", initializeHandler)
	r.GET("/stars", starsHandler)
	r.POST("/stars", starsPostHandler)
	r.Run(":5000")

	//r := mux.NewRouter()
	//r.HandleFunc("/initialize", myHandler(initializeHandler))
	//s := r.PathPrefix("/stars").Subrouter()
	//s.Methods("GET").HandlerFunc(myHandler(starsHandler))
	//s.Methods("POST").HandlerFunc(myHandler(starsPostHandler))
	//
	//log.Fatal(http.ListenAndServe(":5001", r))
}
