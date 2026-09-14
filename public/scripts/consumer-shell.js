import { getContext } from './st-context.js';

const DEFAULT_AVATAR = 'img/ai4.png';
const DEFAULT_BACKGROUND = 'bedroom cyberpunk.jpg';
const BACKGROUNDS = [
    'bedroom cyberpunk.jpg',
    'japan classroom.jpg',
    'landscape beach night.jpg',
    'tavern day.jpg',
];
const FAVORITES_KEY = 'consumer-shell:favorites';
const RECENTS_KEY = 'consumer-shell:recents';
const AUTO_VOICE_KEY = 'consumer-shell:auto-voice';

const escapeHtml = (value) => String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll('\'', '&#039;');

const characterDescription = (character) => {
    const data = character?.data ?? {};
    return data.creator_notes || character?.description || character?.personality || '准备好开始一段新的对话。';
};

const characterTags = (character) => {
    const data = character?.data ?? {};
    const tags = data.tags || character?.tags || [];
    return Array.isArray(tags) ? tags.filter(Boolean).slice(0, 3) : [];
};

const characterKey = (character, index = null) => {
    const data = character?.data ?? {};
    const stable = data.extensions?.character_id || data.character_id || character?.id;
    return String(stable || character?.avatar || (index !== null ? `index:${index}` : character?.name || ''));
};

const readStoredList = (key) => {
    try {
        const value = JSON.parse(window.localStorage.getItem(key) || '[]');
        return Array.isArray(value) ? value.map(String) : [];
    } catch {
        return [];
    }
};

const writeStoredList = (key, value) => {
    try {
        window.localStorage.setItem(key, JSON.stringify(value));
    } catch {
        // Private browsing or a blocked storage policy should not break chat.
    }
};

const characterBackground = (context, character, index = 0) => {
    const extensions = character?.data?.extensions ?? {};
    const configured = extensions.background || extensions.chat_background || extensions.background_url;
    if (configured) {
        if (String(configured).startsWith('http') || String(configured).startsWith('/')) return configured;
        return context.getThumbnailUrl('bg', configured);
    }
    return context.getThumbnailUrl('bg', BACKGROUNDS[index % BACKGROUNDS.length]) || DEFAULT_BACKGROUND;
};

const characterAvatar = (context, character) => {
    if (!character || character.avatar === 'none') return DEFAULT_AVATAR;
    return context.getThumbnailUrl('avatar', character.avatar);
};

const mediaUrl = (value) => {
    if (!value) return '';
    const stringValue = String(value);
    if (/^(https?:|data:|blob:|\/)/i.test(stringValue)) return stringValue;
    return `/${stringValue}`;
};

const isUploadedImagePath = (value) => {
    const stringValue = String(value || '').replace(/^\/+/, '');
    return stringValue.startsWith('user/images/');
};

const characterMedia = (context, character) => {
    const configured = character?.data?.extensions?.consumer_media ?? {};
    const gallery = Array.isArray(configured.gallery)
        ? configured.gallery
            .map((item, index) => typeof item === 'string'
                ? { url: item, order: index, caption: '' }
                : item)
            .filter(item => item?.url)
            .map((item, index) => ({
                url: String(item.url),
                order: Number.isFinite(Number(item.order)) ? Number(item.order) : index,
                caption: String(item.caption || ''),
            }))
            .sort((a, b) => a.order - b.order)
        : [];
    const cover = configured.cover || gallery[0]?.url || characterAvatar(context, character);
    const items = gallery.length ? gallery : [{ url: cover, order: 0, caption: '' }];
    const coverItem = items.find(item => item.url === configured.cover) || items[0];
    return {
        cover: coverItem?.url || cover,
        gallery: items,
    };
};

const fileToBase64 = (file) => new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result).split(',').at(-1));
    reader.onerror = reject;
    reader.readAsDataURL(file);
});

