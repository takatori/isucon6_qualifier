//package main
//
//import (
//	"database/sql"
//	"fmt"
//	"log"
//	"net/http"
//	"net/url"
//	"os"
//	"strconv"
//
//	"github.com/gin-gonic/gin"
//	_ "github.com/go-sql-driver/mysql"
//	"github.com/unrolled/render"
//)
//
//
//func initializeHandler(c *gin.Context) {
//	_, err := db.Exec("TRUNCATE star")
//	panicIf(err)
//	c.JSON(200, gin.H{
//		"result" : "ok",
//	})
//	//re.JSON(w, http.StatusOK, map[string]string{"result": "ok"})
//}
//
//func starsHandler(c *gin.Context) {
//	keyword := c.Param("keyword")
//	rows, err := db.Query(`SELECT * FROM star WHERE keyword = ?`, keyword)
//	if err != nil && err != sql.ErrNoRows {
//		panicIf(err)
//		return
//	}
//
//	stars := make([]Star, 0, 10)
//	for rows.Next() {
//		s := Star{}
//		err := rows.Scan(&s.ID, &s.Keyword, &s.UserName, &s.CreatedAt)
//		panicIf(err)
//		stars = append(stars, s)
//	}
//	rows.Close()
//
//	c.JSON(200, gin.H{
//		"result" : stars,
//	})
//	//re.JSON(w, http.StatusOK, map[string][]Star{
//	//	"result": stars,
//	//})
//}
//
//func starsPostHandler(c *gin.Context) {
//	keyword := c.Query("keyword")
//
//	origin := os.Getenv("ISUDA_ORIGIN")
//	if origin == "" {
//		origin = "http://localhost:5000"
//	}
//	u, err := c.Request.URL.Parse(fmt.Sprintf("%s/keyword/%s", origin, pathURIEscape(keyword)))
//	panicIf(err)
//	resp, err := http.Get(u.String())
//	panicIf(err)
//	defer resp.Body.Close()
//	if resp.StatusCode >= 400 {
//		notFound(c.Writer)
//		return
//	}
//
//	user := c.Query("user")
//	_, err = db.Exec(`INSERT INTO star (keyword, user_name, created_at) VALUES (?, ?, NOW())`, keyword, user)
//	panicIf(err)
//
//	c.JSON(200, gin.H{
//		"result" : "ok",
//	})
//	//re.JSON(w, http.StatusOK, map[string][]Star{
//	//	"result": "ok",
//	//})
//	//re.JSON(w, http.StatusOK, map[string]string{"result": "ok"})
//}
