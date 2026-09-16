/** 编排单个会话的持久化、分支选择和生成生命周期，不依赖 DOM。 */
import { apiClient, streamGeneration, streamRegeneration } from '../api-client.js';
import { allPages, branchPath, idText } from './data.js';

export class ChatSession {
    constructor({ api = apiClient, generate = streamGeneration, regenerate = streamRegeneration, prefs, changed = () => {} }) {
        Object.assign(this, { api, generate, regenerate, prefs, changed });
        this.chat = null;
        this.messages = [];
        this.leaf = null;
        this.active = null;
        this.busy = false;
        this.variants = new Map();
    }

    /** 操作互斥，阻止重复提交以及生成中切换会话。 */
    async work(action) {
        if (this.busy) throw new Error('当前操作尚未完成');
        this.busy = true;
        this.changed();
        try {
            return await action();
        } finally {
            this.busy = false;
            this.changed();
        }
    }

    /** 读取成功后才替换当前会话，失败时保留原页面。 */
    async open(chatID) {
        return this.work(async () => {
            const chat = await this.api.chat(chatID);
            const messages = await allPages(query => this.api.messages(chatID, query));
            this.chat = chat;
            this.messages = messages;
            this.variants.clear();
            this.leaf = this.restoreLeaf();
            this.prefs.set('chat', chat.id);
        });
    }

    /** 优先恢复用户已选择的分支，否则选择最新且祖先完整的消息。 */
    restoreLeaf() {
        const saved = this.prefs.get(`leaf:${this.chat.id}`);
        if (saved === '') return null;
        const ordered = [...this.messages].sort((a, b) => globalThis.BigInt(a.id) > globalThis.BigInt(b.id) ? -1 : 1);
        const candidates = [saved, ...ordered.map(item => item.id)].filter(Boolean);
        for (const id of candidates) {
            try {
                branchPath(this.messages, id);
                return id;
            } catch {
                // 删除祖先后的断裂分支不能作为继续聊天的上下文。
            }
        }
        return null;
    }

    /** 分支父节点持久化为用户隔离的本地偏好，不改写旧消息。 */
    choose(id) {
        if (this.busy) throw new Error('请先停止当前生成');
        this.setLeaf(id);
        this.changed();
    }

    setLeaf(id) {
        branchPath(this.messages, id);
        this.leaf = id;
        this.prefs.set(`leaf:${this.chat.id}`, id ?? '');
    }

    get path() {
        return branchPath(this.messages, this.leaf);
    }

    /** 先校验生成参数，再保存用户消息；生成失败时保留已保存的提问供重试。 */
    async send(content, options) {
        this.checkOptions(options);
        if (!content.trim()) throw new Error('消息不能为空');
        return this.work(async () => {
            const item = await this.api.addMessage(this.chat.id, { content: content.trim(), parent_id: this.leaf });
            this.messages.push(item);
            this.setLeaf(item.id);
            this.changed();
            await this.run(item, options, false);
        });
    }

    /** 重试用户消息不重复保存；重新生成 assistant 消息保留原回复。 */
    async retry(messageID, options) {
        this.checkOptions(options);
        const item = this.messages.find(message => message.id === messageID);
        if (!item || !['user', 'assistant'].includes(item.role)) throw new Error('消息不能重新生成');
        return this.work(() => this.run(item, options, item.role === 'assistant'));
    }

    checkOptions(options) {
        if (!this.chat) throw new Error('请先选择会话');
        if (!Number.isInteger(options.n) || options.n < 1 || options.n > 4) throw new Error('候选数量必须为 1 到 4');
    }

    /** 流事件只更新临时文本，正式回复以服务端持久化后的历史为准。 */
    async run(item, options, regenerate) {
        const active = { controller: new AbortController(), id: null, drafts: new Map(), stopped: false };
        this.active = active;
        this.changed();
        let resultID;
        let failure;
        const handlers = {
            message_start: value => { active.id = idText(value.generation_id); this.changed(); },
            message_delta: value => {
                active.drafts.set(value.index ?? 0, (active.drafts.get(value.index ?? 0) ?? '') + value.text);
                this.changed();
            },
            message_end: value => { resultID = idText(value.message_id); },
        };
        try {
            const body = { ...options, parent_id: item.id };
            const operation = regenerate ? this.regenerate : this.generate;
            await operation(regenerate ? item.id : this.chat.id, body, handlers, active.controller.signal);
        } catch (error) {
            if (!active.stopped || error.name !== 'AbortError') failure = error;
        } finally {
            this.active = null;
        }
        try {
            this.messages = await allPages(query => this.api.messages(this.chat.id, query));
            this.setLeaf(resultID ?? this.leaf);
        } catch (error) {
            failure ??= error;
        }
        if (failure) throw failure;
    }

    /** 优先让服务端终止任务；请求失败仍中止连接，避免界面永久等待。 */
    async stop() {
        const active = this.active;
        if (!active || active.stopped) return;
        active.stopped = true;
        this.changed();
        try {
            if (active.id) await this.api.cancelGeneration(active.id);
        } catch (error) {
            if (error.status !== 404) throw error;
        } finally {
            active.controller.abort();
        }
    }

    /** 候选属于原消息；选择结果是独立的新回复，不覆盖其后代。 */
    async select(messageID, variantID) {
        return this.work(async () => {
            const item = await this.api.selectVariant(messageID, variantID);
            if (!this.messages.some(message => message.id === item.id)) this.messages.push(item);
            this.setLeaf(item.id);
        });
    }

    async loadVariants(messageID) {
        return this.work(async () => {
            const items = await allPages(query => this.api.variants(messageID, query));
            this.variants.set(messageID, items);
        });
    }

    /** 当前只编辑叶节点，避免更改已有后续回答所依赖的历史。 */
    async edit(content) {
        const item = this.path.at(-1);
        this.checkLeaf(item);
        if (item.role !== 'user') throw new Error('只能编辑用户消息');
        return this.work(async () => {
            const updated = await this.api.editMessage(item.id, { content });
            this.messages = this.messages.map(message => message.id === item.id ? updated : message);
        });
    }

    async remove() {
        const item = this.path.at(-1);
        this.checkLeaf(item);
        return this.work(async () => {
            await this.api.deleteMessage(item.id);
            this.messages = this.messages.filter(message => message.id !== item.id);
            this.setLeaf(item.parent_id);
        });
    }

    checkLeaf(item) {
        if (!item || this.messages.some(message => message.parent_id === item.id)) {
            throw new Error('有后续分支的消息不能在此编辑或删除');
        }
    }
}
