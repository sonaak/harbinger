package main

import (
	"github.com/evilwire/go-env"
	"github.com/sonaak/harbinger/bin/app"
)

func main() {
	processor, err := app.Setup(goenv.NewOsEnvReader())

	if err != nil {
		panic(err)
	}

	processor.Run()
}
