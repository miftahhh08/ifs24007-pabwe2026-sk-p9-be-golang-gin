package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body adalah format JSON standar seluruh endpoint: {status, message, data?}.
type Body struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success mengirim respons sukses.
func Success(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, Body{Status: "success", Message: message, Data: data})
}

// Fail mengirim respons kegagalan akibat kesalahan klien (4xx).
func Fail(c *gin.Context, code int, message string) {
	c.JSON(code, Body{Status: "fail", Message: message})
}

// Error mengirim respons kesalahan server (500).
func Error(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Body{Status: "error", Message: message})
}
