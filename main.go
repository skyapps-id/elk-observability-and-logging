package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"

	logrustash "github.com/bshuster-repo/logrus-logstash-hook"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.elastic.co/apm/module/apmgin/v2"
	"go.elastic.co/apm/module/apmsql/v2"
	_ "go.elastic.co/apm/module/apmsql/v2/mysql"
	"go.elastic.co/apm/v2"
)

type (
	User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	PostAPI struct {
		ID     int    `json:"id"`
		UserID int    `json:"userId"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	}

	MapResponse struct {
		ID     int    `json:"id"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Author User   `json:"author"`
	}
)

var (
	db     *sql.DB
	logger *logrus.Entry
)

func init() {
	// Setup Logger
	log := logrus.New()
	conn, err := net.Dial("tcp", "localhost:5000")
	if err != nil {
		panic(err)
	}
	hook := logrustash.New(conn, logrustash.DefaultFormatter(logrus.Fields{"type": "myappName", "env": "dev"}))
	log.Hooks.Add(hook)
	logger = log.WithFields(logrus.Fields{
		"method": "main",
	})

	// Setup Database
	db, err = apmsql.Open("mysql", "user:test@tcp(127.0.0.1:3306)/db")
	if err != nil {
		logger.Error(err)
	}
}

func main() {
	defer db.Close()
	r := gin.Default()
	r.Use(apmgin.Middleware(r))

	r.GET("/posts/:id", func(c *gin.Context) {
		span, ctx := apm.StartSpan(c.Request.Context(), "ControllerGetPostById", "request")
		defer span.End()

		id := c.Param("id")
		post := externalAPI(ctx, id)

		var user User
		err := db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id=?", post.UserID).Scan(&user.ID, &user.Name, &user.Email)
		if err != nil {
			logger.Error(err)
		}

		c.JSON(http.StatusOK, MapResponse{
			ID:    post.ID,
			Title: post.Title,
			Body:  post.Body,
			Author: User{
				ID:    user.ID,
				Name:  user.Name,
				Email: user.Email,
			},
		})
	})

	r.Run()
}

func externalAPI(ctx context.Context, id string) PostAPI {
	span, ctx := apm.StartSpan(ctx, "FetchPostAPI", "api-call")
	defer span.End()

	var data PostAPI
	resp, err := http.Get("https://jsonplaceholder.typicode.com/posts/" + id)
	if err != nil {
		logger.Error(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Fatal(err)
	}
	err = json.Unmarshal([]byte(string(body)), &data)
	if err != nil {
		logger.Error(err)
	}
	logger.Info("Success Fetch API")

	return data
}
