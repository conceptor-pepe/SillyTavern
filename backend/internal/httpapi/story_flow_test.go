// story_flow_test.go 验证故事版本、人设、开场与生成输入之间的实际 API 闭环。
package httpapi

import (
	chat "ai-chat/backend/internal/chat/domain"
	genapp "ai-chat/backend/internal/generation/app"
	memory "ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/model"
	provider "ai-chat/backend/internal/provider/domain"
	story "ai-chat/backend/internal/story/domain"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"testing"
)

func storyJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func storyFixture() story.Definition {
	return story.Definition{SchemaVersion: 1, Title: "雨夜书店", Hook: "一本写着你名字的书", World: "原始月港", Cast: []story.Cast{{ID: "lin", Name: "林晚", Personality: "谨慎"}}, Opening: []story.Segment{{ID: "n1", Kind: "narration", Text: "雨水敲着窗"}, {ID: "d1", Kind: "dialogue", SpeakerID: "lin", Text: "我一直在等你"}}, Examples: "示例风格，不是真实经历", Lore: []story.Lore{{Content: "书店午夜关门", Keywords: []string{"书店"}}}}
}

func startStoryFlow(t *testing.T, f *flowEnv, version, persona, key string) chat.StoryState {
	t.Helper()
	body := `{"mode":"story","story_version_id":"` + version + `","persona_id":"` + persona + `","idempotency_key":"` + key + `"}`
	var out struct{ Data chat.StoryState }
	if err := json.Unmarshal(f.call(t, "POST", "/api/v1/chats", body, 200), &out); err != nil {
		t.Fatal(err)
	}
	if out.Data.Chat.Mode != "story" || out.Data.Chat.ID == 0 {
		t.Fatalf("invalid state: %+v", out.Data)
	}
	return out.Data
}

// TestStoryFlowMySQL 覆盖版本、人设、幂等开聊、记忆隔离与生成上下文。
func TestStoryFlowMySQL(t *testing.T) {
	requests := make(chan provider.Request, 8)
	f := newFlow(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request provider.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if len(request.Messages) > 0 && request.Messages[len(request.Messages)-1].Content == genapp.SuggestionInstruction {
			w.Header().Set("Content-Type", "text/event-stream")
			_, err := w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"[\\\"继续追问\\\",\\\"先观察她\\\",\\\"坦白自己的来意\\\"]\"}}]}\n\ndata: [DONE]\n\n"))
			if err != nil {
				t.Error(err)
			}
			return
		}
		requests <- request
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"故事回答\"}}]}\n\ndata: [DONE]\n\n")); err != nil {
			t.Error(err)
		}
	}))
	f.login(t)
	definition := storyFixture()
	sid := responseID(t, f.call(t, "POST", "/api/v1/stories", storyJSON(t, map[string]any{"definition": definition}), 200))
	version := responseID(t, f.call(t, "POST", "/api/v1/stories/"+sid+"/versions", `{"expected_revision":"1"}`, 200))
	repeat := responseID(t, f.call(t, "POST", "/api/v1/stories/"+sid+"/versions", `{"expected_revision":"1"}`, 200))
	if version != repeat {
		t.Fatal("freeze not idempotent")
	}
	pid := responseID(t, f.call(t, "POST", "/api/v1/personas", `{"name":"阿远","description":"插画师"}`, 200))
	first := startStoryFlow(t, f, version, pid, "first-story-key")
	again := startStoryFlow(t, f, version, pid, "first-story-key")
	if first.Chat.ID != again.Chat.ID || first.OpeningID == nil || *first.OpeningID != *again.OpeningID {
		t.Fatal("start not idempotent")
	}
	definition.World = "后来被改成沙漠"
	f.call(t, "PUT", "/api/v1/stories/"+sid, storyJSON(t, map[string]any{"definition": definition, "expected_revision": "1"}), 200)
	f.call(t, "PUT", "/api/v1/stories/"+sid, storyJSON(t, map[string]any{"definition": definition, "expected_revision": "1"}), 409)
	f.call(t, "POST", "/api/v1/stories/"+sid+"/versions", `{"expected_revision":"1"}`, 409)
	version2 := responseID(t, f.call(t, "POST", "/api/v1/stories/"+sid+"/versions", `{"expected_revision":"2"}`, 200))
	f.call(t, "POST", "/api/v1/chats", `{"mode":"story","story_version_id":"`+version2+`","persona_id":"`+pid+`","idempotency_key":"first-story-key"}`, 409)
	f.call(t, "PUT", "/api/v1/personas/"+pid, `{"name":"另一个身份","expected_revision":"1"}`, 200)
	second := startStoryFlow(t, f, version, pid, "second-story-key")
	if second.Persona.Name != "另一个身份" || first.Persona.Name != "阿远" {
		t.Fatal("persona snapshots not independent")
	}
	chatPath := "/api/v1/chats/" + strconv.FormatUint(first.Chat.ID, 10)
	f.call(t, "POST", chatPath+"/memories", `{"kind":"fact","content":"暗号是蓝鲸","enabled":true,"pinned":true}`, 200)
	f.call(t, "DELETE", "/api/v1/stories/"+sid, "", 200)
	f.call(t, "DELETE", "/api/v1/personas/"+pid, "", 200)
	f.call(t, "GET", "/api/v1/stories/"+sid, "", 404)
	f.call(t, "POST", "/api/v1/chats", `{"mode":"story","story_version_id":"`+version+`","persona_id":"`+pid+`","idempotency_key":"third-story-key"}`, 404)
	checkStoryPrompt(t, f, first, requests, true)
	checkStoryPrompt(t, f, second, requests, false)
	checkStoryForeign(t, f, foreignRefs{StoryID: sid, VersionID: version, PersonaID: pid, ChatID: first.Chat.ID})
}

