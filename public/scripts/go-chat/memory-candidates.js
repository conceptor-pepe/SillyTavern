/** 自动整理只生成候选，用户明确接受后后端才写入正式记忆。 */
import { apiClient } from '../api-client.js';
import { $, element, notice } from './view.js';

export function wireMemoryCandidates({ act, session }) {
    let items = [];

    async function load() {
        const current = session();
        if (!current?.chat) throw new Error('请先打开一段聊天');
        items = (await apiClient.memoryCandidates(current.chat.id)).items || [];
        render();
    }

    function render() {
        const rows = items.map(item => {
            const row = element('article', undefined, 'memory-entry candidate-entry');
            const heading = element('div', undefined, 'memory-entry-heading');
            heading.append(element('strong', item.scope === 'relationship' ? '跨故事关系记忆' : '当前故事记忆'), element('span', '等待确认'));
            const actions = element('div', undefined, 'memory-actions');
            const reject = element('button', '忽略');
            reject.type = 'button';
            reject.dataset.candidateReject = item.id;
            const accept = element('button', '记住', 'primary');
            accept.type = 'button';
            accept.dataset.candidateAccept = item.id;
            actions.append(reject, accept);
            row.append(heading, element('p', item.content), element('small', `依据：${item.evidence}`), actions);
            return row;
        });
        $('#memory-candidate-list').replaceChildren(...(rows.length ? rows : [element('p', '当前没有等待确认的记忆。')]));
    }

    $('#memory-candidates').addEventListener('click', () => void act(async () => {
        await load();
        $('#memory-candidate-dialog').showModal();
    }));
    $('#extract-memory-candidates').addEventListener('click', () => void act(async () => {
        const current = session();
        if (!current.leaf) throw new Error('当前分支还没有可整理的消息');
        items = (await apiClient.extractMemoryCandidates(current.chat.id, { leaf_id: current.leaf })).items || [];
        render();
        notice(items.length ? '候选已生成，请逐条确认。' : '这段对话没有需要长期保存的内容。');
    }));
    $('#memory-candidate-list').addEventListener('click', event => {
        const accept = event.target.closest('[data-candidate-accept]')?.dataset.candidateAccept;
        const reject = event.target.closest('[data-candidate-reject]')?.dataset.candidateReject;
        if (!accept && !reject) return;
        void act(async () => {
            if (accept) await apiClient.acceptMemoryCandidate(accept);
            else await apiClient.rejectMemoryCandidate(reject);
            items = items.filter(item => item.id !== (accept || reject));
            render();
        });
    });
}
