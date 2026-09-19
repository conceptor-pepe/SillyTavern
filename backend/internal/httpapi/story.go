// story.go 集中装配内容模块，HTTP 层不接触作品业务规则。
package httpapi

import (
	personaapp "ai-chat/backend/internal/persona/app"
	personahttp "ai-chat/backend/internal/persona/http"
	personainfra "ai-chat/backend/internal/persona/infra"
	storyapp "ai-chat/backend/internal/story/app"
	storyhttp "ai-chat/backend/internal/story/http"
	storyinfra "ai-chat/backend/internal/story/infra"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func registerStory(e *gin.Engine, db *gorm.DB, auth gin.HandlerFunc, logger *zap.Logger) {
	storyhttp.New(storyapp.New(storyinfra.New(db), logger), logger).Routes(e, auth)
	personahttp.New(personaapp.New(personainfra.New(db), logger), logger).Routes(e, auth)
}
