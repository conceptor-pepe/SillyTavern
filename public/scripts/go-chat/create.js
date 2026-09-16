/** 角色创建页负责表单预览和图片压缩；创建请求仍由入口统一处理。 */
import { $, element, notice } from './view.js';

let imageData = '';
let imageBusy = false;

/** 切换为独立创建页，未提交内容仅保留在当前表单。 */
export function showCreate() {
    $('#library-view').hidden = true;
    $('#chat-view').hidden = true;
    $('#create-view').hidden = false;
    document.body.classList.remove('in-chat');
    document.body.classList.add('in-create');
    $('#library').setAttribute('aria-current', 'false');
    $('#create-character').setAttribute('aria-current', 'page');
    updatePreview();
}

/** 返回与 Go 创建接口一致的字段，禁止把 File 对象或临时路径交给后端。 */
export function characterBody(form) {
    if (imageBusy) throw new Error('封面正在处理，请稍候');
    const body = Object.fromEntries(new FormData(form));
    body.tags = [...new Set(body.tags.split(/[,，、]/).map(tag => tag.trim()).filter(Boolean))];
    if (body.tags.length > 9 || body.tags.some(tag => [...tag].length > 24)) throw new Error('最多 9 个标签，每个标签最多 24 字');
    body.portrait = imageData;
    return body;
}

/** 成功创建或退出账号时清除临时资料，避免跨账号残留。 */
export function resetCreate() {
    $('#create-character-form').reset();
    imageData = '';
    updatePreview();
}

/** 预览使用文本节点，角色输入不能执行 HTML。 */
function updatePreview() {
    const form = $('#create-character-form');
    const name = form.elements.name.value || '角色名称';
    $('#preview-name').textContent = name;
    $('#preview-chat-name').textContent = name;
    $('#preview-description').textContent = form.elements.description.value || '角色简介';
    $('#preview-greeting').textContent = form.elements.first_message.value || '开场白';
    for (const id of ['#upload-preview', '#preview-image', '#preview-chat-image']) $(id).src = imageData || '/img/ai4.png';
    const tags = form.elements.tags.value.split(/[,，、]/).map(tag => tag.trim()).filter(Boolean).slice(0, 9);
    $('#preview-tags').replaceChildren(...tags.map(tag => element('span', tag, 'tag')));
}

/** 浏览器重绘缩略图去除元数据，并限制请求体大小。 */
async function readPortrait(file) {
    if (!file) return '';
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size > 5 * 1024 * 1024) throw new Error('请选择 5 MB 以内的 JPG、PNG 或 WebP 图片');
    const image = await createImageBitmap(file);
    const scale = Math.min(1, 512 / Math.max(image.width, image.height));
    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.round(image.width * scale));
    canvas.height = Math.max(1, Math.round(image.height * scale));
    const context = canvas.getContext('2d');
    context.fillStyle = '#f5f4f7';
    context.fillRect(0, 0, canvas.width, canvas.height);
    context.drawImage(image, 0, 0, canvas.width, canvas.height);
    image.close();
    const data = canvas.toDataURL('image/jpeg', .85);
    if (data.length > 350000) throw new Error('图片过于复杂，请选择较小的图片');
    return data;
}

$('#create-character-form').addEventListener('input', updatePreview);
$('#portrait-file').addEventListener('change', async event => {
    imageBusy = true;
    $('#create-submit').disabled = true;
    try {
        imageData = await readPortrait(event.target.files[0]);
        notice();
    } catch (error) {
        event.target.value = '';
        notice(error.message || '无法读取图片');
    } finally {
        imageBusy = false;
        $('#create-submit').disabled = false;
        updatePreview();
    }
});
document.querySelectorAll('[data-preview]').forEach(button => button.addEventListener('click', () => {
    const chat = button.dataset.preview === 'chat';
    $('#preview-card').hidden = chat;
    $('#preview-chat').hidden = !chat;
    document.querySelectorAll('[data-preview]').forEach(tab => tab.setAttribute('aria-pressed', String(tab === button)));
}));
