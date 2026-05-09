package chatwoot_router

import (
	"github.com/gin-gonic/gin"

	chatwoot_controller "github.com/EvolutionAPI/evolution-go/pkg/chatwoot/controller"
)

// RegisterRoutes mounts the Chatwoot endpoints on the provided engine.
// Set/Find routes require the AuthAdmin middleware (passed as authMiddleware).
// The webhook route is unauthenticated (Chatwoot does not send API keys).
func RegisterRoutes(r *gin.Engine, ctrl *chatwoot_controller.ChatwootController, authMiddleware gin.HandlerFunc) {
	// Admin-protected configuration routes
	admin := r.Group("/chatwoot")
	admin.Use(authMiddleware)
	{
		admin.POST("/set/:instance", ctrl.SetChatwoot)
		admin.GET("/find/:instance", ctrl.FindChatwoot)
	}

	// Public webhook receiver (no auth — Chatwoot doesn't send API keys)
	r.POST("/chatwoot/webhook/:instance", ctrl.ReceiveWebhook)
}
