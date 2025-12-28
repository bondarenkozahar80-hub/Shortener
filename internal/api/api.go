package api

import (
	"github.com/wb-go/wbf/ginext"
	"secondOne/cmd/middleware"
	"secondOne/internal/service"
)

type Routers struct {
	Service service.Service
}

func NewRouters(r *Routers) *ginext.Engine {
	app := ginext.New()

	app.Use(middleware.LoggingMiddleware())

	apiGroup := app.Group("/v1")

	apiGroup.POST("/shorten", r.Service.CreateUrl)
	apiGroup.GET("/s/:short_url", r.Service.Redirect)
	apiGroup.GET("/analytics/:short_url", r.Service.ShowAnalytics)

	return app
}
