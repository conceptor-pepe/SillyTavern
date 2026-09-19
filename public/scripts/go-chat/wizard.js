/** 分步表单只负责呈现与当前步骤校验，不改变提交数据和浏览器原生校验。 */
export function createWizard(form, options = {}) {
    const panels = [...form.querySelectorAll('[data-wizard-step]')];
    const targets = [...form.querySelectorAll('[data-wizard-target]')];
    const previous = form.querySelector('[data-wizard-previous]');
    const next = form.querySelector('[data-wizard-next]');
    const submit = form.querySelector('[data-wizard-submit]');
    const status = form.querySelector('[data-wizard-status]');
    let current = 0;

    function valid(index) {
        const invalid = panels[index]?.querySelector('input:invalid,textarea:invalid,select:invalid');
        if (!invalid) return true;
        invalid.reportValidity();
        invalid.focus();
        return false;
    }

    function show(index, { focus = false } = {}) {
        current = Math.max(0, Math.min(index, panels.length - 1));
        form.dataset.wizardCurrent = String(current);
        panels.forEach((panel, position) => { panel.hidden = position !== current; });
        targets.forEach((target, position) => {
            target.setAttribute('aria-current', position === current ? 'step' : 'false');
            target.classList.toggle('complete', position < current);
        });
        if (previous) previous.hidden = current === 0;
        if (next) next.hidden = current === panels.length - 1;
        if (submit) submit.hidden = current !== panels.length - 1;
        if (status) status.textContent = `${current + 1} / ${panels.length}`;
        options.changed?.(current, panels.length);
        if (focus) {
            const heading = panels[current]?.querySelector('h2,h3');
            heading?.setAttribute('tabindex', '-1');
            heading?.focus({ preventScroll: true });
        }
        form.scrollTo?.({ top: 0, behavior: 'smooth' });
    }

    previous?.addEventListener('click', () => show(current - 1, { focus: true }));
    next?.addEventListener('click', () => { if (valid(current)) show(current + 1, { focus: true }); });
    targets.forEach((target, index) => target.addEventListener('click', () => {
        if (index <= current || valid(current)) show(index, { focus: true });
    }));

    return {
        current: () => current,
        reset: () => show(0),
        show,
        revealInvalid() {
            const invalid = form.querySelector('input:invalid,textarea:invalid,select:invalid');
            if (!invalid) return true;
            const panel = invalid.closest('[data-wizard-step]');
            show(Math.max(0, panels.indexOf(panel)));
            invalid.reportValidity();
            invalid.focus();
            return false;
        },
    };
}

/** 快捷选项与普通文本框共用一个逗号列表，用户仍可自由输入。 */
export function wireChoiceFields(form) {
    const values = input => input.value.split(/[,，、]/).map(value => value.trim()).filter(Boolean);
    form.querySelectorAll('[data-choice-name]').forEach(group => {
        const input = form.elements[group.dataset.choiceName];
        const render = () => {
            const selected = new Set(values(input));
            group.querySelectorAll('[data-choice-value]').forEach(button => {
                button.setAttribute('aria-pressed', String(selected.has(button.dataset.choiceValue)));
            });
        };
        group.addEventListener('click', event => {
            const button = event.target.closest('[data-choice-value]');
            if (!button) return;
            const selected = new Set(values(input));
            if (selected.has(button.dataset.choiceValue)) selected.delete(button.dataset.choiceValue);
            else selected.add(button.dataset.choiceValue);
            input.value = [...selected].join('，');
            input.dispatchEvent(new Event('input', { bubbles: true }));
            render();
        });
        input.addEventListener('input', render);
        form.addEventListener('reset', () => queueMicrotask(render));
        render();
    });
}
