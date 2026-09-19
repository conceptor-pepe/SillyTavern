/** 渲染角色、会话和消息；资料使用文本节点，聊天正文使用受限 Markdown。 */
import { messageText } from './text.js';
export const $ = selector => document.querySelector(selector);

/** 创建安全的文本节点，避免角色卡或 AI 回复中的 HTML 执行。 */
export function element(tag, text, className) {
    const node = document.createElement(tag);
    if (text !== undefined) node.textContent = text;
    if (className) node.className = className;
    return node;
}

/** 工具按钮沿用已有图标库，并同时提供可访问名称和悬停提示。 */
function tool(action, icon, label, id = '') {
    const button = element('button', undefined, 'icon');
    button.type = 'button';
    button.dataset.action = action;
    button.dataset.id = id;
    button.title = label;
    button.setAttribute('aria-label', label);
    const image = element('i', undefined, `fa-solid fa-${icon}`);
    image.setAttribute('aria-hidden', 'true');
    button.append(image);
    return button;
}

export function notice(message = '') {
    $('#notice').textContent = message;
    $('#notice').hidden = !message;
}

export function showPage(chat) {
    $('#create-view').hidden = true;
    $('#story-view').hidden = true;
    document.body.classList.remove('in-create');
    $('#chat-view').hidden = !chat;
    $('#library-view').hidden = chat;
    document.body.classList.toggle('in-chat', chat);
    document.querySelectorAll('.primary-nav button').forEach(button => {
        button.title = button.textContent.trim();
        button.setAttribute('aria-label', button.title);
    });
    $('#library').setAttribute('aria-current', chat ? 'false' : 'page');
    $('#create-character').setAttribute('aria-current', 'false');
    $('#stories-nav').setAttribute('aria-current', 'false');
}

export function renderCharacters(items, favorites) {
    const search = $('#search').value.trim().toLocaleLowerCase();
    renderFilters(items);
    const tag = $('#tag-filters').dataset.selected || '';
    const gender = $('#character-gender').value;
    const filtered = items.filter(item => `${item.name} ${item.description} ${item.tags.join(' ')}`.toLocaleLowerCase().includes(search))
        .filter(item => !$('#only-favorites').checked || favorites.includes(item.id))
        .filter(item => !gender || item.gender === gender)
        .filter(item => !tag || item.tags.includes(tag));
    filtered.sort((a, b) => $('#character-sort').value === 'name' ? a.name.localeCompare(b.name, 'zh-CN') : (BigInt(a.id) > BigInt(b.id) ? -1 : 1));
    $('#all-characters').classList.toggle('active', !$('#only-favorites').checked);
    $('#saved-characters').classList.toggle('active', $('#only-favorites').checked);
    const nodes = filtered.map(item => characterCard(item, favorites.includes(item.id)));
    $('#character-count').textContent = `${filtered.length} 个角色`;
    $('#character-list').replaceChildren(...(nodes.length ? nodes : [element('p', items.length ? '没有找到符合条件的角色' : '还没有角色，创建你的第一位聊天伙伴吧。', 'empty')]));
}

/** 封面仅允许本站静态资源和后端校验过的 JPEG 数据，避免外部追踪图片。 */
export function portrait(item) {
    const value = item?.portrait || '';
    return /^data:image\/jpeg;base64,[A-Za-z0-9+/=]+$/.test(value) || /^\/img\/characters\/[a-z-]+\.jpg$/.test(value) ? value : '/img/ai4.png';
}

/** 卡片图片固定比例，资料长度不会改变封面或操作区位置。 */
function characterCard(item, favorite) {
        const button = element('button', undefined, 'character');
        button.dataset.character = item.id;
        const image = element('img');
        image.src = portrait(item);
        image.alt = '';
        image.loading = 'lazy';
        image.width = 400;
        image.height = 500;
        const copy = element('div', undefined, 'character-copy');
        copy.append(element('h2', item.name), element('p', item.description || item.first_message || '随时聊聊你的想法。'));
        const tags = element('div', undefined, 'tags');
        tags.append(...item.tags.map(tag => element('span', tag, 'tag')));
        copy.append(tags);
        const footer = element('span', undefined, 'character-link');
        footer.append(element('span', '开始聊天'), element('i', undefined, `fa-solid fa-${favorite ? 'heart' : 'arrow-right'}`));
        button.append(image, copy, footer);
        return button;
}