function createShell() {
    const shell = document.createElement('div');
    shell.id = 'consumer-shell';
    shell.innerHTML = `
        <div class="consumer-app">
            <nav class="consumer-nav">
                <button class="consumer-brand consumer-nav-button" data-consumer-action="discover" aria-label="返回发现">
                    <span class="consumer-brand-mark">AI</span>
                    <span>陪伴</span>
                </button>
                <div class="consumer-nav-links" role="navigation" aria-label="主导航">
                    <button class="consumer-nav-button is-active" data-consumer-action="discover">角色</button>
                    <button class="consumer-nav-button" data-consumer-action="history">最近聊天</button>
                    <button class="consumer-nav-button" data-consumer-action="creator">创作者</button>
                </div>
                <div class="consumer-nav-actions">
                    <span class="consumer-connection-status" data-consumer-connection-status><i class="fa-solid fa-circle"></i><span>连接检查中</span></span>
                        <button class="consumer-nav-button consumer-history-button" data-consumer-action="history"><i class="fa-regular fa-clock"></i> 最近聊天</button>
                    <button class="consumer-nav-button consumer-create-button" data-consumer-action="create"><i class="fa-solid fa-user-plus"></i> 创建角色</button>
                    <button class="consumer-nav-button consumer-more-button" data-consumer-action="more"><i class="fa-solid fa-ellipsis"></i> 更多</button>
                    <button class="consumer-icon-button" data-consumer-action="auto-voice" title="自动朗读 AI 回复" aria-label="自动朗读 AI 回复" aria-pressed="false"><i class="fa-solid fa-volume-high"></i></button>
                </div>
            </nav>
            <main class="consumer-main">
                <section class="consumer-view is-active" data-consumer-view-panel="discover">
                    <div class="consumer-hero">
                        <h1 class="consumer-title">选择一个角色开始聊天</h1>
                        <label class="consumer-search-wrap">
                            <i class="fa-solid fa-magnifying-glass"></i>
                            <input class="consumer-search" type="search" placeholder="搜索角色">
                        </label>
                    </div>
                    <div class="consumer-section-heading">
                        <div class="consumer-section-tabs" role="tablist" aria-label="角色筛选">
                            <button class="consumer-tab is-active" data-consumer-filter="recommended" role="tab" aria-selected="true">推荐</button>
                            <button class="consumer-tab" data-consumer-filter="recent" role="tab" aria-selected="false">最近聊天</button>
                            <button class="consumer-tab" data-consumer-filter="favorite" role="tab" aria-selected="false">收藏</button>
                        </div>
                        <label class="consumer-sort-wrap">排序
                            <select data-consumer-sort aria-label="角色排序">
                                <option value="recommended">推荐</option>
                                <option value="latest">最新</option>
                                <option value="name">名称</option>
                            </select>
                        </label>
                        <span data-consumer-count></span>
                    </div>
                    <div class="consumer-tag-filters" data-consumer-tags role="listbox" aria-label="按标签筛选"></div>
                    <div class="consumer-character-grid" data-consumer-characters></div>
                </section>
                <section class="consumer-view" data-consumer-view-panel="creator">
                    <div class="consumer-page-heading">
                        <span class="consumer-detail-kicker">创作者空间</span>
                        <h1>管理你的角色</h1>
                        <p>创建、整理并导入属于你的角色卡。</p>
                        <div class="consumer-detail-actions">
                            <button class="consumer-primary-button" data-consumer-action="create"><i class="fa-solid fa-bolt"></i> 快速创建</button>
                            <button class="consumer-secondary-button" data-consumer-action="advanced"><i class="fa-solid fa-sliders"></i> 高级创建</button>
                        </div>
                    </div>
                    <div class="consumer-creator-grid">
                        <div><strong>我的角色</strong><span data-consumer-creator-count>角色库</span></div>
                        <div><strong>角色卡导入</strong><span>使用现有角色卡继续创作</span></div>
                        <div><strong>媒体资产</strong><span>为角色添加场景图和画廊</span></div>
                    </div>
                </section>
                <section class="consumer-view" data-consumer-view-panel="detail">
                    <button class="consumer-back" data-consumer-action="discover"><i class="fa-solid fa-arrow-left"></i> 返回角色列表</button>
                    <div class="consumer-detail">
                        <div class="consumer-detail-media">
                            <div class="consumer-detail-stage">
                                <img class="consumer-detail-avatar" data-consumer-detail-avatar alt="">
                                <span class="consumer-detail-count" data-consumer-detail-count></span>
                            </div>
                            <div class="consumer-detail-gallery" data-consumer-detail-gallery></div>
                        </div>
                        <div>
                            <div class="consumer-detail-kicker">角色场景</div>
                            <h1 data-consumer-detail-name></h1>
                            <p class="consumer-detail-description" data-consumer-detail-description></p>
                            <div class="consumer-detail-tags" data-consumer-detail-tags></div>
                            <div class="consumer-opening" data-consumer-detail-opening></div>
                            <div class="consumer-detail-actions">
                                <button class="consumer-primary-button" data-consumer-action="start"><i class="fa-solid fa-message"></i> 进入场景聊天</button>
                                <button class="consumer-secondary-button" data-consumer-action="favorite"><i class="fa-regular fa-heart"></i> 收藏角色</button>
                                <button class="consumer-secondary-button" data-consumer-action="manage-media"><i class="fa-solid fa-images"></i> 管理图片</button>
                            </div>
                        </div>
                    </div>
                </section>
                <section class="consumer-view consumer-chat-view" data-consumer-view-panel="chat">
                    <header class="consumer-chat-header">
                        <button class="consumer-icon-button" data-consumer-action="discover" title="返回角色列表" aria-label="返回角色列表"><i class="fa-solid fa-arrow-left"></i></button>
                        <img data-consumer-chat-avatar alt="">
                        <div class="consumer-chat-heading"><strong data-consumer-chat-name></strong><small data-consumer-chat-status>正在这个场景中陪伴你</small></div>
                        <button class="consumer-icon-button consumer-chat-more" data-consumer-action="more" title="更多聊天操作" aria-label="更多聊天操作"><i class="fa-solid fa-ellipsis"></i></button>
                        <button class="consumer-icon-button consumer-chat-favorite" data-consumer-action="favorite" title="收藏角色" aria-label="收藏角色"><i class="fa-regular fa-heart"></i></button>
                    </header>
                    <div class="consumer-chat-frame"></div>
                    <div class="consumer-composer">
                        <div class="consumer-composer-inner">
                            <button class="consumer-voice" data-consumer-action="voice" title="播放最近一条 AI 回复" aria-label="播放最近一条 AI 回复"><i class="fa-solid fa-volume-high"></i></button>
                            <span class="consumer-input-mode" data-consumer-input-mode>文字输入</span>
                            <textarea data-consumer-input rows="1" placeholder="输入消息..." data-text-placeholder="输入消息..." data-voice-placeholder="点击麦克风开始说话"></textarea>
                            <button class="consumer-mic" data-consumer-action="record" title="点击启用语音输入" aria-label="点击启用语音输入" aria-pressed="false"><i class="fa-solid fa-microphone"></i><span>语音</span></button>
                            <button class="consumer-send" data-consumer-action="send" title="发送" aria-label="发送"><i class="fa-solid fa-arrow-up"></i></button>
                        </div>
                        <div class="consumer-recording-status" data-consumer-recording-status aria-live="polite"></div>
                    </div>
                </section>
            </main>
            <div class="consumer-more-menu" data-consumer-more-menu hidden>
                <button data-consumer-action="chat-history"><i class="fa-regular fa-clock"></i> 聊天记录</button>
                <button data-consumer-action="new-chat"><i class="fa-solid fa-plus"></i> 新建聊天</button>
                <button data-consumer-action="chat-note"><i class="fa-solid fa-note-sticky"></i> 本聊天备注</button>
                <button data-consumer-action="copy-latest"><i class="fa-regular fa-copy"></i> 复制最近回复</button>
                <button data-consumer-action="regenerate"><i class="fa-solid fa-rotate"></i> 重新生成</button>
                <button data-consumer-action="advanced"><i class="fa-solid fa-sliders"></i> 切换高级模式</button>
            </div>
        </div>`;
    document.body.prepend(shell);
    const noteSheet = document.createElement('dialog');
    noteSheet.className = 'consumer-note-sheet';
    noteSheet.innerHTML = `
        <form method="dialog" class="consumer-note-dialog">
            <div class="consumer-note-heading">
                <div>
                    <span class="consumer-detail-kicker">当前聊天</span>
                    <h2>本聊天备注</h2>
                </div>
                <button class="consumer-icon-button" value="cancel" title="关闭" aria-label="关闭"><i class="fa-solid fa-xmark"></i></button>
            </div>
            <p>记录你希望在这段对话里持续记住的内容。备注会保存到当前聊天，不会自动发送给 AI。</p>
            <label for="consumer-note-input">备注内容</label>
            <textarea id="consumer-note-input" rows="7" placeholder="例如：我们已经约定在月光图书馆见面。"></textarea>
            <div class="consumer-note-actions">
                <button class="consumer-secondary-button" value="cancel">取消</button>
                <button class="consumer-primary-button" value="save" data-consumer-note-save><i class="fa-solid fa-check"></i> 保存备注</button>
            </div>
        </form>`;
    document.body.append(noteSheet);
    const mediaSheet = document.createElement('dialog');
    mediaSheet.className = 'consumer-media-sheet';
    mediaSheet.innerHTML = `
        <form method="dialog" class="consumer-media-dialog">
            <div class="consumer-note-heading">
                <div>
                    <span class="consumer-detail-kicker">角色视觉资产</span>
                    <h2>管理图片</h2>
                </div>
                <button class="consumer-icon-button" value="cancel" title="关闭" aria-label="关闭"><i class="fa-solid fa-xmark"></i></button>
            </div>
            <p>第一张图片会优先作为角色封面。你可以上传多张图片、调整顺序或更换封面。</p>
            <label class="consumer-upload-dropzone">
                <input type="file" data-consumer-media-input accept="image/*" multiple>
                <i class="fa-solid fa-cloud-arrow-up"></i>
                <strong>添加图片</strong>
                <span>支持 JPG、PNG、WEBP 等常见格式</span>
            </label>
            <div class="consumer-media-list" data-consumer-media-list></div>
            <div class="consumer-note-actions">
                <button class="consumer-secondary-button" value="cancel">完成</button>
            </div>
        </form>`;
    document.body.append(mediaSheet);
    const restore = document.createElement('button');
    restore.id = 'consumer-restore';
    restore.className = 'consumer-restore';
    restore.dataset.consumerAction = 'consumer';
    restore.title = '返回简洁模式';
    restore.setAttribute('aria-label', '返回简洁模式');
    restore.innerHTML = '<i class="fa-solid fa-wand-magic-sparkles"></i><span>简洁模式</span>';
    document.body.append(restore);
    shell.consumerMediaSheet = mediaSheet;
    return shell;
}

