package websocket

import (
	"net/http"
	"strings"

	"example.com/aim/gateway/internal/authcookie"
	"example.com/aim/gateway/internal/rpc"
	authpb "example.com/aim/gateway/kitex_gen/auth"
	"github.com/gin-gonic/gin"
	gwebsocket "github.com/gorilla/websocket"
)

var upgrader = gwebsocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleChat(ctx *gin.Context, hub *Hub) {
	token := accessToken(ctx)
	if token == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": "missing access token",
		})
		return
	}

	authClient, err := rpc.AuthClient()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	authResp, err := authClient.ValidateToken(ctx.Request.Context(), &authpb.ValidateTokenRequest{
		AccessToken: token,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}
	if !authResp.Valid {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": authResp.Reason,
		})
		return
	}

	chatClient, err := rpc.ChatClient()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	client := NewClient(authResp.UserId, conn, hub, chatClient)
	client.Run(ctx.Request.Context())
}

func accessToken(ctx *gin.Context) string {
	if token := strings.TrimSpace(ctx.Query("token")); token != "" {
		return token
	}
	return authcookie.AccessToken(ctx)
}
