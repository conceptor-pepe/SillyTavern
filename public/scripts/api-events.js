/** 解析 Go 生成事件；只有完整的 message_end 才代表消息已持久化。 */

/** 消费分块字节流，异常或回调失败时取消底层读取并释放锁。 */
export async function readEvents(body, handlers = {}) {
    const reader = body.getReader();
    const decoder = new TextDecoder();
    const state = { buffer: '', name: '', data: [], ended: false };
    let drained = false;
    let failure;
    try {
        while (!drained) {
            const { value, done } = await reader.read();
            drained = done;
            state.buffer += done ? decoder.decode() : decoder.decode(value, { stream: true });
            readLines(state, handlers, done);
        }
        if (!state.ended) throw streamError('STREAM_INTERRUPTED', 'Generation stream ended before completion');
    } catch (error) {
        failure = error;
    } finally {
        try {
            if (!drained) await reader.cancel();
        } catch (error) {
            // 清理失败不能覆盖生成错误、用户中止或 UI 回调的原始异常。
            failure ??= error;
        } finally {
            reader.releaseLock();
        }
    }
    if (failure) throw failure;
}

/** 延迟处理块尾 CR，避免把跨块 CRLF 错认成两次换行。 */
function readLines(state, handlers, eof) {
    let match;
    while ((match = /\r\n|\r|\n/.exec(state.buffer))) {
        if (!eof && match[0] === '\r' && match.index === state.buffer.length - 1) return;
        const line = state.buffer.slice(0, match.index);
        state.buffer = state.buffer.slice(match.index + match[0].length);
        readLine(state, line, handlers);
    }
}

/** 空行分发事件，多行 data 合并后再解析，注释和扩展字段忽略。 */
function readLine(state, line, handlers) {
    if (line === '') {
        dispatchEvent(state, handlers);
        state.name = '';
        state.data = [];
        return;
    }
    const colon = line.indexOf(':');
    const key = colon < 0 ? line : line.slice(0, colon);
    const value = colon < 0 ? '' : line.slice(colon + 1).replace(/^ /, '');
    if (key === 'event') state.name = value;
    if (key === 'data') state.data.push(value);
}

/** 错误事件通知界面后仍拒绝 Promise，防止调用方把失败当成完成。 */
function dispatchEvent(state, handlers) {
    if (!state.name || !state.data.length) return;
    let value;
    try {
        value = JSON.parse(state.data.join('\n'));
    } catch {
        throw streamError('INVALID_EVENT', 'Invalid generation event');
    }
    handlers[state.name]?.(value);
    if (state.name === 'generation_error') {
        throw streamError(value?.code || 'GENERATION_FAILED', value?.message || 'Generation failed');
    }
    if (state.name === 'message_end') state.ended = true;
}

/** 使用稳定错误码区分服务端拒绝、协议异常和提前断流。 */
function streamError(code, message) {
    return Object.assign(new Error(message), { code });
}