function latestAssistantMessage() {
    const messages = [...document.querySelectorAll('#chat .mes:not([is_user="true"])')];
    return messages.at(-1);
}

function latestAssistantText() {
    return latestAssistantMessage()?.querySelector('.mes_text')?.textContent?.trim() || '';
}

function speakText(text) {
    if (!('speechSynthesis' in window)) return false;
    if (!text) return false;
    window.speechSynthesis.cancel();
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = document.documentElement.lang || navigator.language || 'zh-CN';
    window.speechSynthesis.speak(utterance);
    return true;
}

async function speakLatestMessage(context) {
    const latestMessage = latestAssistantMessage();
    const ttsSettings = context.extensionSettings?.tts ?? {};
    if (ttsSettings.enabled || ttsSettings.ttsEnabled) {
        const narrateButton = latestMessage?.querySelector('.mes_narrate');
        if (narrateButton) {
            narrateButton.click();
            return;
        }
    }
    const text = latestAssistantText();
    if (!text) {
        window.toastr?.info('还没有可以朗读的 AI 回复');
        return;
    }
    if (!speakText(text)) {
        window.toastr?.info('当前浏览器不支持语音播放');
    }
}

function initConsumerShell(context) {
    const shell = createShell();
    const mediaSheet = shell.consumerMediaSheet;
    const restoreButton = document.querySelector('#consumer-restore');
    const chatFrame = shell.querySelector('.consumer-chat-frame');
    const originalChat = document.querySelector('#chat');
    const chatPlaceholder = originalChat ? document.createComment('consumer-chat-placeholder') : null;
    const originalChatParent = originalChat?.parentElement;
    if (originalChat && chatPlaceholder && originalChatParent) {
        originalChatParent.insertBefore(chatPlaceholder, originalChat);
        chatFrame.append(originalChat);
    }

    const state = {
        characters: [],
        selectedKey: null,
        query: '',
        recording: false,
        voiceMode: false,
        recognition: null,
        generating: false,
        connected: false,
        filter: 'recommended',
        favorites: new Set(readStoredList(FAVORITES_KEY)),
        recents: readStoredList(RECENTS_KEY),
        tag: 'all',
        autoVoice: readStoredList(AUTO_VOICE_KEY).includes('on'),
        lastSpokenMessageId: null,
        previousAssistantMessage: null,
        completionObserver: null,
        editorMedia: {
            characterId: null,
            gallery: [],
            cover: '',
        },
    };
    const panels = [...shell.querySelectorAll('[data-consumer-view-panel]')];
    const showView = (view) => {
        panels.forEach((panel) => panel.classList.toggle('is-active', panel.dataset.consumerViewPanel === view));
    };
    const enterConsumerMode = () => {
        if (originalChat && chatFrame && originalChatParent && !chatFrame.contains(originalChat)) {
            originalChatParent.insertBefore(chatPlaceholder, originalChat);
            chatFrame.append(originalChat);
        }
        document.body.classList.add('consumer-mode');
    };
    const enterAdvancedMode = () => {
        if (originalChat && chatPlaceholder && originalChatParent && chatFrame.contains(originalChat)) {
            chatPlaceholder.replaceWith(originalChat);
        }
        document.body.classList.remove('consumer-mode');
    };
    restoreButton?.addEventListener('click', () => {
        enterConsumerMode();
        showView(state.selectedKey === null ? 'discover' : 'chat');
    });
    const selectedIndex = () => state.selectedKey === null
        ? null
        : state.characters.findIndex((character, index) => characterKey(character, index) === state.selectedKey);
    const selectedCharacter = () => {
        const index = selectedIndex();
        return index === null || index < 0 ? null : state.characters[index];
    };
    const isFavorite = (character) => state.favorites.has(characterKey(character));
    const updateFavoriteButton = () => {
        const character = selectedCharacter();
        const active = Boolean(character && isFavorite(character));
        const buttons = shell.querySelectorAll('[data-consumer-action="favorite"]');
        buttons.forEach((button) => {
            button.classList.toggle('is-active', active);
            button.setAttribute('aria-pressed', String(active));
            button.title = active ? '取消收藏' : '收藏角色';
            button.setAttribute('aria-label', button.title);
            const icon = button.querySelector('i');
            icon?.classList.toggle('fa-solid', active);
            icon?.classList.toggle('fa-regular', !active);
        });
    };
    const toggleFavorite = () => {
        const character = selectedCharacter();
        if (!character) return;
        const key = characterKey(character);
        if (state.favorites.has(key)) state.favorites.delete(key);
        else state.favorites.add(key);
        writeStoredList(FAVORITES_KEY, [...state.favorites]);
        updateFavoriteButton();
        renderCards();
    };
    const availableTags = () => {
        const counts = new Map();
        state.characters.forEach((character) => characterTags(character).forEach((tag) => {
            counts.set(tag, (counts.get(tag) || 0) + 1);
        }));
        return [...counts.entries()]
            .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
            .slice(0, 10)
            .map(([tag]) => tag);
    };
    const renderTagFilters = () => {
        const container = shell.querySelector('[data-consumer-tags]');
        const tags = availableTags();
        container.innerHTML = ['all', ...tags].map((tag) => {
            const active = state.tag === tag;
            return `<button class="consumer-tag-filter ${active ? 'is-active' : ''}" data-consumer-tag="${escapeHtml(tag)}" role="option" aria-selected="${active}">${tag === 'all' ? '全部标签' : escapeHtml(tag)}</button>`;
        }).join('');
    };
    const setBackground = () => {
        const character = selectedCharacter();
        const index = selectedIndex() ?? 0;
        const app = shell.querySelector('.consumer-app');
        app.style.setProperty('--consumer-background', `url("${characterBackground(context, character, index)}")`);
        app.classList.toggle('has-character-background', Boolean(character));
    };
    const renderCards = () => {
        const list = shell.querySelector('[data-consumer-characters]');
        const query = state.query.trim().toLocaleLowerCase();
        let visible = state.characters.filter((character) => String(character.name ?? '').toLocaleLowerCase().includes(query));
        if (state.tag !== 'all') {
            visible = visible.filter((character) => characterTags(character).includes(state.tag));
        }
        if (state.filter === 'recent') {
            const order = new Map(state.recents.map((key, index) => [key, index]));
            visible = visible.filter((character) => order.has(characterKey(character)))
                .sort((a, b) => order.get(characterKey(a)) - order.get(characterKey(b)));
        } else if (state.filter === 'favorite') {
            visible = visible.filter((character) => isFavorite(character));
        }
        const sort = shell.querySelector('[data-consumer-sort]')?.value || 'recommended';
        if (sort === 'name') visible.sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')));
        if (sort === 'latest') visible.sort((a, b) => Number(b.create_date || b.created_at || 0) - Number(a.create_date || a.created_at || 0));
        shell.querySelector('[data-consumer-count]').textContent = `${visible.length} 个角色`;
        list.innerHTML = visible.length ? visible.map((character) => {
            const id = state.characters.indexOf(character);
            const tags = characterTags(character).slice(0, 2);
            const favorite = isFavorite(character);
            const media = characterMedia(context, character);
            return `<article class="consumer-character-card" data-consumer-character="${id}">
                <div class="consumer-character-visual">
                    <img class="consumer-character-avatar" loading="lazy" decoding="async" src="${escapeHtml(mediaUrl(media.cover))}" alt="${escapeHtml(character.name)}" onerror="this.classList.add('is-broken'); this.removeAttribute('src');">
                    <div class="consumer-character-overlay"><strong>${escapeHtml(character.name)}</strong><span>${escapeHtml(characterDescription(character))}</span></div>
                    ${media.gallery.length > 1 ? `<span class="consumer-card-media-count"><i class="fa-solid fa-images"></i> ${media.gallery.length}</span>` : ''}
                </div>
                <button class="consumer-card-favorite ${favorite ? 'is-active' : ''}" data-consumer-action="favorite" data-consumer-character-favorite="${id}" title="${favorite ? '取消收藏' : '收藏角色'}" aria-label="${favorite ? '取消收藏' : '收藏角色'}" aria-pressed="${favorite}"><i class="${favorite ? 'fa-solid' : 'fa-regular'} fa-heart"></i></button>
                <div class="consumer-character-copy">
                    <div class="consumer-card-tags">${tags.map(tag => `<span>${escapeHtml(tag)}</span>`).join('')}</div>
                </div>
            </article>`;
        }).join('') : `<div class="consumer-empty"><strong>${state.characters.length ? '没有找到匹配的角色' : '还没有角色'}</strong><span>${state.characters.length ? '试试其他关键词或标签' : '先创建一个角色，开始你的第一段对话'}</span></div>`;
    };
    const renderDetail = () => {
        const character = selectedCharacter();
        if (!character) return;
        const media = characterMedia(context, character);
        const stage = shell.querySelector('[data-consumer-detail-avatar]');
        stage.src = mediaUrl(media.cover);
        shell.querySelector('[data-consumer-detail-avatar]').alt = character.name;
        shell.querySelector('[data-consumer-detail-count]').textContent = media.gallery.length > 1 ? `${media.gallery.length} 张图片` : '';
        shell.querySelector('[data-consumer-detail-gallery]').innerHTML = media.gallery.map((item, index) => `
            <button class="consumer-detail-thumb ${item.url === media.cover ? 'is-active' : ''}" data-consumer-detail-image="${escapeHtml(item.url)}" title="查看第 ${index + 1} 张图片">
                <img src="${escapeHtml(mediaUrl(item.url))}" alt="">
            </button>`).join('');
        shell.querySelector('[data-consumer-detail-name]').textContent = character.name;
        shell.querySelector('[data-consumer-detail-description]').textContent = characterDescription(character);
        shell.querySelector('[data-consumer-detail-opening]').textContent = character.first_mes || '从一句简单的问候开始吧。';
        shell.querySelector('[data-consumer-detail-tags]').innerHTML = characterTags(character).map(tag => `<span>${escapeHtml(tag)}</span>`).join('');
        updateFavoriteButton();
        setBackground();
    };
    const renderMediaList = () => {
        const character = selectedCharacter();
        const list = mediaSheet.querySelector('[data-consumer-media-list]');
        if (!character || !list) return;
        const media = characterMedia(context, character);
        list.innerHTML = media.gallery.map((item, index) => {
            const isCover = item.url === media.cover;
            return `<div class="consumer-media-row" data-consumer-media-row="${index}">
                <img src="${escapeHtml(mediaUrl(item.url))}" alt="">
                <div class="consumer-media-row-copy">
                    <strong>${isCover ? '封面图' : `图片 ${index + 1}`}</strong>
                    <small>${escapeHtml(item.caption || '角色画廊图片')}</small>
                </div>
                <div class="consumer-media-row-actions">
                    <button type="button" class="consumer-icon-button ${isCover ? 'is-active' : ''}" data-consumer-media-action="cover" data-consumer-media-index="${index}" title="${isCover ? '当前封面' : '设为封面'}" aria-label="${isCover ? '当前封面' : '设为封面'}"><i class="fa-solid fa-star"></i></button>
                    <button type="button" class="consumer-icon-button" data-consumer-media-action="up" data-consumer-media-index="${index}" title="上移" aria-label="上移"><i class="fa-solid fa-arrow-up"></i></button>
                    <button type="button" class="consumer-icon-button" data-consumer-media-action="down" data-consumer-media-index="${index}" title="下移" aria-label="下移"><i class="fa-solid fa-arrow-down"></i></button>
                    <button type="button" class="consumer-icon-button" data-consumer-media-action="delete" data-consumer-media-index="${index}" title="删除图片" aria-label="删除图片"><i class="fa-solid fa-trash"></i></button>
                </div>
            </div>`;
        }).join('');
    };
    const saveCharacterMedia = async (gallery, cover) => {
        const character = selectedCharacter();
        if (!character) return false;
        const value = {
            cover: cover || gallery[0]?.url || characterAvatar(context, character),
            gallery: gallery.map((item, index) => ({ ...item, order: index })),
        };
        try {
            await context.writeExtensionField?.(selectedIndex(), 'consumer_media', value);
            character.data = character.data || {};
            character.data.extensions = character.data.extensions || {};
            character.data.extensions.consumer_media = value;
            renderMediaList();
            renderDetail();
            renderCards();
            return true;
        } catch (error) {
            console.error('Failed to save character media', error);
            window.toastr?.error('图片配置保存失败，请重试');
            return false;
        }
    };
    const editorMediaValue = () => ({
        cover: state.editorMedia.cover || state.editorMedia.gallery[0]?.url || '',
        gallery: state.editorMedia.gallery.map((item, index) => ({ ...item, order: index })),
    });
    const renderEditorMedia = () => {
        const list = document.querySelector('[data-consumer-editor-media-list]');
        if (!list) return;
        list.innerHTML = state.editorMedia.gallery.map((item, index) => {
            const isCover = item.url === state.editorMedia.cover;
            return `<div class="consumer-editor-media-row">
                <img src="${escapeHtml(mediaUrl(item.url))}" alt="">
                <div class="consumer-editor-media-row-copy">
                    <strong>${isCover ? '封面图' : `图片 ${index + 1}`}</strong>
                    <small>${escapeHtml(item.caption || '角色画廊图片')}</small>
                </div>
                <div class="consumer-editor-media-actions">
                    <button type="button" class="consumer-icon-button ${isCover ? 'is-active' : ''}" data-consumer-editor-media-action="cover" data-consumer-editor-media-index="${index}" title="${isCover ? '当前封面' : '设为封面'}" aria-label="${isCover ? '当前封面' : '设为封面'}"><i class="fa-solid fa-star"></i></button>
                    <button type="button" class="consumer-icon-button" data-consumer-editor-media-action="up" data-consumer-editor-media-index="${index}" title="上移" aria-label="上移"><i class="fa-solid fa-arrow-up"></i></button>
                    <button type="button" class="consumer-icon-button" data-consumer-editor-media-action="down" data-consumer-editor-media-index="${index}" title="下移" aria-label="下移"><i class="fa-solid fa-arrow-down"></i></button>
                    <button type="button" class="consumer-icon-button" data-consumer-editor-media-action="delete" data-consumer-media-index="${index}" title="删除图片" aria-label="删除图片"><i class="fa-solid fa-trash"></i></button>
                </div>
            </div>`;
        }).join('');
    };
    const loadEditorMedia = (characterId = null) => {
        const hasCharacterId = characterId !== null && characterId !== undefined && characterId !== '';
        const character = hasCharacterId && Number.isInteger(Number(characterId)) ? context.characters[Number(characterId)] : null;
        const configured = character?.data?.extensions?.consumer_media
            || context.createCharacterData?.extensions?.consumer_media
            || {};
        const gallery = Array.isArray(configured.gallery)
            ? configured.gallery.map((item, index) => typeof item === 'string'
                ? { url: item, order: index, caption: '' }
                : item).filter(item => item?.url).map((item, index) => ({
                url: String(item.url),
                order: Number.isFinite(Number(item.order)) ? Number(item.order) : index,
                caption: String(item.caption || ''),
            })).sort((a, b) => a.order - b.order)
            : [];
        state.editorMedia = {
            characterId: character ? Number(characterId) : null,
            gallery,
            cover: configured.cover || gallery[0]?.url || '',
        };
        renderEditorMedia();
    };
    const saveEditorMedia = async () => {
        const value = editorMediaValue();
        if (state.editorMedia.characterId === null) {
            context.createCharacterData.extensions = context.createCharacterData.extensions || {};
            if (value.gallery.length) context.createCharacterData.extensions.consumer_media = value;
            else delete context.createCharacterData.extensions.consumer_media;
            return true;
        }
        try {
            await context.writeExtensionField?.(state.editorMedia.characterId, 'consumer_media', value);
            const character = context.characters[state.editorMedia.characterId];
            if (character) {
                character.data = character.data || {};
                character.data.extensions = character.data.extensions || {};
                character.data.extensions.consumer_media = value;
            }
            syncCharacters();
            return true;
        } catch (error) {
            console.error('Failed to save editor character media', error);
            window.toastr?.error('图片配置保存失败，请重试');
            return false;
        }
    };
    const uploadEditorMedia = async (files) => {
        if (!files.length) return;
        const name = document.querySelector('#character_name_pole')?.value?.trim() || 'character';
        const uploaded = [];
        for (const file of files) {
            try {
                const format = (file.name.split('.').at(-1) || 'png').toLowerCase();
                const base64 = await fileToBase64(file);
                const response = await fetch('/api/images/upload', {
                    method: 'POST',
                    headers: context.getRequestHeaders?.(),
                    body: JSON.stringify({ image: base64, format, ch_name: name }),
                });
                const payload = await response.json();
                if (!response.ok || !payload.path) throw new Error(payload.error || 'upload failed');
                uploaded.push({ url: payload.path, caption: '' });
            } catch (error) {
                console.error('Failed to upload editor character image', error);
                window.toastr?.error(`图片上传失败：${file.name}`);
            }
        }
        if (uploaded.length) {
            state.editorMedia.gallery.push(...uploaded);
            state.editorMedia.cover ||= uploaded[0].url;
            renderEditorMedia();
            await saveEditorMedia();
        }
    };
    const openMediaSheet = () => {
        if (!selectedCharacter()) {
            window.toastr?.info('请先选择一个角色');
            return;
        }
        renderMediaList();
        if (typeof mediaSheet.showModal === 'function') mediaSheet.showModal();
        else mediaSheet.setAttribute('open', '');
    };
    const uploadMedia = async (files) => {
        const character = selectedCharacter();
        if (!character || !files.length) return;
        const media = characterMedia(context, character);
        const uploaded = [];
        for (const file of files) {
            try {
                const format = (file.name.split('.').at(-1) || 'png').toLowerCase();
                const base64 = await fileToBase64(file);
                const response = await fetch('/api/images/upload', {
                    method: 'POST',
                    headers: context.getRequestHeaders?.(),
                    body: JSON.stringify({ image: base64, format, ch_name: character.name }),
                });
                const payload = await response.json();
                if (!response.ok || !payload.path) throw new Error(payload.error || 'upload failed');
                uploaded.push({ url: payload.path, order: media.gallery.length + uploaded.length, caption: '' });
            } catch (error) {
                console.error('Failed to upload character image', error);
                window.toastr?.error(`图片上传失败：${file.name}`);
            }
        }
        if (uploaded.length) {
            const saved = await saveCharacterMedia([...media.gallery, ...uploaded], media.cover);
            if (saved) window.toastr?.success(`已添加 ${uploaded.length} 张图片`);
        }
    };
    const renderChatHeader = () => {
        const character = selectedCharacter();
        if (!character) return;
        shell.querySelector('[data-consumer-chat-avatar]').src = characterAvatar(context, character);
        shell.querySelector('[data-consumer-chat-name]').textContent = character.name;
        updateFavoriteButton();
        setBackground();
    };
    const input = shell.querySelector('[data-consumer-input]');
    const recordingStatus = shell.querySelector('[data-consumer-recording-status]');
    const micButton = shell.querySelector('.consumer-mic');
    const autoVoiceButton = shell.querySelector('[data-consumer-action="auto-voice"]');
    const connectionStatus = shell.querySelector('[data-consumer-connection-status]');
    const chatStatus = shell.querySelector('[data-consumer-chat-status]');
    const sendButton = shell.querySelector('.consumer-send');
    const noteSheet = document.querySelector('.consumer-note-sheet');
    const noteInput = document.querySelector('#consumer-note-input');
    const finishGeneration = () => renderGenerationState(false);
    let generationWatchdog = null;
    let generationSyncTimer = null;
    let generationStartedAt = 0;
    const hasAssistantText = (text) => Boolean(text && !/^(?:…|\.{3})$/.test(text));
    const renderConnectionStatus = (status = context.onlineStatus) => {
        const connected = status && status !== 'no_connection';
        state.connected = Boolean(connected);
        connectionStatus.classList.toggle('is-connected', Boolean(connected));
        connectionStatus.classList.toggle('is-disconnected', !connected);
        connectionStatus.querySelector('span').textContent = connected ? '已连接' : '未连接';
        connectionStatus.title = connected ? `当前模型：${status}` : '请在高级模式中配置 API';
    };
    const renderGenerationState = (generating) => {
        if (generationWatchdog) window.clearTimeout(generationWatchdog);
        if (generationSyncTimer) window.clearInterval(generationSyncTimer);
        state.generating = generating;
        if (generating) {
            generationStartedAt = performance.now();
            generationWatchdog = window.setTimeout(() => renderGenerationState(false), 60000);
            generationSyncTimer = window.setInterval(() => {
                if (!state.generating || performance.now() - generationStartedAt < 800) return;
                const nativeGenerating = context.isGenerating?.();
                const latest = latestAssistantMessage();
                const text = latest?.querySelector('.mes_text')?.textContent?.trim() || '';
                const assistantFinished = latest && hasAssistantText(text) && (
                    latest !== state.previousAssistantMessage ||
                    text !== state.previousAssistantText
                );
                if (nativeGenerating === false && assistantFinished) finishGeneration();
            }, 250);
        } else {
            generationWatchdog = null;
            generationSyncTimer = null;
        }
        if (!generating) {
            state.completionObserver?.disconnect();
            state.completionObserver = null;
            state.previousAssistantMessage = null;
        }
        chatStatus.textContent = generating ? '正在回复...' : '正在这个场景中陪伴你';
        sendButton.disabled = false;
        micButton.disabled = generating;
        shell.querySelector('.consumer-composer').classList.toggle('is-generating', generating);
        sendButton.dataset.consumerAction = generating ? 'generating' : 'send';
        sendButton.title = generating ? '正在生成' : '发送';
        sendButton.setAttribute('aria-label', sendButton.title);
        sendButton.innerHTML = `<i class="fa-solid ${generating ? 'fa-spinner consumer-generating-spinner' : 'fa-arrow-up'}"></i>`;
    };
    const setRecordingState = (recording, message = '') => {
        state.recording = recording;
        micButton.classList.toggle('is-recording', recording);
        micButton.setAttribute('aria-pressed', String(recording));
        recordingStatus.textContent = message;
    };
    const setVoiceMode = (enabled) => {
        state.voiceMode = enabled;
        input.classList.toggle('is-voice-mode', enabled);
        micButton.classList.toggle('is-voice-mode', enabled);
        micButton.title = enabled ? '退出语音输入' : '启用语音输入';
        micButton.setAttribute('aria-label', micButton.title);
        micButton.setAttribute('aria-pressed', String(enabled));
        input.placeholder = enabled ? input.dataset.voicePlaceholder : input.dataset.textPlaceholder;
        shell.querySelector('[data-consumer-input-mode]').textContent = enabled ? '语音输入' : '文字输入';
        if (!enabled && state.recording) {
            state.recognition?.stop();
            setRecordingState(false);
        }
    };
    const startVoiceInput = () => {
        const Recognition = window.SpeechRecognition || window.webkitSpeechRecognition;
        if (!Recognition) {
            setVoiceMode(false);
            window.toastr?.info('当前浏览器不支持语音输入，请使用 Chrome 或 Edge');
            return;
        }
        if (!state.voiceMode) setVoiceMode(true);
        if (state.recording) {
            state.recognition?.stop();
            return;
        }
        if (state.generating) return;
        const recognition = new Recognition();
        state.recognition = recognition;
        recognition.lang = 'zh-CN';
        recognition.interimResults = true;
        recognition.continuous = false;
        recognition.onstart = () => setRecordingState(true, '正在听...');
        recognition.onresult = (event) => {
            const transcript = [...event.results].map(result => result[0]?.transcript || '').join('');
            input.value = transcript;
            input.dispatchEvent(new Event('input', { bubbles: true }));
        };
        recognition.onerror = (event) => {
            const message = event.error === 'not-allowed' ? '麦克风权限未开启' : '没有听清，请再试一次';
            setRecordingState(false, message);
        };
        recognition.onend = () => {
            setRecordingState(false);
            state.recognition = null;
            input.focus();
        };
        try {
            recognition.start();
        } catch (error) {
            console.warn('Voice input could not start', error);
            setRecordingState(false, '语音输入暂不可用');
        }
    };
    const sendMessage = async () => {
        const originalInput = document.querySelector('#send_textarea');
        const message = input.value.trim();
        if (!message || !originalInput) return;
        if (state.generating) {
            // The shell can miss a native completion event when a provider
            // closes a stream during a chat reload. Trust the native mutex
            // before rejecting a new Enter press.
            if (context.isGenerating?.()) {
                window.toastr?.info('上一条消息仍在生成，请稍候');
                return;
            }
            finishGeneration();
        }
        if (!state.connected) {
            window.toastr?.error('请先在高级模式中连接 AI');
            return;
        }
        state.previousAssistantMessage = latestAssistantMessage();
        state.previousAssistantText = state.previousAssistantMessage?.querySelector('.mes_text')?.textContent?.trim() || '';
        state.completionObserver?.disconnect();
        state.completionObserver = new MutationObserver(() => {
            const latest = latestAssistantMessage();
            const text = latest?.querySelector('.mes_text')?.textContent?.trim();
            if (latest && hasAssistantText(text) && (
                latest !== state.previousAssistantMessage ||
                text !== state.previousAssistantText
            )) {
                finishGeneration();
            }
        });
        state.completionObserver.observe(originalChat, { childList: true, subtree: true, characterData: true });
        originalInput.value = message;
        originalInput.dispatchEvent(new Event('input', { bubbles: true }));
        input.value = '';
        input.style.height = '';
        renderGenerationState(true);
        try {
            const generation = context.sendTextareaMessage?.();
            if (generation && typeof generation.then === 'function') {
                await generation;
            }
        } catch (error) {
            console.error('Consumer chat generation failed', error);
            const message = error?.error?.message || error?.message || 'AI 暂时无法回复，请稍后再试';
            window.toastr?.error(message, '生成失败', { timeOut: 10000 });
        } finally {
            // GENERATION_ENDED is not emitted for every provider/network failure.
            // Always release the consumer shell controls when its request settles.
            finishGeneration();
        }
    };
    const openChat = async () => {
        const index = selectedIndex();
        if (index === null || index < 0) return;
        // A chat switch must never inherit a stale generation lock from the
        // previous view or a request that ended before its UI event arrived.
        finishGeneration();
        await context.selectCharacterById(index, { switchMenu: false });
        const key = characterKey(selectedCharacter());
        state.recents = [key, ...state.recents.filter(item => item !== key)].slice(0, 12);
        writeStoredList(RECENTS_KEY, state.recents);
        renderChatHeader();
        showView('chat');
        requestAnimationFrame(() => originalChat?.scrollTo({ top: originalChat.scrollHeight, behavior: 'instant' }));
        input.focus();
    };
    const clickOriginal = (selector) => {
        const button = document.querySelector(selector);
        if (!button) {
            window.toastr?.info('这个功能暂时不可用');
            return false;
        }
        button.click();
        return true;
    };
    const copyLatestMessage = async () => {
        const text = latestAssistantText();
        if (!text) {
            window.toastr?.info('还没有可以复制的 AI 回复');
            return;
        }
        try {
            await navigator.clipboard.writeText(text);
            window.toastr?.success('已复制最近一条回复');
        } catch {
            const fallback = document.createElement('textarea');
            fallback.value = text;
            fallback.style.position = 'fixed';
            fallback.style.opacity = '0';
            document.body.append(fallback);
            fallback.select();
            document.execCommand('copy');
            fallback.remove();
            window.toastr?.success('已复制最近一条回复');
        }
    };
    const openNoteSheet = () => {
        noteInput.value = context.chatMetadata?.consumer_note || '';
        if (typeof noteSheet.showModal === 'function') noteSheet.showModal();
        else noteSheet.setAttribute('open', '');
        requestAnimationFrame(() => noteInput.focus());
    };
    const saveChatNote = async () => {
        if (!context.chatMetadata) return;
        context.chatMetadata.consumer_note = noteInput.value.trim();
        await context.saveMetadataDebounced?.();
        window.toastr?.success('聊天备注已保存');
    };
    noteSheet?.querySelector('form')?.addEventListener('submit', async (event) => {
        if (event.submitter?.value !== 'save') return;
        event.preventDefault();
        await saveChatNote();
        noteSheet.close();
    });
    autoVoiceButton.setAttribute('aria-pressed', String(state.autoVoice));
    autoVoiceButton.classList.toggle('is-active', state.autoVoice);
    autoVoiceButton.title = state.autoVoice ? '关闭自动朗读' : '自动朗读 AI 回复';
    autoVoiceButton.setAttribute('aria-label', autoVoiceButton.title);
    shell.addEventListener('click', async (event) => {
        const moreMenu = shell.querySelector('[data-consumer-more-menu]');
        const moreAction = event.target.closest('[data-consumer-action="more"]');
        if (moreAction) {
            moreMenu.hidden = !moreMenu.hidden;
            return;
        }
        if (moreMenu && !event.target.closest('[data-consumer-more-menu]')) moreMenu.hidden = true;
        const card = event.target.closest('[data-consumer-character]');
        const favoriteButton = event.target.closest('[data-consumer-character-favorite]');
        if (favoriteButton) {
            state.selectedKey = characterKey(state.characters[Number(favoriteButton.dataset.consumerCharacterFavorite)], Number(favoriteButton.dataset.consumerCharacterFavorite));
            toggleFavorite();
            event.stopPropagation();
            return;
        }
        if (card) {
            const index = Number(card.dataset.consumerCharacter);
            state.selectedKey = characterKey(state.characters[index], index);
            renderDetail();
            showView('detail');
            return;
        }
        const action = event.target.closest('[data-consumer-action]')?.dataset.consumerAction;
        if (action === 'create') {
            enterAdvancedMode();
            document.querySelector('#rm_button_create')?.click();
            return;
        }
        if (action === 'creator') {
            showView('creator');
            return;
        }
        if (action === 'advanced') {
            enterAdvancedMode();
            return;
        }
        if (action === 'consumer') {
            enterConsumerMode();
            showView(state.selectedKey === null ? 'discover' : 'chat');
            return;
        }
        if (action === 'favorite') {
            toggleFavorite();
            return;
        }
        if (action === 'manage-media') {
            openMediaSheet();
            return;
        }
        if (action === 'history') {
            state.filter = 'recent';
            shell.querySelectorAll('[data-consumer-filter]').forEach((tab) => {
                const active = tab.dataset.consumerFilter === state.filter;
                tab.classList.toggle('is-active', active);
                tab.setAttribute('aria-selected', String(active));
            });
            showView('discover');
            renderCards();
            return;
        }
        if (action === 'chat-history') {
            enterAdvancedMode();
            clickOriginal('#option_select_chat');
            return;
        }
        if (action === 'new-chat') {
            clickOriginal('#option_start_new_chat');
            return;
        }
        if (action === 'chat-note') {
            openNoteSheet();
            return;
        }
        if (action === 'regenerate') {
            if (!latestAssistantMessage()) {
                window.toastr?.info('还没有可以重新生成的回复');
                return;
            }
            clickOriginal('#option_regenerate');
            return;
        }
        if (action === 'copy-latest') {
            await copyLatestMessage();
            return;
        }
        if (action === 'discover') {
            showView('discover');
            return;
        }
        if (action === 'start') await openChat();
        if (action === 'send') await sendMessage();
        if (action === 'stop') {
            context.stopGeneration?.();
            renderGenerationState(false);
        }
        if (action === 'voice') await speakLatestMessage(context);
        if (action === 'record') {
            if (state.voiceMode && !state.recording) setVoiceMode(false);
            else startVoiceInput();
        }
        if (action === 'auto-voice') {
            state.autoVoice = !state.autoVoice;
            writeStoredList(AUTO_VOICE_KEY, state.autoVoice ? ['on'] : []);
            autoVoiceButton.setAttribute('aria-pressed', String(state.autoVoice));
            autoVoiceButton.classList.toggle('is-active', state.autoVoice);
            autoVoiceButton.title = state.autoVoice ? '关闭自动朗读' : '自动朗读 AI 回复';
            autoVoiceButton.setAttribute('aria-label', autoVoiceButton.title);
            if (state.autoVoice) void speakLatestMessage(context);
        }
    });
    input.addEventListener('input', () => {
        input.style.height = 'auto';
        input.style.height = `${Math.min(input.scrollHeight, 140)}px`;
    });
    input.addEventListener('keydown', (event) => {
        if (event.key === 'Enter' && !event.shiftKey && !state.voiceMode) {
            event.preventDefault();
            event.stopPropagation();
            void sendMessage();
        }
    });
    shell.querySelector('.consumer-search').addEventListener('input', (event) => {
        state.query = event.target.value;
        renderCards();
    });
    shell.querySelector('[data-consumer-sort]').addEventListener('change', renderCards);
    shell.querySelectorAll('[data-consumer-filter]').forEach((tab) => {
        tab.addEventListener('click', () => {
            state.filter = tab.dataset.consumerFilter;
            shell.querySelectorAll('[data-consumer-filter]').forEach((item) => {
                const active = item === tab;
                item.classList.toggle('is-active', active);
                item.setAttribute('aria-selected', String(active));
            });
            renderCards();
        });
    });
    shell.querySelector('[data-consumer-tags]').addEventListener('click', (event) => {
        const button = event.target.closest('[data-consumer-tag]');
        if (!button) return;
        state.tag = button.dataset.consumerTag;
        renderTagFilters();
        renderCards();
    });
    document.querySelector('#consumer_media_button')?.addEventListener('click', () => {
        const currentId = Number(context.characterId);
        if (Number.isInteger(currentId) && currentId >= 0 && state.characters[currentId]) {
            state.selectedKey = characterKey(state.characters[currentId], currentId);
        }
        enterConsumerMode();
        openMediaSheet();
    });
    document.querySelector('[data-consumer-editor-media-input]')?.addEventListener('change', async (event) => {
        await uploadEditorMedia([...event.target.files]);
        event.target.value = '';
    });
    document.querySelector('[data-consumer-editor-media-list]')?.addEventListener('click', async (event) => {
        const button = event.target.closest('[data-consumer-editor-media-action]');
        if (!button) return;
        const index = Number(button.dataset.consumerEditorMediaIndex ?? button.dataset.consumerMediaIndex);
        if (!state.editorMedia.gallery[index]) return;
        const action = button.dataset.consumerEditorMediaAction;
        const removed = action === 'delete' ? state.editorMedia.gallery.splice(index, 1)[0] : null;
        if (action === 'up' && index > 0) [state.editorMedia.gallery[index - 1], state.editorMedia.gallery[index]] = [state.editorMedia.gallery[index], state.editorMedia.gallery[index - 1]];
        if (action === 'down' && index < state.editorMedia.gallery.length - 1) [state.editorMedia.gallery[index + 1], state.editorMedia.gallery[index]] = [state.editorMedia.gallery[index], state.editorMedia.gallery[index + 1]];
        if (action === 'cover') state.editorMedia.cover = state.editorMedia.gallery[index].url;
        if (removed && state.editorMedia.cover === removed.url) state.editorMedia.cover = state.editorMedia.gallery[0]?.url || '';
        renderEditorMedia();
        const saved = await saveEditorMedia();
        if (saved && removed && isUploadedImagePath(removed.url)) {
            await fetch('/api/images/delete', {
                method: 'POST',
                headers: context.getRequestHeaders?.(),
                body: JSON.stringify({ path: removed.url }),
            }).catch(error => console.warn('Failed to delete editor character image file', error));
        }
    });
    document.querySelector('#rm_button_create')?.addEventListener('click', () => loadEditorMedia());
    shell.querySelector('[data-consumer-detail-gallery]').addEventListener('click', (event) => {
        const button = event.target.closest('[data-consumer-detail-image]');
        if (!button) return;
        const image = shell.querySelector('[data-consumer-detail-avatar]');
        image.src = mediaUrl(button.dataset.consumerDetailImage);
        shell.querySelectorAll('[data-consumer-detail-image]').forEach(item => item.classList.toggle('is-active', item === button));
    });
    mediaSheet.querySelector('[data-consumer-media-input]').addEventListener('change', async (event) => {
        await uploadMedia([...event.target.files]);
        event.target.value = '';
    });
    mediaSheet.querySelector('[data-consumer-media-list]').addEventListener('click', async (event) => {
        const button = event.target.closest('[data-consumer-media-action]');
        if (!button) return;
        const character = selectedCharacter();
        if (!character) return;
        const media = characterMedia(context, character);
        const index = Number(button.dataset.consumerMediaIndex);
        const action = button.dataset.consumerMediaAction;
        if (!media.gallery[index]) return;
        if (action === 'delete') {
            if (media.gallery.length === 1) {
                window.toastr?.info('至少保留一张角色图片');
                return;
            }
            const [removed] = media.gallery.splice(index, 1);
            const nextCover = media.cover === removed.url ? media.gallery[0].url : media.cover;
            const saved = await saveCharacterMedia(media.gallery, nextCover);
            if (saved && isUploadedImagePath(removed.url)) {
                try {
                    await fetch('/api/images/delete', {
                        method: 'POST',
                        headers: context.getRequestHeaders?.(),
                        body: JSON.stringify({ path: removed.url }),
                    });
                } catch (error) {
                    console.warn('Failed to delete character image file', error);
                }
            }
            return;
        }
        if (action === 'up' && index > 0) [media.gallery[index - 1], media.gallery[index]] = [media.gallery[index], media.gallery[index - 1]];
        if (action === 'down' && index < media.gallery.length - 1) [media.gallery[index + 1], media.gallery[index]] = [media.gallery[index], media.gallery[index + 1]];
        if (action === 'cover') media.cover = media.gallery[index].url;
        await saveCharacterMedia(media.gallery, media.cover);
    });
    const syncCharacters = () => {
        state.characters = Array.isArray(context.characters) ? context.characters.filter(Boolean) : [];
        const creatorCount = shell.querySelector('[data-consumer-creator-count]');
        if (creatorCount) creatorCount.textContent = `${state.characters.length} 个角色`;
        renderTagFilters();
        renderCards();
    };
    syncCharacters();
    setBackground();
    renderConnectionStatus();
    enterConsumerMode();
    context.eventSource?.on(context.eventTypes.APP_READY, syncCharacters);
    context.eventSource?.on(context.eventTypes.CHARACTER_PAGE_LOADED, syncCharacters);
    context.eventSource?.on(context.eventTypes.CHARACTER_EDITOR_OPENED, (characterId) => loadEditorMedia(characterId));
    context.eventSource?.on(context.eventTypes.CHARACTER_EDITED, async (event) => {
        const characterId = Number(event?.detail?.id);
        if (!Number.isInteger(characterId) || characterId < 0) return;
        if (state.editorMedia.characterId === null && state.editorMedia.gallery.length) {
            state.editorMedia.characterId = characterId;
            await saveEditorMedia();
        } else if (state.editorMedia.characterId === characterId) {
            loadEditorMedia(characterId);
        }
        syncCharacters();
    });
    context.eventSource?.on(context.eventTypes.ONLINE_STATUS_CHANGED, renderConnectionStatus);
    context.eventSource?.on(context.eventTypes.CHAT_CHANGED, () => {
        if (noteSheet?.open) noteInput.value = context.chatMetadata?.consumer_note || '';
    });
    context.eventSource?.on(context.eventTypes.GENERATION_STARTED, () => renderGenerationState(true));
    context.eventSource?.on(context.eventTypes.GENERATION_ENDED, finishGeneration);
    context.eventSource?.on(context.eventTypes.GENERATION_STOPPED, finishGeneration);
    context.eventSource?.on(context.eventTypes.MESSAGE_RECEIVED, finishGeneration);
    renderGenerationState(false);
    context.eventSource?.on(context.eventTypes.CHARACTER_MESSAGE_RENDERED, (messageId) => {
        // Some OpenAI-compatible gateways render the final message without
        // emitting GENERATION_ENDED. The rendered assistant message is still
        // an authoritative completion signal for the consumer shell.
        const latest = latestAssistantMessage();
        const text = latest?.querySelector('.mes_text')?.textContent?.trim();
        if (state.generating && latest && hasAssistantText(text) && (
            latest !== state.previousAssistantMessage ||
            text !== state.previousAssistantText
        )) {
            finishGeneration();
        }
        if (state.autoVoice) {
            if (messageId === state.lastSpokenMessageId) return;
            state.lastSpokenMessageId = messageId;
            window.setTimeout(() => void speakLatestMessage(context), 80);
        }
    });
}

let bootAttempts = 0;

const boot = () => {
    const context = getContext?.();
    if (document.querySelector('#consumer-shell')) return;
    if (!context) {
        if (bootAttempts++ < 100) {
            window.setTimeout(boot, 100);
        }
        return;
    }
    initConsumerShell(context);
};

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => setTimeout(boot, 0), { once: true });
} else {
    setTimeout(boot, 0);
}
