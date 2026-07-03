package user

import "github.com/gin-gonic/gin"

type Handler struct {
}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) RegisterRoutes(route *gin.Engine) {
	group := route.Group("/api/v1")
	{
		group.GET("/users", h.GetUsers)
	}
}

func (h *Handler) GetUsers(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "Get Users",
	})
}
