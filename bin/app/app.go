package app

import (
	"github.com/golang/glog"
	"fmt"
	"flag"
	"time"
	"github.com/gin-gonic/gin"
)

func Setup() (*App, error) {
	flag.Parse()

	return &App{}, nil
}

type App struct {}


func (app *App) Run() error {

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	go r.Run()

	for range time.Tick(1 * time.Second) {
		glog.Info("Printing hello world")
		fmt.Println("Hello world!")
	}

	return nil
}