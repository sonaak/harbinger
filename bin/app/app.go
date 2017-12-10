package app

import (
	"github.com/golang/glog"
	"fmt"
	"flag"
	"time"
)

func Setup() (*App, error) {
	flag.Parse()

	return &App{}, nil
}

type App struct {}


func (app *App) Run() error {

	for range time.Tick(1 * time.Second) {
		glog.Info("Printing hello world")
		fmt.Println("Hello world!")
	}

	return nil
}