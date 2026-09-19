// edit.go 提供完整角色资料更新与软删除，历史会话保留供读取。
package characterhttp

import (
	"ai-chat/backend/internal/character/app"
	"ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/reply"
	"errors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

type characterInput struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Personality   string   `json:"personality"`
	Scenario      string   `json:"scenario"`
	FirstMessage  string   `json:"first_message"`
	Portrait      string   `json:"portrait"`
	Tags          []string `json:"tags"`
	Gender        string   `json:"gender"`
	Age           string   `json:"age"`
	MessageSample string   `json:"message_sample"`
}

// updateChar 更新当前用户的角色，不接收客户端指定的归属字段。
func (h *Handler) updateChar(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	var in characterInput
	if err != nil || c.ShouldBindJSON(&in) != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		reply.Fail(c, 400, "INVALID_INPUT", "角色资料无效")
		return
	}
	uid := contextID(c)
	if _, err := h.repo.Find(c.Request.Context(), uid, id); err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		h.editFail(c, err)
		return
	}
	item := domain.Character{ID: id, UserID: uid, Name: in.Name, Description: in.Description, Personality: in.Personality, Scenario: in.Scenario, FirstMessage: in.FirstMessage, Portrait: in.Portrait, Tags: in.Tags, Gender: in.Gender, Age: in.Age, MessageSample: in.MessageSample}
	if err := app.Validate(&item); err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		reply.Fail(c, 400, "INVALID_INPUT", "角色资料无效")
		return
	}
	repo, ok := h.repo.(domain.EditorRepo)
	if !ok {
		reply.Fail(c, 503, "UNAVAILABLE", "角色编辑暂不可用")
		return
	}
	out, err := repo.Update(c.Request.Context(), item)
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		h.editFail(c, err)
		return
	}
	h.logger.Info("character updated", zap.Uint64("user_id", uid), zap.Uint64("character_id", id))
	reply.OK(c, out)
}

// deleteChar 只软删除角色，不级联清除用户的聊天历史。
func (h *Handler) deleteChar(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		reply.Fail(c, 400, "INVALID_INPUT", "角色编号无效")
		return
	}
	repo, ok := h.repo.(domain.EditorRepo)
	if !ok {
		reply.Fail(c, 503, "UNAVAILABLE", "角色编辑暂不可用")
		return
	}
	if err := repo.Delete(c.Request.Context(), contextID(c), id); err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		h.editFail(c, err)
		return
	}
	h.logger.Info("character deleted", zap.Uint64("user_id", contextID(c)), zap.Uint64("character_id", id))
	reply.OK(c, gin.H{"deleted": true})
}

// editFail 不泄露其他账号角色与底层数据库错误。
func (h *Handler) editFail(c *gin.Context, err error) {
	h.logger.Warn("character mutation failed", zap.Uint64("user_id", contextID(c)), zap.Error(logx.SafeError(err)))
	if errors.Is(err, domain.ErrNotFound) {
		reply.Fail(c, 404, "NOT_FOUND", "角色不存在或已删除")
		return
	}
	reply.Fail(c, 500, "STORAGE_ERROR", "角色暂时无法保存")
}
