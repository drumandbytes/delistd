// Behaviour every page shares: CSRF on HTMX requests, the phone menu, the
// profile switcher, and polling of a running send job so it stays visible
// wherever you navigate.
(function () {
    'use strict';

    document.addEventListener('DOMContentLoaded', () => {
        const token = document.querySelector('meta[name="csrf-token"]');
        if (token) {
            document.body.addEventListener('htmx:configRequest', (evt) => {
                evt.detail.headers['X-CSRF-Token'] = token.content;
            });
        }

        const menu = document.querySelector('.mobile-menu');
        if (menu) {
            menu.addEventListener('click', () => {
                const nav = document.getElementById('mobile-nav');
                const wasOpen = !nav.classList.contains('hidden');
                nav.classList.toggle('hidden');
                menu.setAttribute('aria-expanded', String(!wasOpen));
            });
        }

        document.querySelectorAll('select[data-autosubmit]').forEach((sel) =>
            sel.addEventListener('change', () => sel.form.submit()));
    });

    // Job polling. The job runs on the server whatever is watching it. A page
    // that starts a job calls window.delistdJobPoll.start(id) and listens for
    // the "delistd:job-update" event on window, so there is one poll loop per
    // page and not one per script.
    let jobId = null;
    let timer = null;

    function showIndicator(job) {
        const el = document.getElementById('global-job-indicator');
        const text = document.getElementById('global-job-indicator-text');
        if (!el || !text) return;
        const running = job && job.status === 'running';
        if (running) text.textContent = `Sending ${job.sent + job.failed}/${job.total}`;
        el.classList.toggle('hidden', !running);
        el.classList.toggle('flex', running);
    }

    function stop() {
        if (timer) clearInterval(timer);
        timer = null;
    }

    async function poll() {
        if (!jobId) return;
        try {
            const resp = await fetch(`/api/job/${jobId}/status`);
            if (!resp.ok) {
                // gone from this profile's view (switched profile, aged out):
                // stop, and tell the page so no card hangs "in progress"
                stop();
                showIndicator(null);
                window.dispatchEvent(new CustomEvent('delistd:job-update', {detail: {id: jobId, status: 'error', error_type: 'lost'}}));
                return;
            }
            const job = await resp.json();
            showIndicator(job);
            window.dispatchEvent(new CustomEvent('delistd:job-update', {detail: job}));
            if (job.status !== 'running') stop();
        } catch (err) {
            console.error('job poll error:', err);
        }
    }

    function start(id) {
        jobId = id;
        stop();
        poll();
        timer = setInterval(poll, 1500);
    }

    window.delistdJobPoll = {start, stop, get jobId() { return jobId; }};

    document.addEventListener('DOMContentLoaded', async () => {
        try {
            const data = await (await fetch('/api/job/active')).json();
            if (data.job && data.job.status === 'running') start(data.job.id);
        } catch (err) {
            console.error('active job check failed:', err);
        }
    });
})();