/** 只呈现资料中实际存在的标签，筛选值不依赖猜测角色属性。 */
function renderFilters(items) {
    const nav = $('#tag-filters');
    const tags = [...new Set(items.flatMap(item => item.tags))];
    if (!tags.includes(nav.dataset.selected)) nav.dataset.selected = '';
    nav.replaceChildren(...['', ...tags].map(tag => {
        const button = element('button', tag || '全部');
        button.dataset.tag = tag;
        button.setAttribute('aria-pressed', String(tag === (nav.dataset.selected || '')));
        return button;
    }));
}

export function renderChats(items, selected, characters = []) {
    const nodes = items.map(item => {
        const row = element('div', undefined, 'chat-row');
        const button = element('button', undefined, 'chat-link');
        const avatar = element('img', undefined, 'chat-list-avatar');
        avatar.src = portrait(characters.find(role => role.id === String(item.character_id)));
        avatar.alt = '';
        const copy = element('span', undefined, 'chat-list-copy');
        const heading = element('span', undefined, 'chat-list-heading');
        heading.append(element('strong', item.title || '未命名会话'));
        if (item.last_msg_at) {
            const date = new Date(Number(item.last_msg_at) * 1000);
            if (Number.isFinite(date.getTime())) heading.append(element('time', date.toLocaleDateString('zh-CN', {month:'2-digit',day:'2-digit'})));
        }
        copy.append(heading, element('span', item.preview || '打开聊天', 'chat-preview'));
        button.append(avatar, copy);
        button.dataset.chat = item.id;
        if (item.id === selected) button.setAttribute('aria-current', 'page');
        const remove = tool('delete-chat', 'trash', `删除会话：${item.title || '未命名会话'}`, item.id);
        remove.dataset.deleteChat = item.id;
        row.append(button, remove);
        return row;
    });
    $('#chat-list').replaceChildren(...(nodes.length ? nodes : [element('p', '暂无会话', 'empty')]));
}

export function renderDetail(item, favorite) {
    $('#detail-name').textContent = item.name;
    $('.detail-avatar').src = portrait(item);
    const fields = [['性别', ({ female: '女性', male: '男性', other: '非二元' })[item.gender]], ['年龄', item.age], ['简介', item.description], ['性格', item.personality], ['场景', item.scenario], ['开场白', item.first_message], ['示例对话', item.message_sample]];
    $('#detail-body').replaceChildren(...fields.filter(([, value]) => value).flatMap(([label, value]) =>
        [element('h3', label), element('p', value)]));
    $('#detail-body').scrollTop = 0;
    $('#favorite').setAttribute('aria-pressed', String(favorite));
}

/** 同步聊天页头像、资料和开场白，历史会话使用其绑定角色而非名称猜测。 */
export function renderRole(item) {
    $('.chat-column').style.setProperty('--portrait', `url("${portrait(item)}")`);
    $('.chat-column').classList.toggle('has-portrait', portrait(item) !== '/img/ai4.png');
    $('#chat-avatar').src = portrait(item);
    $('#role-portrait').src = portrait(item);
    $('#role-name').textContent = item?.name || '聊天伙伴';
    $('#role-description').textContent = item?.description || item?.personality || '随时聊聊你的想法。';
    $('#role-tags').replaceChildren(...(item?.tags || []).map(tag => element('span', tag, 'tag')));
}

