package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	HdlUser "github.com/freeDog-wy/go-backend-template/internal/handler/user"
	"github.com/gin-gonic/gin"
)

type App struct {
	server *http.Server
}

func initApp(cfg *config.Config) *App {
	userHdl := HdlUser.New()

	registry := handler.NewRegistry()

	registry.Add(userHdl)

	if cfg.App.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	registry.RegisterAll(r)

	server := &http.Server{
		Addr:         cfg.Server.IP + ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}
	return &App{
		server: server,
	}
}
