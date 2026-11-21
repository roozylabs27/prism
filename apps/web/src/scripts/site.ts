// Client behaviour for the landing page: theme, mobile nav, section scroll-spy,
// Hero stage route inspector & pointer tilt, code tabs, and command/code copy.

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
		header.dataset.menuOpen = String(menuOpen);
		header.classList.toggle('border-transparent', !scrolled);
		header.classList.toggle('bg-transparent', !scrolled);
		header.classList.toggle('border-rule', scrolled);
		header.classList.toggle('bg-paper/95', menuOpen);
		header.classList.toggle('bg-paper/85', scrolled && !menuOpen);
		header.classList.toggle('backdrop-blur-md', scrolled);
	};

	update();
	window.addEventListener('scroll', update, { passive: true });
	return update;
}

function setupMobileNav(onMenuChange?: () => void) {
	const toggle = document.getElementById('nav-toggle');
	const panel = document.getElementById('mobile-nav');
	const backdrop = document.getElementById('mobile-nav-backdrop');
	if (!toggle || !panel) return;
	const iconOpen = toggle.querySelector<HTMLElement>('[data-icon-open]');
	const iconClose = toggle.querySelector<HTMLElement>('[data-icon-close]');
	const label = toggle.querySelector('[data-menu-label]');
	const list = panel.querySelector<HTMLElement>('[data-mobile-nav-list]');

	const setOpen = (open: boolean, returnFocus = false) => {
		panel.dataset.open = String(open);
		panel.setAttribute('aria-hidden', String(!open));
		if (open) {
			panel.removeAttribute('inert');
		} else {
			panel.setAttribute('inert', '');
		}

		if (backdrop) {
			backdrop.dataset.open = String(open);
		}

		if (list) {
			list.classList.toggle('-translate-y-2', !open);
			list.classList.toggle('opacity-0', !open);
			list.classList.toggle('translate-y-0', open);
			list.classList.toggle('opacity-100', open);
		}

		if (iconOpen) {
			iconOpen.classList.toggle('opacity-100', !open);
			iconOpen.classList.toggle('rotate-0', !open);
			iconOpen.classList.toggle('scale-100', !open);
			iconOpen.classList.toggle('opacity-0', open);
			iconOpen.classList.toggle('rotate-90', open);
			iconOpen.classList.toggle('scale-75', open);
		}
		if (iconClose) {
			iconClose.classList.toggle('opacity-0', !open);
			iconClose.classList.toggle('-rotate-90', !open);
			iconClose.classList.toggle('scale-75', !open);
			iconClose.classList.toggle('opacity-100', open);
			iconClose.classList.toggle('rotate-0', open);
			iconClose.classList.toggle('scale-100', open);
		}

		if (label) label.textContent = open ? 'Close' : 'Menu';
		toggle.setAttribute('aria-expanded', String(open));
		toggle.setAttribute('aria-label', open ? 'Close menu' : 'Open menu');
		onMenuChange?.();
		if (open) panel.querySelector<HTMLElement>('a')?.focus();
		else if (returnFocus) toggle.focus();
	};

	toggle.addEventListener('click', () => setOpen(toggle.getAttribute('aria-expanded') !== 'true'));
	backdrop?.addEventListener('click', () => setOpen(false));
	panel.querySelectorAll('a').forEach((a) => a.addEventListener('click', () => setOpen(false)));
	document.addEventListener('keydown', (e) => {
		if (e.key === 'Escape' && toggle.getAttribute('aria-expanded') === 'true') setOpen(false, true);
	});
	window.matchMedia('(min-width: 1024px)').addEventListener('change', (e) => {
		if (e.matches) setOpen(false);
	});
}

function setupSectionObserver() {
	const links = Array.from(document.querySelectorAll<HTMLAnchorElement>('[data-nav-link]'));
	if (!links.length || !('IntersectionObserver' in window)) return;

	const linkById = new Map<string, HTMLAnchorElement>();
	links.forEach((link) => {
		const href = link.getAttribute('href');
		if (href?.startsWith('#')) {
			linkById.set(href.slice(1), link);
		}
	});

	const sections = Array.from(document.querySelectorAll<HTMLElement>('main section[id]')).filter((sec) =>
		linkById.has(sec.id),
	);
	if (!sections.length) return;

	const setActive = (activeId: string | null) => {
		links.forEach((link) => {
			const id = link.getAttribute('href')?.slice(1) ?? '';
			if (activeId && id === activeId) {
				link.setAttribute('aria-current', 'location');
			} else {
				link.removeAttribute('aria-current');
			}
		});
	};

	const visible = new Set<string>();
	const observer = new IntersectionObserver(
		(entries) => {
			entries.forEach((entry) => {
				if (entry.isIntersecting) visible.add(entry.target.id);
				else visible.delete(entry.target.id);
			});
			const firstVisible = sections.find((sec) => visible.has(sec.id));
			if (firstVisible) {
				setActive(firstVisible.id);
			} else if (window.scrollY < 160) {
				setActive(null);
			}
		},
		{ rootMargin: '-25% 0px -55% 0px', threshold: 0 },
	);

	sections.forEach((sec) => observer.observe(sec));
}

async function copyTextWithFeedback(
	text: string,
	label: Element | null,
	status: Element | null,
) {
	try {
		await navigator.clipboard.writeText(text);
		if (label) label.textContent = 'Copied';
		if (status) status.textContent = 'Code copied to clipboard';
	} catch {
		if (label) label.textContent = 'Copy failed';
		if (status) status.textContent = 'Copy failed. Select the code and copy it manually.';
	}
	window.setTimeout(() => {
		if (label) label.textContent = 'Copy';
	}, 2000);
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
		copy?.addEventListener('click', () => {
			const code = panels.find((p) => !p.hidden)?.querySelector('code')?.textContent ?? '';
			void copyTextWithFeedback(code, label, status);
		});
	});
}

function setupCommandCopyBlocks() {
	document.querySelectorAll<HTMLElement>('[data-copy-block]').forEach((block) => {
		const button = block.querySelector<HTMLButtonElement>('[data-copy-command]');
		const label = block.querySelector('[data-copy-label]');
		const status = block.querySelector('[data-copy-status]');
		const code = block.querySelector('code');
		if (!button || !code) return;

		button.addEventListener('click', () => {
			void copyTextWithFeedback(code.textContent ?? '', label, status);
		});
	});
}

function setupPrismStageInspector() {
	const stage = document.querySelector<HTMLElement>('[data-prism-stage]');
	const inspector = stage?.querySelector<HTMLElement>('[data-stage-inspector]');
	if (!stage || !inspector) return;

	const tabs = Array.from(inspector.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
	const panels = Array.from(inspector.querySelectorAll<HTMLElement>('[role="tabpanel"]'));

	const select = (index: number, focus = false) => {
		tabs.forEach((tab, i) => {
			const active = i === index;
			tab.setAttribute('aria-selected', String(active));
			tab.tabIndex = active ? 0 : -1;
			if (panels[i]) panels[i].hidden = !active;
			if (active) {
				const mode = tab.dataset.stageMode;
				if (mode) stage.dataset.activeMode = mode;
			}
		});
		if (focus) tabs[index]?.focus();
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
setupSectionObserver();
setupTabs();
setupCommandCopyBlocks();
setupPrismStageInspector();
setupPrismStageParallax();