/** 生成期间保留阅读位置；只有原本接近底部时才自动跟随新文本。 */
export function renderSession(session) {
    if (!session.chat) return;
    $('#chat-title').textContent = session.chat.title || '未命名会话';
    $('#chat-state').textContent = session.active ? (session.active.stopped ? '正在停止' : '正在输入…') : (session.busy ? '正在加载' : '陪你聊聊');
    const branch = $('#branch');
    const signature = JSON.stringify(session.messages.map(item => [item.id, item.content.slice(0, 32)])) + `:${session.leaf}`;
    if (branch.dataset.signature !== signature) {
        branch.replaceChildren(new Option('新分支', ''));
        session.messages.forEach(item => branch.add(new Option(`${item.role === 'user' ? '我' : 'AI'} · ${item.content.slice(0, 32)}`, item.id)));
        branch.value = session.leaf ?? '';
        branch.dataset.signature = signature;
    }
    const messages = $('#messages');
    const follow = messages.scrollHeight - messages.scrollTop - messages.clientHeight < 100;
    const nodes = session.path.map(item => renderMessage(item, session));
    for (const [index, content] of session.active?.drafts ?? []) {
        const draft = element('article', undefined, 'message draft');
        draft.append(element('header', session.chat.title || '聊天伙伴'), element('div', content, 'content'));
        nodes.push(draft);
    }
    if (!nodes.length) {
        const greeting = session.character?.first_message;
        const welcome = element('div', undefined, greeting ? 'message greeting' : 'chat-welcome');
        if (greeting) {
            const content = element('div', undefined, 'content');
            content.append(messageText(greeting));
            welcome.append(element('header', session.chat.title), content);
        } else {
            welcome.append(element('h2', session.chat.title || '你好'), element('p', '今天，有什么想分享的？'));
        }
        nodes.push(welcome);
    }
    messages.replaceChildren(...nodes);
    if (follow) messages.scrollTop = messages.scrollHeight;
}

function renderMessage(item, session) {
    const article = element('article', undefined, 'message');
    article.dataset.role = item.role;
    article.dataset.message = item.id;
    const content = element('div', undefined, 'content');
    content.append(messageText(item.content));
    article.append(messageHeader(item, session), content);
    const actions = element('div', undefined, 'message-actions');
    const label = item.role === 'user' ? '生成回复' : '重新生成';
    actions.append(tool('retry', 'rotate', label, item.id));
    if (item.role === 'assistant' && !item.source_variant_id) actions.append(tool('variants', 'layer-group', '查看候选', item.id));
    const leaf = item.id === session.leaf && !session.messages.some(message => message.parent_id === item.id);
    if (leaf) {
        const menu = element('details', undefined, 'message-menu');
        const summary = element('summary', undefined, 'icon');
        summary.title = '更多消息操作';
        summary.setAttribute('aria-label', '更多消息操作');
        summary.append(element('i', undefined, 'fa-solid fa-ellipsis'));
        const panel = element('div', undefined, 'message-menu-items');
        if (item.role === 'user') panel.append(tool('edit', 'pen', '编辑消息', item.id));
        if (item.role === 'assistant') panel.append(tool('revise', 'pen-to-square', '编辑 AI 回复并创建分支', item.id));
        panel.append(tool('remove', 'trash', '删除消息', item.id));
        menu.append(summary, panel);
        actions.append(menu);
    }
    actions.querySelectorAll('button').forEach(button => { button.disabled = session.busy; });
    article.append(actions);
    if (session.variants.has(item.id)) article.append(renderVariants(item.id, session));
    return article;
}

/** 头像与真实时间标识消息归属，缺失历史时间时不伪造当前时间。 */
function messageHeader(item, session) {
    const header = element('header');
    const user = item.role === 'user';
    const avatar = element(user ? 'span' : 'img', undefined, 'message-avatar');
    if (user) avatar.append(element('i', undefined, 'fa-solid fa-user'));
    else { avatar.src = portrait(session.character); avatar.alt = ''; }
    header.append(avatar, element('span', user ? '我' : session.chat.title || '聊天伙伴'));
    const date = new Date(item.created_at);
    if (Number.isFinite(date.getTime()) && date.getFullYear() > 1) {
        const time = element('time', date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }));
        time.dateTime = date.toISOString();
        time.title = date.toLocaleString('zh-CN');
        header.append(time);
    }
    return header;
}

function renderVariants(messageID, session) {
    const section = element('div', undefined, 'variants');
    const variants = session.variants.get(messageID);
    if (!variants.length) section.append(element('p', '没有其他候选', 'empty'));
    for (const item of variants) {
        const row = element('div', undefined, 'variant');
        const button = tool('select', 'check', `选择候选 ${item.variant_no + 1}`, messageID);
        button.dataset.variant = item.id;
        button.disabled = session.busy;
        row.append(element('p', item.content), button);
        section.append(row);
    }
    return section;
}
