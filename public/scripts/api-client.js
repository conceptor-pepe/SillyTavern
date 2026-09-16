/**
 * Go v1 API client for the separated AI Chat frontend.
 * This module keeps request formatting and response errors in one place.
 */

import { readEvents } from './api-events.js';

const API_ROOT = '/api/v1';

async function readBody(response) {
    const text = await response.text();
    if (!text) return null;
    try {
        return JSON.parse(text);
    } catch {
        return { message: text };
    }
}

async function request(path, options = {}) {
    const response = await fetch(`${API_ROOT}${path}`, {
        credentials: 'include',
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...(options.headers || {}),
        },
    });
    const body = await readBody(response);
    if (!response.ok) {
        const error = new Error(body?.message || body?.error || 'Request failed');
        error.code = body?.code || 'REQUEST_FAILED';
        error.status = response.status;
        throw error;
    }
    return body?.data ?? body;
}

export const apiClient = {
    login: (body) => request('/auth/login', { method: 'POST', body: JSON.stringify(body) }),
    register: (body) => request('/auth/register', { method: 'POST', body: JSON.stringify(body) }),
    logout: () => request('/auth/logout', { method: 'POST' }),
    me: () => request('/me'),
    characters: (query = '') => request(`/characters${query}`),
    character: (id) => request(`/characters/${encodeURIComponent(id)}`),
    createCharacter: (body) => request('/characters', { method: 'POST', body: JSON.stringify(body) }),
    chats: (query = '') => request(`/chats${query}`),
    chat: (id) => request(`/chats/${encodeURIComponent(id)}`),
    createChat: (body) => request('/chats', { method: 'POST', body: JSON.stringify(body) }),
    deleteChat: (id) => request(`/chats/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    messages: (id, query = '') => request(`/chats/${encodeURIComponent(id)}/messages${query}`),
    addMessage: (id, body) => request(`/chats/${encodeURIComponent(id)}/messages`, {
        method: 'POST',
        body: JSON.stringify(body),
    }),
    editMessage: (id, body) => request(`/messages/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: JSON.stringify(body),
    }),
    deleteMessage: (id) => request(`/messages/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    variants: (id, query = '') => request(`/messages/${encodeURIComponent(id)}/variants${query}`),
    selectVariant: (id, variantId) => request(
        `/messages/${encodeURIComponent(id)}/variants/${encodeURIComponent(variantId)}/select`,
        { method: 'POST' },
    ),
    cancelGeneration: (id) => request(`/generations/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    generation: (id) => request(`/generations/${encodeURIComponent(id)}`),
};

export async function streamGeneration(chatId, body, handlers = {}, signal) {
    await streamRequest(`/chats/${encodeURIComponent(chatId)}/generations`, body, handlers, signal);
}

/** 重新生成只创建同父消息的新回复，不覆盖原有消息分支。 */
export async function streamRegeneration(messageId, body, handlers = {}, signal) {
    await streamRequest(`/messages/${encodeURIComponent(messageId)}/regenerate`, body, handlers, signal);
}

/** 统一流请求的错误契约，避免把代理返回的 HTML 当成生成成功。 */
async function streamRequest(path, body, handlers, signal) {
    const response = await fetch(`${API_ROOT}${path}`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
        body: JSON.stringify(body),
        signal,
    });
    if (!response.ok || !response.body) {
        const errorBody = await readBody(response);
        const error = new Error(errorBody?.message || 'Generation failed');
        error.code = errorBody?.code || 'GENERATION_FAILED';
        error.status = response.status;
        throw error;
    }
    const type = response.headers.get('content-type')?.split(';')[0].trim().toLowerCase();
    if (type !== 'text/event-stream') {
        await response.body.cancel();
        throw new Error('Expected an event stream');
    }
    await readEvents(response.body, handlers);
}
