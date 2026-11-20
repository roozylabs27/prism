// Client behaviour for the landing page: theme, mobile nav, code tabs, copy, and Hero stage pointer tilt.

function setupTheme() {
	const buttons = document.querySelectorAll<HTMLButtonElement>('[data-theme-toggle]');
	const sync = () => {
		const dark = document.documentElement.classList.contains('dark');
		buttons.forEach((b) => b.setAttribute('aria-label', dark ? 'Switch to light theme' : 'Switch to dark theme'));
	};
	buttons.forEach((b) =>
		b.addEventListener('click', () => {
			const dark = !document.documentElement.classList.contains('dark');
			document.documentElement.classList.toggle('dark', dark);
			try {
				localStorage.setItem('theme', dark ? 'dark' : 'light');
			} catch {
				/* storage blocked: theme still applies for this page view */
			}
			sync();
		}),
	);
	sync();
}

function setupHeaderScroll() {
	const header = document.getElementById('site-header');
	const toggle = document.getElementById('nav-toggle');
	if (!header) return;

	const update = () => {
		const menuOpen = toggle?.getAttribute('aria-expanded') === 'true';
		const scrolled = window.scrollY > 12 || menuOpen;
		header.dataset.scrolled = String(scrolled);
		header.classList.toggle('border-transparent', !scrolled);
		header.classList.toggle('bg-transparent', !scrolled);
		header.classList.toggle('border-rule', scrolled);
		header.classList.toggle('bg-paper/85', scrolled);
		header.classList.toggle('backdrop-blur-md', scrolled);
	};

	update();
	window.addEventListener('scroll', update, { passive: true });
	return update;
}

function setupMobileNav(onMenuChange?: () => void) {
	const toggle = document.getElementById('nav-toggle');
	const panel = document.getElementById('mobile-nav');
	if (!toggle || !panel) return;
	const iconOpen = toggle.querySelector('[data-icon-open]');
	const iconClose = toggle.querySelector('[data-icon-close]');

	const setOpen = (open: boolean, returnFocus = false) => {
		panel.classList.toggle('hidden', !open);
		iconOpen?.classList.toggle('hidden', open);
		iconClose?.classList.toggle('hidden', !open);
		toggle.setAttribute('aria-expanded', String(open));
		toggle.setAttribute('aria-label', open ? 'Close menu' : 'Open menu');
		onMenuChange?.();
		if (open) panel.querySelector<HTMLElement>('a')?.focus();
		else if (returnFocus) toggle.focus();
	};

	toggle.addEventListener('click', () => setOpen(toggle.getAttribute('aria-expanded') !== 'true'));
	panel.querySelectorAll('a').forEach((a) => a.addEventListener('click', () => setOpen(false)));
	document.addEventListener('keydown', (e) => {
		if (e.key === 'Escape' && toggle.getAttribute('aria-expanded') === 'true') setOpen(false, true);
	});
	window.matchMedia('(min-width: 1024px)').addEventListener('change', (e) => {
		if (e.matches) setOpen(false);
	});
}

function setupTabs() {
	document.querySelectorAll<HTMLElement>('[data-tabs]').forEach((root) => {
		const tabs = Array.from(root.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
		const panels = Array.from(root.querySelectorAll<HTMLElement>('[role="tabpanel"]'));

		const select = (index: number, focus = false) => {
			tabs.forEach((tab, i) => {
				const active = i === index;
				tab.setAttribute('aria-selected', String(active));
				tab.tabIndex = active ? 0 : -1;
				panels[i].hidden = !active;
			});
			if (focus) tabs[index].focus();
		};

		tabs.forEach((tab, i) => {
			tab.addEventListener('click', () => select(i));
			tab.addEventListener('keydown', (e) => {
				const keys: Record<string, number> = {
					ArrowRight: (i + 1) % tabs.length,
					ArrowLeft: (i - 1 + tabs.length) % tabs.length,
					Home: 0,
					End: tabs.length - 1,
				};
				if (e.key in keys) {
					e.preventDefault();
					select(keys[e.key], true);
				}
			});
		});

		const copy = root.querySelector<HTMLButtonElement>('[data-copy]');
		const label = root.querySelector('[data-copy-label]');
		const status = root.querySelector('[data-copy-status]');
		copy?.addEventListener('click', async () => {
			const code = panels.find((p) => !p.hidden)?.querySelector('code')?.textContent ?? '';
			try {
				await navigator.clipboard.writeText(code);
				if (label) label.textContent = 'Copied';
				if (status) status.textContent = 'Code copied to clipboard';
			} catch {
				if (label) label.textContent = 'Copy failed';
				if (status) status.textContent = 'Copy failed. Select the code and copy it manually.';
			}
			window.setTimeout(() => {
				if (label) label.textContent = 'Copy';
			}, 2000);
		});
	});
}

function setupPrismStageParallax() {
	const stage = document.querySelector<HTMLElement>('[data-prism-stage]');
	const group = stage?.querySelector<HTMLElement>('[data-prism-parallax]');
	if (!stage || !group || !window.matchMedia) return;

	const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
	const coarsePointer = window.matchMedia('(pointer: coarse)');

	stage.addEventListener('pointermove', (e) => {
		if (reducedMotion.matches || coarsePointer.matches) return;
		const rect = stage.getBoundingClientRect();
		if (!rect.width || !rect.height) return;
		const nx = Math.max(-1, Math.min(1, ((e.clientX - rect.left) / rect.width - 0.5) * 2));
		const ny = Math.max(-1, Math.min(1, ((e.clientY - rect.top) / rect.height - 0.5) * 2));
		const rotY = (nx * 4.5).toFixed(2);
		const rotX = (-ny * 4.5).toFixed(2);
		const tx = (nx * 6).toFixed(1);
		const ty = (ny * 5).toFixed(1);
		group.style.transform = `perspective(1000px) rotateX(${rotX}deg) rotateY(${rotY}deg) translate3d(${tx}px, ${ty}px, 0px)`;
	});

	stage.addEventListener('pointerleave', () => {
		group.style.transform = 'none';
	});
}

setupTheme();
const syncHeader = setupHeaderScroll();
setupMobileNav(syncHeader);
setupTabs();
setupPrismStageParallax();
