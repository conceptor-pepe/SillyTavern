/** 账号背景与角色记忆的界面编排，所有持久化都经过 Go API。 */
import { apiClient } from '../api-client.js';
import { $, element, notice } from './view.js';
import { readPortrait } from './create.js';

let background = { image: '', enabled: false };
let draftImage = '';
let memories = [];
let characterID = null;
let editing = null;

/** 每次打开会话都读取账号配置，实现刷新及跨设备同步。 */
export async function loadBackground() {
    background = await apiClient.background();
    applyBackground();
}

/** 背景仅接受服务器验证过的 JPEG 或内置封面，不拼接任意 URL。 */
export function applyBackground() {
    const column = $('.chat-column');
    const image = /^data:image\/jpeg;base64,[A-Za-z0-9+/=]+$/.test(background.image || '') ? background.image : '';
    if (image) column.style.setProperty('--portrait', `url("${image}")`);
    column.classList.toggle('picture-background', Boolean(background.enabled));
    if (image) column.classList.add('has-portrait');
    $('#chat-background').setAttribute('aria-pressed', String(Boolean(background.enabled)));
}

/** 登出时清理图片及记忆正文，避免共享浏览器账号间残留。 */
export function clearSettings() {
    background = { image: '', enabled: false };
    draftImage = '';
    memories = [];
    characterID = null;
    $('#memory-list').replaceChildren();
    $('#memory-form').reset();
    $('#background-preview').removeAttribute('src');
    $('.chat-column').style.removeProperty('--portrait');
}

/** 与主入口共用操作互斥及错误提示，不允许重复写入。 */
export function wireSettings({ act, selected, refreshRole }) {
    $('#chat-background').addEventListener('click', () => void act(async () => {
        background = await apiClient.background();
        background = await apiClient.saveBackground({ ...background, enabled: !background.enabled });
        refreshRole();
        applyBackground();
    }));
    $('#background-settings').addEventListener('click', () => void act(async () => {
        await loadBackground();
        draftImage = background.image || '';
        $('#background-file').value = '';
        $('#background-enabled').checked = background.enabled;
        previewBackground();
        $('#background-dialog').showModal();
    }));
    $('#background-file').addEventListener('change', event => void act(async () => {
        draftImage = await readPortrait(event.target.files[0], 1024);
        $('#background-enabled').checked = true;
        previewBackground();
    }));
    $('#background-clear').addEventListener('click', () => { draftImage = ''; $('#background-file').value = ''; previewBackground(); });
    $('#background-form').addEventListener('submit', event => {
        event.preventDefault();
        void act(async () => {
            background = await apiClient.saveBackground({ image: draftImage, enabled: $('#background-enabled').checked });
            refreshRole();
            applyBackground();
            $('#background-dialog').close();
        });
    });
    wireMemories(act, selected);
}

function previewBackground() {
    $('#background-preview').hidden = !draftImage;
    if (draftImage) $('#background-preview').src = draftImage;
    else $('#background-preview').removeAttribute('src');
}

function resetMemory() {
    editing = null;
    $('#memory-form').reset();
    $('#memory-form-title').textContent = '添加记忆';
}

async function loadMemories() {
    memories = (await apiClient.memories(characterID)).items;
    const rows = memories.map(item => {
        const row = element('article', undefined, 'memory-entry');
        const heading = element('div', undefined, 'memory-entry-heading');
        heading.append(element('strong', item.kind === 'lore' ? '世界书' : '长期记忆'), element('span', !item.enabled ? '已停用' : item.pinned ? '始终记住' : '按需回忆'));
        const edit = element('button', '编辑');
        edit.type = 'button';
        edit.dataset.memoryEdit = item.id;
        const remove = element('button', '删除', 'danger');
        remove.type = 'button';
        remove.dataset.memoryDelete = item.id;
        const actions = element('div', undefined, 'memory-actions');
        actions.append(edit, remove);
        row.append(heading, element('p', item.content), element('small', item.keywords?.join(' · ') || ''), actions);
        return row;
    });
    $('#memory-list').replaceChildren(...rows);
    if (!rows.length) $('#memory-list').append(element('p', '还没有保存记忆。添加重要约定，让新会话也能记住。'));
}

function wireMemories(act, selected) {
    $('#manage-memories').addEventListener('click', () => void act(async () => {
        characterID = selected().id;
        resetMemory();
        await loadMemories();
        $('#character-dialog').close();
        $('#memory-dialog').showModal();
    }));
    $('#memory-reset').addEventListener('click', resetMemory);
    $('#memory-form').addEventListener('submit', event => {
        event.preventDefault();
        const form = event.currentTarget;
        const body = { kind: form.elements.kind.value, content: form.elements.content.value,
            keywords: form.elements.keywords.value.split(/[,，、]/).map(x => x.trim()).filter(Boolean),
            pinned: form.elements.pinned.checked, enabled: form.elements.enabled.checked };
        void act(async () => {
            await apiClient.saveMemory(characterID, body, editing);
            resetMemory();
            await loadMemories();
            notice('记忆已保存，下次回复会使用最新设置。');
        });
    });
    $('#memory-list').addEventListener('click', event => {
        const id = event.target.closest('[data-memory-edit]')?.dataset.memoryEdit;
        if (id) {
            const item = memories.find(x => x.id === id);
            editing = id;
            const form = $('#memory-form');
            for (const key of ['kind', 'content']) form.elements[key].value = item[key];
            form.elements.keywords.value = (item.keywords || []).join('，');
            for (const key of ['enabled', 'pinned']) form.elements[key].checked = item[key];
            $('#memory-form-title').textContent = '编辑记忆';
            form.elements.content.focus();
        }
        const remove = event.target.closest('[data-memory-delete]')?.dataset.memoryDelete;
        if (remove) void act(async () => {
            await apiClient.deleteMemory(characterID, remove);
            if (editing === remove) resetMemory();
            await loadMemories();
        });
    });
}
