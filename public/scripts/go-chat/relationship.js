/** 关系面板只维护跨故事资料，当前故事剧情继续由会话记忆负责。 */
import { apiClient } from '../api-client.js';
import { $, element, notice } from './view.js';

export function wireRelationship({ act, session }) {
    let relation = null;
    let memories = [];

    async function open() {
        relation = await apiClient.relationship(session().chat.id);
        memories = (await apiClient.relationshipMemories(relation.companion_id)).items || [];
        const form = $('#relationship-form');
        form.elements.stage.value = relation.stage;
        form.elements.narrative.value = relation.narrative || '';
        form.elements.milestones.value = (relation.milestones || []).join('\n');
        renderMemories();
        $('#relationship-dialog').showModal();
    }

    function renderMemories() {
        const rows = memories.map(item => {
            const row = element('article', undefined, 'memory-entry');
            const heading = element('div', undefined, 'memory-entry-heading');
            heading.append(element('strong', '关系记忆'), element('span', item.pinned ? '始终记住' : '按需回忆'));
            const remove = element('button', '删除', 'danger');
            remove.type = 'button';
            remove.dataset.relationshipMemoryDelete = item.id;
            row.append(heading, element('p', item.content), remove);
            return row;
        });
        $('#relationship-memory-list').replaceChildren(...(rows.length ? rows : [element('p', '还没有跨故事记忆。')]));
    }

    $('#relationship-settings').addEventListener('click', () => void act(open));
    $('#relationship-form').addEventListener('submit', event => {
        event.preventDefault();
        void act(async () => {
            const form = event.currentTarget;
            relation = await apiClient.updateRelationship(session().chat.id, {
                stage: form.elements.stage.value,
                narrative: form.elements.narrative.value.trim(),
                milestones: form.elements.milestones.value.split(/\n/).map(value => value.trim()).filter(Boolean),
                expected_revision: relation.revision,
            });
            session().relationship = relation;
            notice('关系档案已同步到这个角色的所有故事。');
        });
    });
    $('#relationship-memory-form').addEventListener('submit', event => {
        event.preventDefault();
        void act(async () => {
            const form = event.currentTarget;
            await apiClient.saveRelationshipMemory(relation.companion_id, {
                kind: 'fact', content: form.elements.content.value.trim(), keywords: [], enabled: true, pinned: true,
            });
            form.reset();
            memories = (await apiClient.relationshipMemories(relation.companion_id)).items || [];
            renderMemories();
        });
    });
    $('#relationship-memory-list').addEventListener('click', event => {
        const id = event.target.closest('[data-relationship-memory-delete]')?.dataset.relationshipMemoryDelete;
        if (!id) return;
        void act(async () => {
            await apiClient.deleteRelationshipMemory(relation.companion_id, id);
            memories = memories.filter(item => item.id !== id);
            renderMemories();
        });
    });

    return { open };
}
