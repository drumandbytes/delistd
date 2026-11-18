// The brokers page: starting a send, following its progress, and keeping the
// list in step. Job tracking itself is window.delistdJobPoll (layout.html);
// this reacts to its "delistd:job-update" events.
(function () {
    'use strict';

    const $ = (id) => document.getElementById(id);
    const csrf = () => document.querySelector('meta[name="csrf-token"]')?.content || '';
    const progressArea = () => $('send-all-progress');
    let lastSeenBrokerId = null;

    // the filter bar is a form, so a refresh sends its current values and the
    // search and filters survive it
    const refreshList = () => htmx.ajax('GET', '/api/brokers', {target: '#broker-list', swap: 'innerHTML', source: '#broker-filters'});

    function esc(text) {
        const el = document.createElement('div');
        el.textContent = text;
        return el.innerHTML;
    }

    const buttons = {
        retry: {btn: 'retry-failed-btn', text: 'retry-failed-text', spinner: 'retry-failed-spinner', label: 'Retry Failed'},
        all: {btn: 'send-all-btn', text: 'send-all-text', spinner: 'send-all-spinner', label: 'Send to All'},
    };

    function setDisabled(disabled) {
        for (const b of Object.values(buttons)) {
            const el = $(b.btn);
            if (el) el.disabled = disabled;
        }
    }

    function resetButtons() {
        setDisabled(false);
        for (const b of Object.values(buttons)) {
            if ($(b.text)) $(b.text).textContent = b.label;
            if ($(b.spinner)) $(b.spinner).classList.add('hidden');
        }
    }

    function showProgressCard() {
        progressArea().innerHTML = `
            <div class="card p-4">
                <div class="flex items-center justify-between mb-2">
                    <span class="font-medium text-primary">Sending emails...</span>
                    <button onclick="cancelJob()" class="btn btn-ghost text-sm py-1 px-3 text-error">Cancel</button>
                </div>
                <div id="current-broker" class="text-sm text-secondary mb-2"></div>
                <div class="w-full rounded-full h-2 mb-2" style="background: var(--bg-elevated);">
                    <div id="progress-bar" class="h-2 rounded-full transition-all" style="width: 0%; background: var(--accent);"></div>
                </div>
                <div id="progress-text" class="text-xs text-muted">0 of 0 complete (0 sent, 0 failed)</div>
            </div>`;
    }

    function showProgress(job) {
        if ($('current-broker')) $('current-broker').textContent = job.current_broker ? `Sending to: ${job.current_broker}` : '';
        if ($('progress-bar')) $('progress-bar').style.width = job.progress + '%';
        if ($('progress-text')) {
            $('progress-text').textContent = `${job.sent + job.failed} of ${job.total} complete (${job.sent} sent, ${job.failed} failed)`;
        }
    }

    function showMessage(html) { progressArea().innerHTML = html; }

    function showError(message) {
        showMessage(`<div class="alert alert-error"><strong>Error:</strong> ${esc(message)}</div>`);
    }

    function showLost() {
        showMessage(`
            <div class="alert alert-warning px-4 py-3 rounded">
                <strong>Lost track of this send.</strong> It may have finished, or you switched profiles mid-send.
                <a href="/brokers" class="underline ml-2">Refresh page</a> to check the latest status.
            </div>`);
    }

    function showFinished(job) {
        const left = job.total - job.sent - job.failed;
        if (job.error_type === 'auth') {
            showMessage(`
                <div class="alert alert-error">
                    <div class="flex items-start gap-3">
                        ${window.brokerIcons.warning}
                        <div>
                            <h3 class="font-semibold" style="color: var(--text-primary);">Email Authentication Failed</h3>
                            <p class="text-sm mt-1">Sent ${job.sent} emails before stopping. Your email provider blocked further sends.</p>
                            <div class="mt-3 p-3 rounded" style="background: var(--bg-surface); border: 1px solid var(--border);">
                                <p class="font-medium mb-2" style="color: var(--text-primary);">What to do:</p>
                                <ol class="text-sm space-y-1 list-decimal list-inside" style="color: var(--text-secondary);">
                                    <li>Check your email account for security alerts</li>
                                    <li>Generate a new app password with your provider and update it in <a href="/settings" class="text-accent hover:underline">Settings</a></li>
                                    <li>Wait 24 hours before trying again (rate limits reset daily)</li>
                                    <li>Check your <a href="/settings" class="text-accent hover:underline">SMTP settings</a> - some providers cap how many emails an app password can send per day</li>
                                </ol>
                            </div>
                            <p class="text-sm mt-3" style="color: var(--text-secondary);">
                                <strong>${left} brokers remaining.</strong> Filter by "Pending" status to send to them later.
                            </p>
                        </div>
                    </div>
                </div>`);
        } else if (job.status === 'paused') {
            showMessage(`
                <div class="alert alert-info">
                    <div class="flex items-start gap-3">
                        ${window.brokerIcons.clock}
                        <div>
                            <h3 class="font-semibold" style="color: var(--text-primary);">Daily Limit Reached</h3>
                            <p class="text-sm mt-1">Sent ${job.sent} emails today.</p>
                            <p class="text-sm mt-2" style="color: var(--text-secondary);">${left} brokers left. They go out with the next automatic run (<a href="/settings#automation" class="underline">Settings &rarr; Automation</a>), or click Send all again tomorrow.</p>
                        </div>
                    </div>
                </div>`);
        } else {
            const cancelled = job.status === 'cancelled';
            showMessage(`
                <div class="alert ${cancelled ? 'alert-warning' : 'alert-success'} px-4 py-3 rounded">
                    <strong>${cancelled ? 'Cancelled' : 'Complete'}!</strong> Sent ${job.sent} emails (${job.failed} failed).
                    <a href="/brokers" class="underline ml-2">Refresh page</a>
                </div>`);
        }
    }

    // During a send only the row that just finished is refreshed, not the
    // whole ~700-row table on every poll.
    function refreshRow(brokerId) {
        if (!$('status-' + brokerId)) return; // not in the current view; the refresh at the end covers it
        htmx.ajax('GET', `/api/brokers/${brokerId}/status`, {target: '#status-' + brokerId, swap: 'innerHTML'});
    }

    function onJobUpdate(job) {
        // build the card on the first update, whatever its status: the job may
        // have finished before this page's first poll
        if (!$('progress-bar')) showProgressCard();

        if (job.status === 'error' && job.error_type === 'lost') {
            // polling failed (the job vanished: a profile switch, or it aged
            // out), so there is no real progress to show
            showLost();
            resetButtons();
            refreshList();
            return;
        }

        showProgress(job);
        if (job.current_broker_id && job.current_broker_id !== lastSeenBrokerId) {
            if (lastSeenBrokerId) refreshRow(lastSeenBrokerId);
            lastSeenBrokerId = job.current_broker_id;
        }

        if (job.status !== 'running') {
            if (lastSeenBrokerId) {
                refreshRow(lastSeenBrokerId);
                lastSeenBrokerId = null;
            }
            showFinished(job);
            resetButtons();
            refreshList(); // catches anything the per-row updates missed (several brokers within one poll)
        }
    }

    // "failed" is the Retry Failed button: only brokers whose last attempt
    // failed. With no argument the "Advanced send filters" status applies.
    window.startSendAll = async function (statusOverride) {
        const b = statusOverride === 'failed' ? buttons.retry : buttons.all;
        setDisabled(true);
        $(b.text).textContent = 'Starting...';
        $(b.spinner).classList.remove('hidden');

        const status = statusOverride || $('send-status').value;
        if (status === 'all' && !confirm('Send to all matching brokers regardless of send history?')) {
            resetButtons();
            return;
        }
        const params = {
            search: $('broker-search').value,
            category: document.querySelector('[name="category"]').value,
            region: document.querySelector('[name="region"]').value,
            status,
            broker_ids: $('send-broker-ids').value,
            exclude_ids: $('send-exclude-ids').value,
        };

        try {
            const resp = await fetch('/api/send-all', {
                method: 'POST',
                headers: {'Content-Type': 'application/x-www-form-urlencoded', 'X-CSRF-Token': csrf()},
                body: new URLSearchParams(params),
            });
            const data = await resp.json();
            if (!resp.ok) {
                showError(data.error || 'Failed to start sending');
                resetButtons();
                return;
            }
            showProgressCard();
            window.delistdJobPoll.start(data.job_id);
        } catch (err) {
            showError('Network error: ' + err.message);
            resetButtons();
        }
    };

    window.cancelJob = async function () {
        const jobId = window.delistdJobPoll.jobId;
        if (!jobId) return;
        try {
            await fetch(`/api/job/${jobId}/cancel`, {method: 'POST', headers: {'X-CSRF-Token': csrf()}});
        } catch (err) {
            console.error('Error cancelling job:', err);
        }
    };

    window.addEventListener('delistd:job-update', (e) => onJobUpdate(e.detail));
})();
