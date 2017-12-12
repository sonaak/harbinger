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

func MustGetServerMeta(env goenv.EnvReader) *ServerMeta {
	marshaller := goenv.DefaultEnvMarshaler{
		Environment: env,
	}

	meta := ServerMeta{}
	marshalErr := marshaller.Unmarshal(&meta)
	if marshalErr != nil {
		panic(marshalErr)
	}

	return &meta
}

type Config struct {
}

func NewConfig(env goenv.EnvReader) *Config {
	return &Config{}
}

func setupFlags() {
	flag.Parse()
}

func Setup(env goenv.EnvReader) (*App, error) {
	setupFlags()

	config := NewConfig(env)
	meta := MustGetServerMeta(env)
	server := newWebServer(config, meta)

	return &App{
		server,
	}, nil
}

type webserver struct {
	*gin.Engine
	Meta *ServerMeta
}


func newWebServer(config *Config, meta *ServerMeta) *webserver {
	engine := gin.New()
	server := &webserver{
		Engine: engine,
		Meta: meta,
	}

	engine.GET("/meta", server.meta)
	return server
}

func (server *webserver) meta(c *gin.Context) {
	c.JSON(200, struct {
		Meta *ServerMeta `json:"meta"`
	}{
		Meta: server.Meta,
	})
}

func (server *webserver) Run() error {
	return server.Engine.Run()
}

type App struct {
	*webserver
}

func (app *App) Run() error {

	go app.webserver.Run()

	for range time.Tick(1 * time.Second) {
		glog.Info("Printing hello world")
		fmt.Println("Hello world!")
	}

	return nil
}
