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
	sidebar?.querySelectorAll('a').forEach((link) => {
		link.addEventListener('click', closeSidebar);
	});

	document.addEventListener('keydown', (event) => {
		if (event.key === 'Escape') closeSidebar();
	});

	mobileMenu.addEventListener('change', closeSidebar);

	document.querySelectorAll('form[data-confirm]').forEach((form) => {
		form.addEventListener('submit', (event) => {
			if (!window.confirm(form.dataset.confirm)) event.preventDefault();
		});
	});

	let pinDialog;

	const addPinDialogStyles = () => {
		if (document.querySelector('#pin-dialog-styles')) return;

		const style = document.createElement('style');
		style.id = 'pin-dialog-styles';
		style.textContent = `
			.pin-access-dialog {
				width: min(390px, calc(100% - 32px));
				max-width: 390px;
				padding: 0;
				color: var(--text, #edf3ff);
				background: var(--panel, #1a2845);
				border: 1px solid var(--line-strong, rgba(164,184,218,.2));
				border-radius: 12px;
				box-shadow: 0 24px 80px rgba(0,0,0,.42);
			}
			.pin-access-dialog::backdrop {
				background: rgba(4,9,18,.62);
				backdrop-filter: blur(2px);
			}
			.pin-access-content {
				padding: 27px 28px 28px;
				text-align: center;
			}
			.pin-access-prompt {
				margin: 0 0 8px;
				color: var(--muted, #94a4c1);
				font-size: 13px;
			}
			.pin-access-title {
				margin: 0 0 22px;
				color: var(--teal, #29c7b2);
				font-size: 18px;
				font-weight: 800;
				letter-spacing: 1.6px;
			}
			.pin-access-error {
				margin: 0 0 14px;
				padding: 9px 11px;
				color: #ff9aa8;
				font-size: 12px;
				line-height: 1.5;
				text-align: left;
				background: rgba(255,113,133,.08);
				border: 1px solid rgba(255,113,133,.22);
				border-radius: 7px;
			}
			.pin-access-input {
				display: block;
				width: 100%;
				height: 48px;
				margin: 0 0 17px;
				padding: 0 14px;
				color: var(--text, #edf3ff);
				font-size: 22px;
				font-weight: 700;
				letter-spacing: 8px;
				text-align: center;
				background: var(--bg-deep, #0d1628);
				border: 1px solid var(--line-strong, rgba(164,184,218,.2));
				border-radius: 7px;
				outline: none;
			}
			.pin-access-input:focus {
				border-color: var(--teal, #29c7b2);
				box-shadow: 0 0 0 3px rgba(41,199,178,.12);
			}
			.pin-access-submit {
				width: 100%;
				min-height: 42px;
				color: #0d2430;
				font-size: 11px;
				font-weight: 800;
				letter-spacing: .65px;
				background: linear-gradient(135deg, #55d9c7, #27b8aa);
				border: 0;
				border-radius: 7px;
				cursor: pointer;
			}
			.pin-access-submit:hover {
				background: #67e2d2;
			}
			.pin-access-submit:disabled {
				cursor: wait;
				opacity: .7;
			}
			@media (max-width: 480px) {
				.pin-access-content {
					padding: 24px 21px;
				}
			}
		`;
		document.head.appendChild(style);
	};

	const createPinDialog = () => {
		addPinDialogStyles();

		const dialog = document.createElement('dialog');
		dialog.className = 'pin-access-dialog';
		dialog.setAttribute('aria-labelledby', 'pin-access-title');
		dialog.innerHTML = `
			<div class="pin-access-content">
				<p class="pin-access-prompt">для доступа введите</p>
				<h2 class="pin-access-title" id="pin-access-title">TIME PIN CODE</h2>
				<p class="pin-access-error" role="alert" hidden></p>
				<form action="/settings" method="post">
					<input
						class="pin-access-input"
						name="pin"
						type="password"
						inputmode="numeric"
						pattern="[0-9]{4}"
						minlength="4"
						maxlength="4"
						autocomplete="off"
						aria-label="Четырёхзначный PIN-код"
						required
					>
					<button class="pin-access-submit" type="submit">ПОЛУЧИТЬ ДОСТУП</button>
				</form>
			</div>
		`;

		const form = dialog.querySelector('form');
		const input = dialog.querySelector('input');
		const error = dialog.querySelector('[role="alert"]');
		const submit = dialog.querySelector('button');

		form.addEventListener('submit', async (event) => {
			event.preventDefault();
			error.hidden = true;
			submit.disabled = true;

			try {
				const response = await fetch('/settings', {
					method: 'POST',
					body: new FormData(form),
					headers: {
						'X-Requested-With': 'XMLHttpRequest',
					},
				});

				if (response.ok) {
					const page = await response.text();
					document.open();
					document.write(page);
					document.close();
					return;
				}

				let message = 'Не удалось проверить код. Попробуйте ещё раз.';
				try {
					const result = await response.json();
					if (result.error) message = result.error;
				} catch (_) {
					// Оставляем сообщение по умолчанию, если сервер вернул не JSON.
				}

				error.textContent = message;
				error.hidden = false;
				input.value = '';
				input.focus();
			} catch (_) {
				error.textContent = 'Не удалось связаться с сервером. Попробуйте ещё раз.';
				error.hidden = false;
			} finally {
				submit.disabled = false;
			}
		});

		dialog.addEventListener('close', () => {
			dialog.remove();
			pinDialog = null;
		});

		document.body.appendChild(dialog);
		dialog.showModal();
		input.focus();

		pinDialog = dialog;
	};

	const openPinDialog = (event) => {
		if (event) event.preventDefault();
		if (!pinDialog) createPinDialog();
	};

	document.addEventListener('click', (event) => {
		const link = event.target.closest('a[href]');
		if (!link) return;

		if (
			event.defaultPrevented ||
			event.button !== 0 ||
			event.metaKey ||
			event.ctrlKey ||
			event.shiftKey ||
			event.altKey ||
			link.target === '_blank'
		) {
			return;
		}

		const url = new URL(link.href, window.location.href);
		if (url.origin !== window.location.origin || url.pathname !== '/settings') return;
		if (url.searchParams.get('section') === 'logs') return;

		openPinDialog(event);
	});

	document.addEventListener('submit', (event) => {
		const form = event.target;
		if (!(form instanceof HTMLFormElement)) return;

		const url = new URL(form.action, window.location.href);
		if (url.origin !== window.location.origin || url.pathname !== '/settings') return;
		if (form.classList.contains('pin-access-form')) return;

		event.preventDefault();
		openPinDialog();
	});
})();
