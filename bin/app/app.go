package app

import (
	"github.com/golang/glog"
	"fmt"
)

type App struct {}


func (app *App) Run() error {
	glog.Info("Printing hello world")
	fmt.Println("Hello world!")

	return nil
}