package main


import (
	"github.com/sonaak/harbinger/bin/app"
)


func main() {
	processor, err := app.Setup()

	if err != nil {
		panic(err)
	}

	processor.Run()
}
