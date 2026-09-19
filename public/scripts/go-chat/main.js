/** 独立 Go 聊天页面入口，仅装配 API、会话状态和 UI 事件。 */
import { apiClient } from '../api-client.js';
import { allPages, characterView, preferences, idText } from './data.js';
import { ChatSession } from './session.js';
import { $, notice, renderCharacters, renderChats, renderDetail, renderSession, renderRole, showPage } from './view.js';
import { showCreate, characterBody, resetCreate } from './create.js';
import { wireSettings, loadBackground, applyBackground, clearSettings } from './settings.js';
import { wireStories } from './stories.js';
import { wireRelationship } from './relationship.js';
import { wireMemoryCandidates } from './memory-candidates.js';

const state = { user: null, prefs: null, session: null, characters: [], chats: [], selected: null, register: false, pending: false };

/** 界面层统一提示失败，401 时销毁当前用户的内存消息。 */
function report(error) {
    if (error.status === 401 && state.user) signedOut();
    notice(error.message || '请求失败，请稍后重试');
}

async function act(action) {
    if (state.pending) return;
    state.pending = true;
    notice();
    controls();
    try {
        await action();
    } catch (error) {
        report(error);
    } finally {
        state.pending = false;
        controls();
    }
}

function controls() {
    const busy = state.pending || Boolean(state.session?.busy);
    for (const selector of ['#auth-submit', '#auth-mode', '#logout', '#library', '#stories-nav', '#refresh-chats', '#create-character', '#back', '#new-chat', '#delete-chat', '#start-chat', '#branch', '#send', '#message', '#suggest-replies', '#create-submit', '#create-back', '#create-cancel', '#recent-chats', '#favorites-nav', '#edit-character', '#delete-character', '#manage-memories', '#relationship-settings', '#memory-candidates', '#background-settings', '#chat-background']) {
        $(selector).disabled = busy;
        if (['#send', '#message', '#new-chat'].includes(selector) && state.session?.chat && !state.session.character) $(selector).disabled = true;
    }
    document.querySelectorAll('#background-form input, #background-form button, #memory-form input, #memory-form textarea, #memory-form select, #memory-form button, #memory-list button').forEach(input => { input.disabled = busy; });
    document.querySelectorAll('#relationship-dialog input, #relationship-dialog textarea, #relationship-dialog select, #relationship-dialog button').forEach(input => { input.disabled = busy; });
    document.querySelectorAll('#memory-candidate-dialog button').forEach(input => { input.disabled = busy; });
    document.querySelectorAll('#story-view button, #story-dialog button, #story-dialog select, #story-editor input, #story-editor textarea, #story-editor select, #story-editor button, #persona-dialog input, #persona-dialog textarea, #persona-dialog button').forEach(input => { input.disabled = busy; });
    document.querySelectorAll('#reply-suggestions button').forEach(button => { button.disabled = busy; });
    $('#create-character-form').querySelectorAll('input,textarea,select').forEach(input => { input.disabled = busy; });
    $('#stop').hidden = !state.session?.active;
    $('#stop').disabled = Boolean(state.session?.active?.stopped);
    $('#send').hidden = Boolean(state.session?.active);
    $('#chat-list').querySelectorAll('button').forEach(button => { button.disabled = busy; });
}

function changed() {
    renderSession(state.session);
    controls();
}

async function signedIn(result) {
    const user = result.user ?? result;
    state.user = user;
    state.prefs = preferences(idText(user.id ?? user.ID));
    state.session = new ChatSession({ prefs: state.prefs, changed });
    $('#user-name').textContent = user.name || user.Name || user.handle || user.Handle;
    $('#auth').hidden = true;
    $('#workspace').hidden = false;
    $('.account').hidden = false;
    showPage(false);
    await loadCatalog();
    const saved = state.prefs.get('chat');
    if (saved && state.chats.some(chat => chat.id === saved)) await openChat(saved);
}

function signedOut() {
    state.session = null;
    state.user = null;
    state.prefs = null;
    state.characters = [];
    state.chats = [];
    state.selected = null;
    resetCreate();
    clearSettings();
    $('#messages').replaceChildren();
    $('#message').value = '';
    $('#chat-list').replaceChildren();
    $('#character-list').replaceChildren();
    document.querySelectorAll('dialog[open]').forEach(dialog => dialog.close());
    $('#auth').hidden = false;
    $('#workspace').hidden = true;
    $('.account').hidden = true;
    document.body.classList.remove('in-chat');
    document.body.classList.remove('in-create');
}

