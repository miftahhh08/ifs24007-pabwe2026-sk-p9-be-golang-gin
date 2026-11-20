package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func run(h gin.HandlerFunc) (*httptest.ResponseRecorder, Body) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	h(c)
	var b Body
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return w, b
}

func TestSuccess(t *testing.T) {
	w, b := run(func(c *gin.Context) { Success(c, 201, "ok", gin.H{"a": 1}) })
	assert.Equal(t, 201, w.Code)
	assert.Equal(t, "success", b.Status)
	assert.NotNil(t, b.Data)
}

func TestFail(t *testing.T) {
	w, b := run(func(c *gin.Context) { Fail(c, 404, "nope") })
	assert.Equal(t, 404, w.Code)
	assert.Equal(t, "fail", b.Status)
	assert.Nil(t, b.Data)
}

func TestError(t *testing.T) {
	w, b := run(func(c *gin.Context) { Error(c, "boom") })
	assert.Equal(t, 500, w.Code)
	assert.Equal(t, "error", b.Status)
}
