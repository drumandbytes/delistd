// Theme: "auto" follows the OS, "light" and "dark" are explicit. Only an
// explicit choice is stored; auto is the absence of one. Loaded in <head>, so
// the theme is applied before the first paint.
(function () {
    'use strict';
    const KEY = 'delistd-theme';
    const osDark = window.matchMedia('(prefers-color-scheme: dark)');

    function mode() {
        let stored = null;
        try { stored = localStorage.getItem(KEY); } catch (e) { /* storage blocked */ }
        return stored === 'light' || stored === 'dark' ? stored : 'auto';
    }

    function apply() {
        const current = mode();
        const dark = current === 'dark' || (current === 'auto' && osDark.matches);
        document.documentElement.classList.toggle('light', !dark);
        document.querySelectorAll('[data-theme-set]').forEach((b) => {
            b.setAttribute('aria-checked', String(b.dataset.themeSet === current));
        });
    }

    window.setThemeMode = function (next) {
        try {
            if (next === 'auto') localStorage.removeItem(KEY);
            else localStorage.setItem(KEY, next);
        } catch (e) { /* storage blocked */ }
        apply();
    };

    osDark.addEventListener('change', apply);
    apply();
    document.addEventListener('DOMContentLoaded', () => {
        document.querySelectorAll('[data-theme-set]').forEach((b) =>
            b.addEventListener('click', () => window.setThemeMode(b.dataset.themeSet)));
        apply(); // mark the active choice now that the switch exists
    });
})();