async function loadCatalog() {
    const [characters, chats] = await Promise.all([
        allPages(query => apiClient.characters(query)),
        allPages(query => apiClient.chats(query)),
    ]);
    state.characters = characters.map(characterView);
    state.chats = chats;
    await loadPreviews(chats);
    renderCharacters(state.characters, state.prefs.get('favorites', []));
    for (const chat of chats) {
        if (chat.id === state.session.chat?.id) chat.preview = state.session.path.at(-1)?.content;
    }
    renderChats(chats, state.session.chat?.id, state.characters);
}

/** 每批最多四个查询，只读取末条消息，不下载所有会话正文。 */
async function loadPreviews(chats) {
    for (let offset = 0; offset < chats.length; offset += 4) {
        await Promise.all(chats.slice(offset, offset + 4).map(async chat => {
            try {
                const first = await apiClient.messages(chat.id, '?page=1&size=1');
                const total = Number(first.total || 0);
                const page = total > 1 ? await apiClient.messages(chat.id, `?page=${total}&size=1`) : first;
                chat.preview = page.items?.[0]?.content || '暂无消息';
            } catch {
                chat.preview = '摘要暂不可用';
            }
        }));
    }
}

async function openChat(id) {
    await state.session.open(id);
    const definition = state.session.story?.version?.definition;
    const cast = definition?.cast?.[0];
    const character = state.session.chat.mode === 'story' ? {
        id: cast?.id || 'story-cast', name: cast?.name || state.session.chat.title,
        description: cast?.description || definition?.hook || '', personality: cast?.personality || '',
        scenario: definition?.world || '', first_message: '', portrait: cast?.portrait || definition?.cover || '',
        tags: definition?.tags || [], gender: cast?.gender || '', age: cast?.age || '', message_sample: definition?.examples || '',
    } : state.characters.find(item => item.id === String(state.session.chat.character_id));
    state.session.character = character;
    $('#relationship-settings').hidden = !state.session.relationship;
    $('#role-scope').textContent = state.session.chat.mode === 'story' ? '故事角色' : '我的角色';
    $('#reply-suggestions').hidden = true;
    $('#reply-suggestions').replaceChildren();
    renderRole(character);
    await loadBackground();
    renderSession(state.session);
    showPage(true);
    const current = state.chats.find(chat => chat.id === id);
    if (current) current.preview = state.session.path.at(-1)?.content;
    renderChats(state.chats, id, state.characters);
    $('#messages').scrollTop = $('#messages').scrollHeight;
}

async function createChat(id, title) {
    const number = Number(id);
    if (!Number.isSafeInteger(number)) throw new Error('当前创建会话接口尚不支持此角色编号');
    const chat = await apiClient.createChat({ character_id: number, title });
    state.chats.unshift(chat);
    $('#character-dialog').close();
    await openChat(chat.id);
}

async function createCharacter(body) {
    const character = characterView(await apiClient.createCharacter(body));
    state.characters.unshift(character);
    resetCreate();
    renderCharacters(state.characters, state.prefs.get('favorites', []));
    state.selected = character;
    showPage(false);
    renderDetail(character, false);
    $('#character-dialog').showModal();
    await createChat(character.id, character.name);
}

function options() {
    return { n: 1 };
}

/** 删除必须显式确认，Esc 与取消按钮均不执行写操作。 */
function confirmDelete(title) {
    const dialog = $('#confirm-dialog');
    $('#confirm-title').textContent = title;
    dialog.returnValue = 'cancel';
    dialog.showModal();
    return new Promise(resolve => dialog.addEventListener('close', () => resolve(dialog.returnValue === 'confirm'), { once: true }));
}

wireStories({ act, openChat, refreshChats: loadCatalog, confirmDelete });

$('#auth-form').addEventListener('submit', event => {
    event.preventDefault();
    void act(async () => {
        const body = Object.fromEntries(new FormData(event.currentTarget));
        const result = await (state.register ? apiClient.register(body) : apiClient.login(body));
        $('#auth-form').reset();
        await signedIn(result);
    });
});

$('#auth-mode').addEventListener('click', () => {
    state.register = !state.register;
    $('#auth h1').textContent = state.register ? '创建账号' : '欢迎回来';
    $('#name-field').hidden = !state.register;
    $('#auth-submit').textContent = state.register ? '注册' : '登录';
    $('#auth-mode').textContent = state.register ? '已有账号，登录' : '创建账号';
    $('#auth-form [name="password"]').autocomplete = state.register ? 'new-password' : 'current-password';
    notice();
});

$('#logout').addEventListener('click', () => void act(async () => {
    await apiClient.logout();
    signedOut();
}));

