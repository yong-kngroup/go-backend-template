package handler

import "github.com/gin-gonic/gin"

type Registrar interface {
	RegisterRoutes(route *gin.Engine)
}

type Registry struct {
	handlers []Registrar
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: []Registrar{},
	}
}

func (r *Registry) Add(handler Registrar) {
	r.handlers = append(r.handlers, handler)
}

func (r *Registry) RegisterAll(route *gin.Engine) {

	for _, handler := range r.handlers {
		handler.RegisterRoutes(route)
	}
}
