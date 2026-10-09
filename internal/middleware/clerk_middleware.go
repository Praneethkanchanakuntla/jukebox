package middleware

import (
	"strings"

	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/gin-gonic/gin"
)

func ClerkAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		parts := strings.Fields(header)

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "invalid authorization header"})
			return
		}

		token := parts[1]
		claims, err := jwt.Verify(ctx.Request.Context(), &jwt.VerifyParams{
			Token: token,
		})

		if err != nil {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "invalid session token"})
			return
		}

		if claims.Subject == "" {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "missing user identity"})
			return
		}
		ctx.Set("clerk_user_id", claims.Subject)
		ctx.Next()
	}
}
