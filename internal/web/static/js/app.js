(() => {
    const toggle = document.querySelector('[data-sidebar-toggle]');
    const backdrop = document.querySelector('[data-sidebar-close]');
    const sidebar = document.querySelector('#sidebar');
    const mobileMenu = window.matchMedia('(max-width: 760px)');

    const closeSidebar = () => {
        document.body.classList.remove('sidebar-open', 'sidebar-collapsed');
        toggle?.setAttribute('aria-expanded', 'false');
    };

    toggle?.addEventListener('click', () => {
        const className = mobileMenu.matches ? 'sidebar-open' : 'sidebar-collapsed';
        const isOpen = document.body.classList.toggle(className);
        toggle.setAttribute('aria-expanded', String(isOpen));
    });
    backdrop?.addEventListener('click', closeSidebar);
    sidebar?.querySelectorAll('a').forEach((link) => link.addEventListener('click', closeSidebar));
    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape') closeSidebar();
    });
    mobileMenu.addEventListener('change', closeSidebar);

    document.querySelectorAll('form[data-confirm]').forEach((form) => {
        form.addEventListener('submit', (event) => {
            if (!window.confirm(form.dataset.confirm)) event.preventDefault();
        });
    });
})();
