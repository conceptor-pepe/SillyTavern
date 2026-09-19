// opening.go 保留开场发言归属，兼容旧消息读取协议。
package domain

import "strings"

// OpeningText 生成可用于历史和摘要的带说话人文本。
// @param d 故事定义
// @return 开场文本
func OpeningText(d Definition) string {
	names := make(map[string]string, len(d.Cast))
	for _, c := range d.Cast {
		names[c.ID] = c.Name
	}
	var out strings.Builder
	for _, s := range d.Opening {
		name := "旁白"
		if s.Kind == "dialogue" {
			name = names[s.SpeakerID]
		}
		out.WriteString("[" + name + "] " + s.Text + "\n")
	}
	return strings.TrimSpace(out.String())
}
