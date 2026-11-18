// History page: "Clear Failed" deletes the failed requests after a confirmation.
(function () {
    'use strict';
    const btn = document.getElementById('clear-failed-btn');
    const result = document.getElementById('clear-result');
    if (!btn) return;

    function show(kind, html) {
        result.innerHTML = `<div class="alert alert-${kind}">${html}</div>`;
    }
    const esc = (text) => {
        const el = document.createElement('div');
        el.textContent = text;
        return el.innerHTML;
    };

    btn.addEventListener('click', async () => {
        if (!confirm('Delete all failed records? This cannot be undone.')) return;
        btn.disabled = true;
        btn.textContent = 'Clearing...';
        const csrf = document.querySelector('meta[name="csrf-token"]')?.content || '';
        try {
            const resp = await fetch('/api/history/failed', {method: 'DELETE', headers: {'X-CSRF-Token': csrf}});
            const data = await resp.json();
            if (!resp.ok) throw new Error(data.error || 'Failed to clear records');
            show('success', `${esc(data.message)}. <a href="/history" class="underline">Refresh page</a>`);
        } catch (err) {
            show('error', `Error: ${esc(err.message)}`);
        }
        btn.disabled = false;
        btn.innerHTML = window.clearFailedLabel;
    });
})();