// TestPublicStoryFlowMySQL 验证匿名发现、跨账号开聊与撤下后的会话保留。
func TestPublicStoryFlowMySQL(t *testing.T) {
	f := newFlow(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n")) // 测试不调用生成，兜底响应保持 Provider 协议完整。
	}))
	f.login(t)
	sid := responseID(t, f.call(t, "POST", "/api/v1/stories", storyJSON(t, map[string]any{
		"definition": storyFixture(),
	}), 200))
	var published struct {
		Data story.Story `json:"data"`
	}
	value := f.call(t, "POST", "/api/v1/stories/"+sid+"/publication", `{"expected_revision":"1"}`, 200)
	if err := json.Unmarshal(value, &published); err != nil || published.Data.PublishedVersionID == nil {
		t.Fatalf("publication missing version: %s err=%v", value, err)
	}

	reader := publicReader(t, f)
	reader.call(t, "GET", "/api/v1/public/stories?q=%E9%9B%A8%E5%A4%9C", "", 200)
	public := reader.call(t, "GET", "/api/v1/public/stories/"+sid, "", 200)
	if strings.Contains(string(public), "示例风格") || strings.Contains(string(public), "书店午夜关门") {
		t.Fatalf("public response leaked internal writing hints: %s", public)
	}
	reader.call(t, "GET", "/api/v1/stories/"+sid, "", 404)
	pid := responseID(t, reader.call(t, "POST", "/api/v1/personas", `{"name":"访客"}`, 200))
	versionID := strconv.FormatUint(*published.Data.PublishedVersionID, 10)
	state := startStoryFlow(t, reader, versionID, pid, "public-story-key")

	f.call(t, "DELETE", "/api/v1/stories/"+sid+"/publication", "", 200)
	reader.call(t, "GET", "/api/v1/public/stories/"+sid, "", 404)
	reader.call(t, "POST", "/api/v1/chats", `{"mode":"story","story_version_id":"`+versionID+`","persona_id":"`+pid+`","idempotency_key":"public-story-next"}`, 404)
	reader.call(t, "GET", "/api/v1/chats/"+strconv.FormatUint(state.Chat.ID, 10)+"/bootstrap", "", 200)
}

