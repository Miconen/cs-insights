<script lang="ts">
	import { onMount } from 'svelte';

	type Theme = 'system' | 'dark' | 'light';

	let theme: Theme = $state('system');
	let mediaQuery: MediaQueryList;

	onMount(() => {
		mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
		mediaQuery.addEventListener('change', onMediaChange);

		const stored = (localStorage.getItem('theme') as Theme | null) ?? 'system';
		// If someone had 'parchment' saved from before, default to dark
		if (stored !== 'system' && stored !== 'dark' && stored !== 'light') {
			theme = 'dark';
			apply('dark');
		} else {
			theme = stored;
			apply(stored);
		}

		return () => mediaQuery.removeEventListener('change', onMediaChange);
	});

	function onMediaChange() {
		if (theme === 'system') apply('system');
	}

	function apply(t: Theme) {
		if (t === 'system') {
			const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
			document.documentElement.dataset.theme = isDark ? 'dark' : 'light';
		} else {
			document.documentElement.dataset.theme = t;
		}
	}

	function setTheme(t: Theme) {
		theme = t;
		localStorage.setItem('theme', t);
		apply(t);
	}
</script>

<div class="segment-toggle" aria-label="Theme">
	<button
		type="button"
		class="segment-btn {theme === 'light' ? 'active' : ''}"
		onclick={() => setTheme('light')}
		title="Light"
	>
		<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/></svg>
	</button>
	<button
		type="button"
		class="segment-btn {theme === 'system' ? 'active' : ''}"
		onclick={() => setTheme('system')}
		title="System"
	>
		<span style="font-size: 10px; font-weight: 600;">SYS</span>
	</button>
	<button
		type="button"
		class="segment-btn {theme === 'dark' ? 'active' : ''}"
		onclick={() => setTheme('dark')}
		title="Dark"
	>
		<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/></svg>
	</button>
</div>
