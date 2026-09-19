/** H5 故事与玩家身份入口，复用现有 API 和主题，不保存私有正文到浏览器。 */
import { apiClient } from '../api-client.js';
import { $, element, portrait } from './view.js';
import { readPortrait } from './create.js';
import { characterView } from './data.js';
import { createWizard, wireChoiceFields } from './wizard.js';

export function wireStories({ act, openChat, refreshChats, confirmDelete }) {
    const model = { stories: [], publicStories: [], personas: [], characters: [], selected: null, scope: 'public', view: 'public' };
    let storyCover = '';
    let personaAvatar = '';
    const storyForm = $('#story-form');
    const storyWizard = createWizard(storyForm, { changed: updateStoryReview });
    wireChoiceFields(storyForm);

    function updateStoryReview() {
        const value = name => storyForm.elements[name].value.trim();
        const tags = value('tags').split(/[,，、]/).map(item => item.trim()).filter(Boolean).slice(0, 6);
        $('#story-review-cover').src = storyCover || '/img/ai4.png';
        $('#story-review-title').textContent = value('title') || '未命名故事';
        $('#story-review-hook').textContent = value('hook') || '还没有填写一句话简介';
        $('#story-review-tags').textContent = tags.length ? tags.join(' · ') : '私有草稿';
        const companion = storyForm.elements.companion_id.selectedOptions[0]?.text || '仅用于这个故事';
        const row = (label, content) => [element('dt', label), element('dd', content || '未填写')];
        $('#story-review-summary').replaceChildren(
            ...row('世界', value('world')),
            ...row('主角', value('character_name')),
            ...row('长期角色', companion),
            ...row('开场', value('opening_dialogue') || value('opening_narration')),
        );
    }

    function showStoryView() {
        $('#library-view').hidden = true;
        $('#chat-view').hidden = true;
        $('#create-view').hidden = true;
        $('#story-view').hidden = false;
        document.body.classList.remove('in-chat', 'in-create');
        document.querySelectorAll('.primary-nav button').forEach(button => button.setAttribute('aria-current', 'false'));
        $('#stories-nav').setAttribute('aria-current', 'page');
    }

    async function load() {
        const [stories, publicStories, personas, characters] = await Promise.all([
            apiClient.stories('?page=1'), apiClient.publicStories('?page=1'), apiClient.personas('?page=1'), apiClient.characters('?page=1&size=100'),
        ]);
        model.stories = stories.items || [];
        model.publicStories = publicStories.items || [];
        model.personas = personas.items || [];
        model.characters = (characters.items || []).map(characterView);
        renderCurrentStories();
        renderPersonas(model.personas);
    }

    function renderCurrentStories() {
        const source = model.view === 'public' ? model.publicStories : model.stories;
        const query = $('#story-search').value.trim().toLocaleLowerCase();
        const items = source.filter(item => {
            const definition = item.definition;
            return `${definition.title} ${definition.hook} ${(definition.tags || []).join(' ')} ${item.author_name || ''}`.toLocaleLowerCase().includes(query);
        });
        renderStories(items, model.view);
        $('#discover-stories').classList.toggle('active', model.view === 'public');
        $('#my-stories').classList.toggle('active', model.view === 'mine');
        $('#story-page-title').textContent = model.view === 'public' ? '发现故事' : '我的故事';
        $('#story-page-copy').textContent = model.view === 'public' ? '选择一个世界，用自己的身份进入剧情。' : '编辑、发布并试玩你创作的互动故事。';
    }

    function renderStories(items, scope) {
        const nodes = items.map(item => {
            const definition = item.definition;
            const card = element('button', undefined, 'character story-card');
            card.type = 'button';
            card.dataset.story = scope === 'public' ? item.story_id : item.id;
            card.dataset.storyScope = scope;
            const image = element('img', undefined, 'story-cover');
            image.src = portrait({ portrait: definition.cover || definition.cast?.[0]?.portrait });
            image.alt = '';
            const copy = element('div', undefined, 'character-copy');
            copy.append(element('h2', definition.title), element('p', definition.hook || definition.world || '进入这个世界，写下属于你的剧情。'));
            const tags = element('div', undefined, 'tags');
            tags.append(...(definition.tags || []).map(tag => element('span', tag, 'tag')));
            const meta = scope === 'public' ? `作者 ${item.author_name || `@${item.author_handle}`}` :
                `${item.published_version_id ? '已发布' : '未发布'} · 草稿修订 ${item.revision}`;
            copy.append(tags, element('div', meta, 'story-meta'));
            const footer = element('span', undefined, 'character-link');
            footer.append(element('span', '查看故事'), element('i', undefined, 'fa-solid fa-arrow-right'));
            card.append(image, copy, footer);
            return card;
        });
        const empty = model.view === 'public' ? '还没有公开故事。切换到“我的故事”发布第一部作品吧。' : '还没有故事。先创建一个世界，再选择身份进入剧情。';
        $('#story-list').replaceChildren(...(nodes.length ? nodes : [element('p', empty, 'empty')]));
    }

    function renderPersonas(items) {
        const nodes = items.map(item => {
            const row = element('article', undefined, 'persona-item');
            const avatar = element('img');
            avatar.src = portrait({ portrait: item.avatar });
            avatar.alt = '';
            const copy = element('div');
            copy.append(element('strong', item.name), element('span', item.description || '未填写身份描述'));
            const edit = element('button', '编辑');
            edit.type = 'button';
            edit.dataset.personaEdit = item.id;
            const remove = element('button', '删除', 'danger');
            remove.type = 'button';
            remove.dataset.personaDelete = item.id;
            row.append(avatar, copy, edit, remove);
            return row;
        });
        $('#persona-list').replaceChildren(...(nodes.length ? nodes : [element('p', '创建一个身份后，就可以进入故事。', 'empty')]));
        fillPersonaSelect();
    }

    function fillPersonaSelect() {
        const select = $('#story-persona');
        const current = select.value;
        select.replaceChildren(...model.personas.map(item => new Option(`${item.name}${item.description ? ` · ${item.description.slice(0, 24)}` : ''}`, item.id)));
        if (model.personas.some(item => item.id === current)) select.value = current;
        $('#start-story').disabled = !model.personas.length;
    }

    function openStory(item, scope) {
        model.selected = item;
        model.scope = scope;
        const definition = item.definition;
        $('#story-detail-title').textContent = definition.title;
        $('#story-detail-meta').textContent = scope === 'public' ? `作者：${item.author_name || `@${item.author_handle}`}` :
            (item.published_version_id ? '当前已有公开版本；继续编辑草稿不会改变已发布内容。' : '仅自己可见，发布后会出现在发现页。');
        const body = $('#story-detail-body');
        body.replaceChildren();
        if (definition.hook) body.append(element('p', definition.hook, 'story-hook'));
        if (definition.world) body.append(element('div', definition.world, 'story-world'));
        const cast = definition.cast?.[0];
        if (cast) {
            const section = element('section');
            section.append(element('h3', cast.name), element('p', cast.description || cast.personality || '故事中的主要角色'));
            body.append(section);
        }
        if (definition.opening?.length) {
            const section = element('section');
            section.append(element('h3', '开场'), ...definition.opening.map(segment => element('p', segment.text)));
            body.append(section);
        }
        fillPersonaSelect();
        const own = scope === 'mine';
        $('#edit-story').hidden = !own;
        $('#delete-story').hidden = !own;
        $('#publish-story').hidden = !own;
        $('#publish-story').textContent = item.published_version_id ? '更新发布版本' : '发布到发现页';
        $('#unpublish-story').hidden = !own || !item.published_version_id;
        $('#story-dialog').showModal();
    }

    function storyBody(form) {
        const value = name => form.elements[name].value.trim();
        const tags = value('tags').split(/[,，]/).map(item => item.trim()).filter(Boolean).slice(0, 16);
        const opening = [];
        if (value('opening_narration')) opening.push({ id: 'opening_narration', kind: 'narration', text: value('opening_narration') });
        if (value('opening_dialogue')) opening.push({ id: 'opening_dialogue', kind: 'dialogue', speaker_id: 'lead', text: value('opening_dialogue') });
        const keywords = value('lore_keywords').split(/[,，]/).map(item => item.trim()).filter(Boolean).slice(0, 16);
        const lore = value('lore') ? [{ content: value('lore'), keywords, pinned: keywords.length === 0 }] : [];
        return { definition: {
            schema_version: 1, title: value('title'), hook: value('hook'), cover: storyCover, tags,
            world: value('world'), examples: value('examples'), lore,
            cast: [{ id: 'lead', companion_id: value('companion_id') || undefined, name: value('character_name'), description: value('character_description'), personality: value('character_personality'), gender: '', age: '', portrait: '' }],
            opening,
        } };
    }

    function editStory(item = null) {
        const form = storyForm;
        form.reset();
        storyCover = item?.definition.cover || '';
        $('#story-cover-preview').src = storyCover || '/img/ai4.png';
        delete form.dataset.id;
        delete form.dataset.revision;
        const companion = form.elements.companion_id;
        companion.replaceChildren(new Option('仅用于这个故事', ''), ...model.characters.map(character => new Option(character.name, character.id)));
        if (item) {
            const d = item.definition;
            form.dataset.id = item.id;
            form.dataset.revision = item.revision;
            const values = {
                title: d.title, hook: d.hook, tags: (d.tags || []).join('，'), world: d.world,
                companion_id: d.cast?.[0]?.companion_id,
                character_name: d.cast?.[0]?.name, character_description: d.cast?.[0]?.description,
                character_personality: d.cast?.[0]?.personality, examples: d.examples,
                opening_narration: d.opening?.find(value => value.kind === 'narration')?.text,
                opening_dialogue: d.opening?.find(value => value.kind === 'dialogue')?.text,
                lore: d.lore?.[0]?.content, lore_keywords: (d.lore?.[0]?.keywords || []).join('，'),
            };
            for (const [name, value] of Object.entries(values)) form.elements[name].value = value || '';
        }
        form.elements.tags.dispatchEvent(new Event('input', { bubbles: true }));
        storyWizard.reset();
        updateStoryReview();
        $('#story-editor').showModal();
    }

    $('#story-form').elements.companion_id.addEventListener('change', event => {
        const character = model.characters.find(item => item.id === event.target.value);
        if (!character) return;
        const form = $('#story-form');
        form.elements.character_name.value = character.name || '';
        form.elements.character_description.value = character.description || '';
        form.elements.character_personality.value = character.personality || '';
    });

    function resetPersona(item = null) {
        const form = $('#persona-form');
        form.reset();
        personaAvatar = item?.avatar || '';
        $('#persona-avatar-preview').src = personaAvatar || '/img/ai4.png';
        form.elements.id.value = item?.id || '';
        form.elements.revision.value = item?.revision || '';
        form.elements.name.value = item?.name || '';
        form.elements.description.value = item?.description || '';
    }

    $('#stories-nav').addEventListener('click', () => void act(async () => { showStoryView(); await load(); }));
    $('#discover-stories').addEventListener('click', () => { model.view = 'public'; renderCurrentStories(); });
    $('#my-stories').addEventListener('click', () => { model.view = 'mine'; renderCurrentStories(); });
    $('#story-search').addEventListener('input', renderCurrentStories);
    $('#create-story').addEventListener('click', () => editStory());
    $('#story-cover-file').addEventListener('change', event => void act(async () => {
        storyCover = await readPortrait(event.target.files[0], 900);
        $('#story-cover-preview').src = storyCover || '/img/ai4.png';
        updateStoryReview();
    }));
    $('#story-cover-clear').addEventListener('click', () => {
        storyCover = '';
        $('#story-cover-file').value = '';
        $('#story-cover-preview').src = '/img/ai4.png';
        updateStoryReview();
    });
    storyForm.addEventListener('input', updateStoryReview);
    storyForm.addEventListener('change', updateStoryReview);
    $('#manage-personas').addEventListener('click', () => void act(async () => { await load(); resetPersona(); $('#persona-dialog').showModal(); }));
    $('#story-list').addEventListener('click', event => {
        const card = event.target.closest('[data-story]');
        if (!card) return;
        const items = card.dataset.storyScope === 'public' ? model.publicStories : model.stories;
        const key = card.dataset.storyScope === 'public' ? 'story_id' : 'id';
        const item = items.find(value => value[key] === card.dataset.story);
        if (item) openStory(item, card.dataset.storyScope);
    });
    $('#edit-story').addEventListener('click', () => { $('#story-dialog').close(); editStory(model.selected); });
    $('#delete-story').addEventListener('click', () => void act(async () => {
        if (!await confirmDelete(`删除故事「${model.selected.definition.title}」？已有聊天仍会保留。`)) return;
        await apiClient.deleteStory(model.selected.id);
        $('#story-dialog').close();
        await load();
    }));
    $('#publish-story').addEventListener('click', () => void act(async () => {
        model.selected = await apiClient.publishStory(model.selected.id, { expected_revision: model.selected.revision });
        $('#story-dialog').close();
        model.view = 'public';
        await load();
    }));
    $('#unpublish-story').addEventListener('click', () => void act(async () => {
        model.selected = await apiClient.unpublishStory(model.selected.id);
        $('#story-dialog').close();
        await load();
    }));
    $('#story-form').addEventListener('submit', event => {
        event.preventDefault();
        if (!storyWizard.revealInvalid()) return;
        void act(async () => {
            const form = event.currentTarget;
            const body = storyBody(form);
            if (form.dataset.id) {
                body.expected_revision = form.dataset.revision;
                await apiClient.updateStory(form.dataset.id, body);
            } else await apiClient.createStory(body);
            $('#story-editor').close();
            showStoryView();
            await load();
        });
    });
    $('#start-story').addEventListener('click', () => void act(async () => {
        const personaID = $('#story-persona').value;
        if (!personaID) throw new Error('请先创建并选择一个玩家身份');
        const versionID = model.scope === 'public' ? model.selected.version_id :
            (await apiClient.freezeStory(model.selected.id, { expected_revision: model.selected.revision })).id;
        const key = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random()}`;
        const state = await apiClient.createChat({ mode: 'story', story_version_id: versionID, persona_id: personaID, idempotency_key: key });
        $('#story-dialog').close();
        await refreshChats();
        await openChat(state.chat.id);
    }));

    $('#persona-reset').addEventListener('click', () => resetPersona());
    $('#persona-avatar-file').addEventListener('change', event => void act(async () => {
        personaAvatar = await readPortrait(event.target.files[0], 384);
        $('#persona-avatar-preview').src = personaAvatar || '/img/ai4.png';
    }));
    $('#persona-avatar-clear').addEventListener('click', () => {
        personaAvatar = '';
        $('#persona-avatar-file').value = '';
        $('#persona-avatar-preview').src = '/img/ai4.png';
    });
    $('#persona-list').addEventListener('click', event => {
        const editID = event.target.closest('[data-persona-edit]')?.dataset.personaEdit;
        if (editID) resetPersona(model.personas.find(item => item.id === editID));
        const deleteID = event.target.closest('[data-persona-delete]')?.dataset.personaDelete;
        if (deleteID) void act(async () => {
            const item = model.personas.find(value => value.id === deleteID);
            if (!await confirmDelete(`删除身份「${item.name}」？已有故事存档不会变化。`)) return;
            await apiClient.deletePersona(deleteID);
            await load();
            resetPersona();
        });
    });
    $('#persona-form').addEventListener('submit', event => {
        event.preventDefault();
        void act(async () => {
            const form = event.currentTarget;
            const body = { name: form.elements.name.value.trim(), description: form.elements.description.value.trim(), avatar: personaAvatar };
            if (form.elements.id.value) {
                body.expected_revision = form.elements.revision.value;
                await apiClient.updatePersona(form.elements.id.value, body);
            } else await apiClient.createPersona(body);
            await load();
            resetPersona();
        });
    });

    return { show: showStoryView, load };
}
