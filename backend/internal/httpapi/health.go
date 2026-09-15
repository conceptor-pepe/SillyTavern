// health.go 负责提供服务存活和就绪检查接口。
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerHealth 注册不依赖外部数据库的基础检查接口。
func registerHealth(engine *gin.Engine) {
	engine.GET("/healthz", health)
	engine.GET("/readyz", ready)
}

// health 返回进程存活状态。
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ready 返回基础服务就绪状态。
func ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
