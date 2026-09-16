/** 适配迁移期响应格式，并集中处理分页、分支和用户隔离的本地偏好。 */

/** 拒绝已经丢失精度的数字 ID，不把错误编号发送给后端。 */
export function idText(value) {
    if (typeof value === 'number' && !Number.isSafeInteger(value)) throw new Error('接口编号超出安全范围');
    const text = String(value ?? '');
    if (!/^[1-9]\d*$/.test(text)) throw new Error('接口返回了无效编号');
    return text;
}

/** 兼容当前角色接口的大写字段，页面内部统一使用小写。 */
export function characterView(item) {
    return {
        id: idText(item.id ?? item.ID),
        name: item.name ?? item.Name ?? '',
        description: item.description ?? item.Description ?? '',
        personality: item.personality ?? item.Personality ?? '',
        scenario: item.scenario ?? item.Scenario ?? '',
        first_message: item.first_message ?? item.FirstMessage ?? '',
        portrait: item.portrait ?? item.Portrait ?? '',
        tags: item.tags ?? item.Tags ?? [],
        gender: item.gender ?? item.Gender ?? '',
        age: item.age ?? item.Age ?? '',
        message_sample: item.message_sample ?? item.MessageSample ?? '',
    };
}

/** 完整读取分页，避免只把最早一页误认为整个会话。 */
export async function allPages(load) {
    const items = [];
    for (let page = 1; ; page++) {
        const result = await load(`?page=${page}&size=100`);
        const batch = result.items ?? [];
        items.push(...batch);
        if (globalThis.BigInt(items.length) >= globalThis.BigInt(result.total ?? 0)) return items;
        if (!batch.length) throw new Error('列表读取不完整，请重新加载');
    }
}

/** 只沿父节点回溯当前分支，禁止把兄弟回复一起送入可见历史。 */
export function branchPath(messages, leaf) {
    if (!leaf) return [];
    const index = new Map(messages.map(item => [item.id, item]));
    const result = [];
    const seen = new Set();
    let id = leaf;
    while (id) {
        const item = index.get(id);
        if (!item || seen.has(id)) throw new Error('消息分支不完整，请选择其他分支');
        seen.add(id);
        result.unshift(item);
        id = item.parent_id;
    }
    return result;
}

/** 本地只保存编号和界面偏好，不保存消息正文、密码或 Provider 密钥。 */
export function preferences(userID, storage = globalThis.localStorage) {
    const prefix = `go-chat:${userID}:`;
    return {
        get(key, fallback = null) {
            try {
                return JSON.parse(storage.getItem(prefix + key)) ?? fallback;
            } catch {
                return fallback;
            }
        },
        set(key, value) {
            try {
                storage.setItem(prefix + key, JSON.stringify(value));
            } catch {
                // 隐私模式禁用存储时，当前页面仍可正常聊天。
            }
        },
    };
}