$('#library').addEventListener('click', () => void act(async () => { showPage(false); await loadCatalog(); }));
$('#create-character').addEventListener('click', () => showCreate());
$('#create-back').addEventListener('click', () => showPage(false));
$('#create-cancel').addEventListener('click', () => showPage(false));
$('#recent-chats').addEventListener('click', () => void act(async () => {
    await loadCatalog();
    if (state.chats.length) await openChat(state.chats[0].id);
    else { showPage(false); notice('还没有聊天记录，选择一位角色开始聊天吧。'); }
}));
/** 收藏入口与列表筛选共用同一个状态。 */
function showFavorites(value) {
    showPage(false);
    $('#only-favorites').checked = value;
    renderCharacters(state.characters, state.prefs.get('favorites', []));
}
$('#favorites-nav').addEventListener('click', () => showFavorites(true));
$('#saved-characters').addEventListener('click', () => showFavorites(true));
$('#all-characters').addEventListener('click', () => showFavorites(false));
$('#tag-filters').addEventListener('click', event => {
    const button = event.target.closest('[data-tag]');
    if (!button) return;
    $('#tag-filters').dataset.selected = button.dataset.tag;
    renderCharacters(state.characters, state.prefs.get('favorites', []));
});
$('#back').addEventListener('click', () => showPage(false));
$('#refresh-chats').addEventListener('click', () => void act(loadCatalog));
for (const selector of ['#search', '#only-favorites', '#character-sort', '#character-gender']) {
    $(selector).addEventListener('input', () => renderCharacters(state.characters, state.prefs.get('favorites', [])));
}

/** 聊天页与窄屏入口复用同一角色详情，不展示内部模型配置。 */
function showChatRole() {
    state.selected = state.session.character;
    if (!state.selected) return;
    renderDetail(state.selected, state.prefs.get('favorites', []).includes(state.selected.id));
    const story = state.session.chat?.mode === 'story';
    for (const selector of ['#edit-character', '#delete-character', '#manage-memories', '#favorite', '#start-chat']) $(selector).hidden = story;
    $('#character-dialog').showModal();
}
$('#chat-info').addEventListener('click', showChatRole);
$('#role-detail').addEventListener('click', showChatRole);
$('#compose-role').addEventListener('click', showChatRole);
$('#compose-history').addEventListener('click', () => {
    const history = $('.chat-settings');
    history.open = !history.open;
    if (history.open) $('#branch').focus();
});

$('#character-list').addEventListener('click', event => {
    const button = event.target.closest('[data-character]');
    if (!button) return;
    void act(async () => {
        const result = await apiClient.character(button.dataset.character);
        state.selected = characterView(result.character ?? result);
        for (const selector of ['#edit-character', '#delete-character', '#manage-memories', '#favorite', '#start-chat']) $(selector).hidden = false;
        renderDetail(state.selected, state.prefs.get('favorites', []).includes(state.selected.id));
        $('#character-dialog').showModal();
    });
});

$('#favorite').addEventListener('click', () => {
    const items = state.prefs.get('favorites', []);
    const id = state.selected.id;
    const next = items.includes(id) ? items.filter(item => item !== id) : [...items, id];
    state.prefs.set('favorites', next);
    renderDetail(state.selected, next.includes(id));
    renderCharacters(state.characters, next);
});

$('#start-chat').addEventListener('click', () => void act(() => createChat(state.selected.id, state.selected.name)));
$('#create-character-form').addEventListener('submit', event => {
    event.preventDefault();
    const form = event.currentTarget;
    try {
        const body = characterBody(form);
        void act(async () => {
            const id = form.dataset.editId;
            if (!id) { await createCharacter(body); return; }
            state.selected = characterView(await apiClient.updateCharacter(id, body));
            resetCreate();
            await loadCatalog();
            if (state.session.chat?.character_id === id) {
                state.session.character = state.selected; renderRole(state.selected); applyBackground();
            }
            showPage(false);
            renderDetail(state.selected, state.prefs.get('favorites', []).includes(id));
            $('#character-dialog').showModal();
        });
    } catch (error) { report(error); }
});
$('#new-chat').addEventListener('click', () => void act(() => {
    if (state.session.chat.mode === 'story') throw new Error('请从故事详情重新开始一段剧情');
    return createChat(state.session.chat.character_id, state.session.chat.title);
}));
$('#chat-list').addEventListener('click', event => {
    const remove = event.target.closest('[data-delete-chat]');
    if (remove) { void act(() => deleteChat(remove.dataset.deleteChat)); return; }
    const id = event.target.closest('[data-chat]')?.dataset.chat;
    if (id) void act(() => openChat(id));
});

