package app

import (
	"flag"
	"fmt"
	"github.com/evilwire/go-env"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"time"
)

type ServerMeta struct {
	BuildTime time.Time `env:"BUILD_TIME" json:"build-time"`
	GHash     string    `env:"GHASH" json:"ghash"`
	Version   string    `env:"VERSION" json:"version"`
}

type Config struct {
}

func Setup(env goenv.EnvReader) (*App, error) {
	flag.Parse()

	marshaller := goenv.DefaultEnvMarshaler{
		Environment: env,
	}

	meta := ServerMeta{}
	marshalErr := marshaller.Unmarshal(&meta)
	if marshalErr != nil {
		return nil, marshalErr
	}

	return &App{
		Meta: &meta,
	}, nil
}

type App struct {
	Config *Config
	Meta   *ServerMeta
}

func (app *App) Run() error {

	r := gin.Default()
	r.GET("/meta", func(c *gin.Context) {
		c.JSON(200, app.Meta)
	})

	go r.Run()

	for range time.Tick(1 * time.Second) {
		glog.Info("Printing hello world")
		fmt.Println("Hello world!")
	}

	return nil
}
