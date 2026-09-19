// query.go 提供有界作品列表及本人版本读取。
package storyhttp

import (
	"ai-chat/backend/internal/reply"
	"ai-chat/backend/internal/story/domain"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

func readPage(c *gin.Context) (int, error) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 1000000 { // audit:allow-no-log 调用方统一返回固定查询错误。
		return 0, domain.ErrInvalid
	}
	return page, nil
}

func (h *Handler) list(c *gin.Context) {
	page, err := readPage(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, domain.ErrInvalid)
		return
	}
	out, err := h.service.List(c.Request.Context(), c.GetUint64("user_id"), page)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, gin.H{"items": out, "page": page, "size": 20})
}

func (h *Handler) publicList(c *gin.Context) {
	page, err := readPage(c)
	query := strings.TrimSpace(c.Query("q"))
	if err != nil || len(query) > 64 { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, domain.ErrInvalid)
		return
	}
	out, err := h.service.PublicList(c.Request.Context(), page, query)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, gin.H{"items": out, "page": page, "size": 20})
}

func (h *Handler) publicFind(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	out, err := h.service.PublicFind(c.Request.Context(), id)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, out)
}

func (h *Handler) find(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	out, err := h.service.Find(c.Request.Context(), c.GetUint64("user_id"), id)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, out)
}

func (h *Handler) version(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	out, err := h.service.Version(c.Request.Context(), c.GetUint64("user_id"), id)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, out)
}
