package middleware

import "github.com/gin-gonic/gin"

// AIConsoleUploadCleanup removes temporary multipart files after parsing.
func AIConsoleUploadCleanup() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}
}