// TestRelationshipContinuityMySQL 验证同一长期角色跨故事共享关系与记忆，但仍使用各自冻结世界。
func TestRelationshipContinuityMySQL(t *testing.T) {
	requests := make(chan provider.Request, 2)
	f := newFlow(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request provider.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if strings.Contains(promptText(request), "记忆整理器") {
			content := `[{"scope":"relationship","content":"用户希望林晚称呼自己为小鹿","evidence":"她答应叫我小鹿"},{"scope":"story","content":"两人在雪山木屋重逢","evidence":"当前故事发生在雪山木屋"}]`
			chunk := storyJSON(t, map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": content}}}})
			_, _ = w.Write([]byte("data: " + chunk + "\n\ndata: [DONE]\n\n"))
			return
		}
		requests <- request
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"我记得\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	f.login(t)
	companionID := responseNumericID(t, f.call(t, "POST", "/api/v1/characters", `{"name":"林晚","personality":"温柔但谨慎"}`, 200))
	personaID := responseID(t, f.call(t, "POST", "/api/v1/personas", `{"name":"阿远"}`, 200))
	first := relationshipStory(t, f, companionID, personaID, "月港书店", "relationship-story-one")
	second := relationshipStory(t, f, companionID, personaID, "雪山木屋", "relationship-story-two")
	if first.Relationship == nil || second.Relationship == nil || first.Relationship.ID != second.Relationship.ID {
		t.Fatalf("relationship not shared: first=%+v second=%+v", first.Relationship, second.Relationship)
	}
	firstPath := "/api/v1/chats/" + strconv.FormatUint(first.Chat.ID, 10)
	secondPath := "/api/v1/chats/" + strconv.FormatUint(second.Chat.ID, 10)
	f.call(t, "PUT", firstPath+"/relationship", `{"stage":"close","narrative":"我们已经互相信任","milestones":["一起守过书店的雨夜"],"expected_revision":"1"}`, 200)
	f.call(t, "PUT", firstPath+"/relationship", `{"stage":"friend","narrative":"过期写入","milestones":[],"expected_revision":"1"}`, 409)
	f.call(t, "POST", "/api/v1/relationships/"+companionID+"/memories", `{"kind":"fact","content":"她答应叫我小鹿","enabled":true,"pinned":true}`, 200)
	state := f.call(t, "GET", secondPath+"/bootstrap", "", 200)
	if !strings.Contains(string(state), "我们已经互相信任") || !strings.Contains(string(state), `"stage":"close"`) {
		t.Fatalf("relationship update missing from another story: %s", state)
	}
	parentID := strconv.FormatUint(*second.OpeningID, 10)
	messageID := responseID(t, f.call(t, "POST", secondPath+"/messages", `{"content":"你还记得我吗","parent_id":"`+parentID+`"}`, 200))
	f.call(t, "POST", secondPath+"/generations", `{"model":"test","parent_id":"`+messageID+`"}`, 200)
	request := <-requests
	content := promptText(request)
	for _, want := range []string{"雪山木屋", "我们已经互相信任", "一起守过书店的雨夜", "她答应叫我小鹿"} {
		if !strings.Contains(content, want) {
			t.Fatalf("relationship context missing %q: %s", want, content)
		}
	}
	var generated model.Message
	if err := f.db.WithContext(t.Context()).Where("conversation_id = ? AND role = ?", second.Chat.ID, "assistant").Order("id DESC").First(&generated).Error; err != nil {
		t.Fatal(err)
	}
	leafID := strconv.FormatUint(generated.ID, 10)
	candidates := extractCandidates(t, f, second.Chat.ID, leafID)
	if len(candidates) != 2 || candidates[0].Scope == candidates[1].Scope {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	for _, item := range candidates {
		path := "/api/v1/memory-candidates/" + strconv.FormatUint(item.ID, 10)
		if item.Scope == "relationship" {
			f.call(t, "POST", path+"/accept", "", 200)
			f.call(t, "POST", path+"/accept", "", 409)
		} else {
			f.call(t, "DELETE", path, "", 200)
		}
	}
	f.call(t, "POST", secondPath+"/memory-candidates", `{"leaf_id":"`+leafID+`"}`, 200)
	pending := f.call(t, "GET", secondPath+"/memory-candidates", "", 200)
	if strings.Contains(string(pending), "用户希望林晚") || strings.Contains(string(pending), "雪山木屋重逢") {
		t.Fatalf("processed candidates returned again: %s", pending)
	}
	memories := f.call(t, "GET", "/api/v1/relationships/"+companionID+"/memories", "", 200)
	if !strings.Contains(string(memories), "用户希望林晚称呼自己为小鹿") {
		t.Fatalf("accepted candidate not saved: %s", memories)
	}
	foreign := publicReader(t, f)
	foreign.call(t, "GET", firstPath+"/relationship", "", 404)
	foreign.call(t, "GET", "/api/v1/relationships/"+companionID+"/memories", "", 404)
	foreign.call(t, "GET", secondPath+"/memory-candidates", "", 404)
}

func extractCandidates(t *testing.T, f *flowEnv, chatID uint64, leafID string) []memory.Candidate {
	t.Helper()
	path := "/api/v1/chats/" + strconv.FormatUint(chatID, 10) + "/memory-candidates"
	value := f.call(t, "POST", path, `{"leaf_id":"`+leafID+`"}`, 200)
	var result struct {
		Data struct {
			Items []memory.Candidate `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(value, &result); err != nil {
		t.Fatal(err)
	}
	return result.Data.Items
}

func responseNumericID(t *testing.T, value []byte) string {
	t.Helper()
	var result struct{ Data struct{ ID uint64 } }
	if err := json.Unmarshal(value, &result); err != nil || result.Data.ID == 0 {
		t.Fatalf("missing numeric data.id: %s err=%v", value, err)
	}
	return strconv.FormatUint(result.Data.ID, 10)
}

func relationshipStory(t *testing.T, f *flowEnv, companionID, personaID, world, key string) chat.StoryState {
	t.Helper()
	definition := storyFixture()
	definition.World = world
	definition.Cast[0].CompanionID, _ = strconv.ParseUint(companionID, 10, 64)
	storyID := responseID(t, f.call(t, "POST", "/api/v1/stories", storyJSON(t, map[string]any{"definition": definition}), 200))
	versionID := responseID(t, f.call(t, "POST", "/api/v1/stories/"+storyID+"/versions", `{"expected_revision":"1"}`, 200))
	return startStoryFlow(t, f, versionID, personaID, key)
}

func promptText(request provider.Request) string {
	var out strings.Builder
	for _, item := range request.Messages {
		out.WriteString(item.Content)
	}
	return out.String()
}

func publicReader(t *testing.T, author *flowEnv) *flowEnv {
	t.Helper()
	client := *author.client
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client.Jar = jar
	reader := &flowEnv{url: author.url, client: &client}
	reader.call(t, "GET", "/api/v1/public/stories", "", 200)
	reader.call(t, "POST", "/api/v1/auth/register",
		`{"handle":"public-reader","password":"local-reader-password","name":"读者"}`, 201)
	return reader
}

func checkStoryPrompt(t *testing.T, f *flowEnv, state chat.StoryState, requests <-chan provider.Request, hasMemory bool) {
	t.Helper()
	path := "/api/v1/chats/" + strconv.FormatUint(state.Chat.ID, 10)
	body := `{"content":"书店在哪里","parent_id":"` + strconv.FormatUint(*state.OpeningID, 10) + `"}`
	mid := responseID(t, f.call(t, "POST", path+"/messages", body, 200))
	events := f.call(t, "POST", path+"/generations", `{"model":"test","parent_id":"`+mid+`"}`, 200)
	if !strings.Contains(string(events), "message_end") {
		t.Fatalf("generation failed: %s", events)
	}
	request := <-requests
	var content strings.Builder
	for _, m := range request.Messages {
		content.WriteString(m.Content)
	}
	all := content.String()
	for _, want := range []string{"原始月港", state.Persona.Name, "书店午夜关门", "林晚"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(all, "后来被改成沙漠") || strings.Count(all, "我一直在等你") != 1 {
		t.Fatal("mutable version or duplicated opening")
	}
	if strings.Contains(all, "暗号是蓝鲸") != hasMemory {
		t.Fatal("memory crossed story sessions")
	}
	checkChatControls(t, f, state.Chat.ID)
	f.call(t, "GET", path+"/bootstrap", "", 200)
}

// checkChatControls 验证回复建议不入历史，AI 编辑另建同父节点分支。
func checkChatControls(t *testing.T, f *flowEnv, chatID uint64) {
	t.Helper()
	var source model.Message
	err := f.db.WithContext(t.Context()).Where("conversation_id = ? AND role = ?", chatID, "assistant").
		Order("id DESC").First(&source).Error
	if err != nil {
		t.Fatal(err)
	}
	chatPath := "/api/v1/chats/" + strconv.FormatUint(chatID, 10)
	sourceID := strconv.FormatUint(source.ID, 10)
	var before int64
	if err := f.db.WithContext(t.Context()).Model(&model.Message{}).Where("conversation_id = ?", chatID).Count(&before).Error; err != nil {
		t.Fatal(err)
	}
	suggestions := string(f.call(t, "POST", chatPath+"/reply-suggestions",
		`{"model":"test","parent_id":"`+sourceID+`"}`, 200))
	for _, want := range []string{"继续追问", "先观察她", "坦白自己的来意"} {
		if !strings.Contains(suggestions, want) {
			t.Fatalf("suggestions missing %q: %s", want, suggestions)
		}
	}
	var after int64
	if err := f.db.WithContext(t.Context()).Model(&model.Message{}).Where("conversation_id = ?", chatID).Count(&after).Error; err != nil || after != before {
		t.Fatalf("suggestions changed history: before=%d after=%d err=%v", before, after, err)
	}
	userID := strconv.FormatUint(*source.ParentID, 10)
	f.call(t, "POST", chatPath+"/reply-suggestions", `{"model":"test","parent_id":"`+userID+`"}`, 400)
	f.call(t, "POST", "/api/v1/messages/"+userID+"/revisions", `{"content":"不能编辑用户消息"}`, 409)
	revisedID := responseID(t, f.call(t, "POST", "/api/v1/messages/"+sourceID+"/revisions",
		`{"content":"编辑后的剧情回复"}`, 200))
	if revisedID == sourceID {
		t.Fatal("revision overwrote source message")
	}
	var revised model.Message
	if err := f.db.WithContext(t.Context()).First(&revised, revisedID).Error; err != nil {
		t.Fatal(err)
	}
	if revised.Content != "编辑后的剧情回复" || revised.ParentID == nil || source.ParentID == nil ||
		*revised.ParentID != *source.ParentID {
		t.Fatalf("source=%+v revised=%+v", source, revised)
	}
	if err := f.db.WithContext(t.Context()).First(&source, source.ID).Error; err != nil || source.Content != "故事回答" {
		t.Fatalf("source changed: %+v err=%v", source, err)
	}
}

type foreignRefs struct {
	StoryID   string
	VersionID string
	PersonaID string
	ChatID    uint64
}

func checkStoryForeign(t *testing.T, f *flowEnv, refs foreignRefs) {
	t.Helper()
	client := *f.client
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client.Jar = jar
	other := &flowEnv{url: f.url, client: &client}
	other.call(t, "POST", "/api/v1/auth/register", `{"handle":"story-foreign","password":"local-test-password","name":"另一个用户"}`, 201)
	other.call(t, "GET", "/api/v1/story-versions/"+refs.VersionID, "", 404)
	other.call(t, "GET", "/api/v1/stories/"+refs.StoryID, "", 404)
	other.call(t, "GET", "/api/v1/personas/"+refs.PersonaID, "", 404)
	path := "/api/v1/chats/" + strconv.FormatUint(refs.ChatID, 10)
	var assistant model.Message
	if err := f.db.WithContext(t.Context()).Where("conversation_id = ? AND role = ?", refs.ChatID, "assistant").
		Order("id DESC").First(&assistant).Error; err != nil {
		t.Fatal(err)
	}
	assistantID := strconv.FormatUint(assistant.ID, 10)
	other.call(t, "GET", path+"/bootstrap", "", 404)
	other.call(t, "GET", path+"/memories", "", 404)
	other.call(t, "POST", path+"/memories", `{"kind":"fact","content":"伪造事实","enabled":true}`, 404)
	other.call(t, "POST", path+"/reply-suggestions", `{"model":"test","parent_id":"`+assistantID+`"}`, 404)
	other.call(t, "POST", "/api/v1/messages/"+assistantID+"/revisions", `{"content":"越权编辑"}`, 404)
}