/** 列表和当前会话共用删除流程，删除其他会话不打断正在查看的内容。 */
async function deleteChat(id) {
    const chat = state.chats.find(item => item.id === id);
    if (!await confirmDelete(`删除「${chat?.title || '未命名会话'}」及全部消息？`)) return;
    await apiClient.deleteChat(id);
    state.prefs.set(`leaf:${id}`, null);
    if (state.session.chat?.id === id) {
        state.prefs.set('chat', null);
        state.session = new ChatSession({ prefs: state.prefs, changed });
        $('#messages').replaceChildren();
        $('#message').value = '';
        showPage(false);
    }
    await loadCatalog();
}
$('#delete-chat').addEventListener('click', () => void act(() => deleteChat(state.session.chat.id)));

wireSettings({ act, selected: () => state.selected, refreshRole: () => renderRole(state.session.character) });
wireRelationship({ act, session: () => state.session });
wireMemoryCandidates({ act, session: () => state.session });

$('#edit-character').addEventListener('click', () => {
    $('#character-dialog').close();
    showCreate(state.selected);
});
$('#delete-character').addEventListener('click', () => void act(async () => {
    const item = state.selected;
    if (!await confirmDelete(`删除角色「${item.name}」？聊天历史会保留，但不能继续生成。`)) return;
    await apiClient.deleteCharacter(item.id);
    $('#character-dialog').close();
    state.prefs.set('favorites', state.prefs.get('favorites', []).filter(id => id !== item.id));
    await loadCatalog();
    showPage(false);
}));

$('#branch').addEventListener('change', () => {
    try {
        state.session.choose($('#branch').value || null);
    } catch (error) {
        $('#branch').value = state.session.leaf ?? '';
        report(error);
    }
});

$('#composer').addEventListener('submit', event => {
    event.preventDefault();
    void act(async () => {
        const before = state.session.messages.length;
        try {
            await state.session.send($('#message').value, options());
        } finally {
            if (state.session.messages.length > before) $('#message').value = '';
        }
        await loadCatalog();
    });
});

$('#message').addEventListener('keydown', event => {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
        event.preventDefault();
        $('#composer').requestSubmit();
    }
});
$('#stop').addEventListener('click', () => void state.session.stop().catch(report));

$('#messages').addEventListener('click', event => {
    const button = event.target.closest('[data-action]');
    if (!button) return;
    void act(async () => {
        const { action, id, variant } = button.dataset;
        if (action === 'retry') await state.session.retry(id, options());
        if (action === 'variants') await state.session.loadVariants(id);
        if (action === 'select') await state.session.select(id, variant);
        if (action === 'remove' && await confirmDelete('删除这条消息？')) await state.session.remove();
        if (action === 'edit') {
            $('#edit-form').dataset.mode = 'user';
            $('#edit-content').value = state.session.path.at(-1).content;
            $('#edit-dialog').showModal();
        }
        if (action === 'revise') {
            $('#edit-form').dataset.mode = 'assistant';
            $('#edit-content').value = state.session.path.at(-1).content;
            $('#edit-dialog h2').textContent = '编辑 AI 回复并创建分支';
            $('#edit-dialog').showModal();
        }
    });
});

$('#edit-form').addEventListener('submit', event => {
    event.preventDefault();
    void act(async () => {
        if ($('#edit-form').dataset.mode === 'assistant') await state.session.revise($('#edit-content').value);
        else await state.session.edit($('#edit-content').value);
        $('#edit-dialog').close();
        $('#edit-dialog h2').textContent = '编辑消息';
        delete $('#edit-form').dataset.mode;
    });
});

$('#suggest-replies').addEventListener('click', () => void act(async () => {
    const result = await state.session.suggestions();
    const panel = $('#reply-suggestions');
    panel.replaceChildren(...result.items.map(text => {
        const button = document.createElement('button');
        button.type = 'button';
        button.textContent = text;
        return button;
    }));
    panel.hidden = false;
}));
$('#reply-suggestions').addEventListener('click', event => {
    const button = event.target.closest('button');
    if (!button) return;
    $('#message').value = button.textContent;
    $('#reply-suggestions').hidden = true;
    $('#message').focus();
});
document.querySelectorAll('[data-close]').forEach(button => button.addEventListener('click', () => button.closest('dialog').close()));

window.addEventListener('beforeunload', event => {
    if (!state.session?.busy) return;
    event.preventDefault();
    event.returnValue = '';
});

void act(async () => {
    try {
        await signedIn(await apiClient.me());
    } catch (error) {
        signedOut();
        if (error.status !== 401) throw error;
    }
});
