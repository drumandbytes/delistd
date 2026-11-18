// Settings page: the buttons that act without leaving the page.
(function () {
    'use strict';
    const csrf = () => document.querySelector('meta[name="csrf-token"]')?.content || '';
    const esc = (text) => {
        const el = document.createElement('div');
        el.textContent = text;
        return el.innerHTML;
    };

    // show or hide a form (the inbox editor)
    document.querySelectorAll('[data-toggle]').forEach((btn) =>
        btn.addEventListener('click', () => document.querySelector(btn.dataset.toggle).classList.toggle('hidden')));
    document.querySelectorAll('[data-hide]').forEach((btn) =>
        btn.addEventListener('click', () => document.querySelector(btn.dataset.hide).classList.add('hidden')));

    document.querySelectorAll('[data-delete-profile]').forEach((btn) => btn.addEventListener('click', async () => {
        const id = btn.dataset.deleteProfile;
        if (!confirm(`Remove profile "${id}"? This does not delete its existing send history - just the profile itself.`)) return;
        try {
            const resp = await fetch('/settings/profiles/' + encodeURIComponent(id) + '/delete', {method: 'POST', headers: {'X-CSRF-Token': csrf()}});
            if (resp.ok) {
                window.location.href = '/settings';
            } else {
                alert('Failed to delete profile: ' + await resp.text());
            }
        } catch (err) {
            alert('Network error: ' + err.message);
        }
    }));

    const clear = document.getElementById('clear-history-btn');
    if (clear) clear.addEventListener('click', async () => {
        if (!confirm('Delete ALL send history (sent + failed + pending records)? This cannot be undone. Your broker database and settings are not affected.')) return;
        const result = document.getElementById('danger-zone-result');
        clear.disabled = true;
        clear.textContent = 'Clearing...';
        try {
            const resp = await fetch('/api/history', {method: 'DELETE', headers: {'X-CSRF-Token': csrf()}});
            const data = await resp.json();
            if (!resp.ok) throw new Error(data.error || 'Failed to clear history');
            result.innerHTML = `<div class="alert alert-success">${esc(data.message)}.</div>`;
        } catch (err) {
            result.innerHTML = `<div class="alert alert-error">Error: ${esc(err.message)}</div>`;
        }
        clear.disabled = false;
        clear.innerHTML = window.clearHistoryLabel;
    });
})();
