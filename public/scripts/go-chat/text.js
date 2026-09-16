/** 聊天文本使用已有 Markdown 库，禁止角色或回复注入 HTML、远程图片及脚本。 */
const converter = new globalThis.showdown.Converter({ simpleLineBreaks: true, strikethrough: true });

/** 先转义原始 HTML，再只允许基础排版标签；图片与链接不产生外部请求。 */
export function messageText(value) {
    const safe = String(value).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    const html = converter.makeHtml(safe);
    return globalThis.DOMPurify.sanitize(html, {
        ALLOWED_TAGS: ['p', 'br', 'em', 'strong', 'del', 'blockquote', 'pre', 'code', 'ul', 'ol', 'li', 'hr'],
        ALLOWED_ATTR: [],
        RETURN_DOM_FRAGMENT: true,
    });
}
