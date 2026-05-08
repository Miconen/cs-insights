<script lang="ts">
	import { page } from '$app/stores';
	import ThemeSwitcher from './ThemeSwitcher.svelte';
	import { onMount, onDestroy } from 'svelte';

	const links = [
		{ href: '/', label: 'Dashboard' },
		{ href: '/fetch', label: 'Fetch Demos' }
	];

	function isActive(href: string): boolean {
		const current = $page.url.pathname;
		if (href === '/') {
			return current === '/';
		}
		return current.startsWith(href);
	}

	let mobileMenuOpen = $state(false);

	function toggleMenu() {
		mobileMenuOpen = !mobileMenuOpen;
	}

	function closeMenu() {
		mobileMenuOpen = false;
	}

	function handleOutsideClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (!target.closest('.site-header')) {
			closeMenu();
		}
	}

	onMount(() => {
		document.addEventListener('click', handleOutsideClick);
	});

	onDestroy(() => {
		if (typeof document !== 'undefined') {
			document.removeEventListener('click', handleOutsideClick);
		}
	});

	$effect(() => {
		if ($page.url) {
			closeMenu();
		}
	});
</script>

<header class="site-header">
	<div class="site-header-inner">
		<button class="burger-btn" aria-label="Toggle menu" onclick={toggleMenu}>
			<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
				{#if mobileMenuOpen}
					<path d="M18 6 6 18"/><path d="m6 6 12 12"/>
				{:else}
					<line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="18" y2="18"/>
				{/if}
			</svg>
		</button>

		<a class="brand" href="/">CS Insights</a>
		
		<nav class="desktop-nav">
			{#each links as link (link.href)}
				<a
					href={link.href}
					class:active={isActive(link.href)}
				>
					{link.label}
				</a>
			{/each}
		</nav>
		
		<ThemeSwitcher />
	</div>

	{#if mobileMenuOpen}
		<div class="mobile-nav">
			{#each links as link (link.href)}
				<a
					href={link.href}
					class="mobile-nav-link"
					class:active={isActive(link.href)}
					onclick={closeMenu}
				>
					{link.label}
				</a>
			{/each}
		</div>
	{/if}
</header>

<style>
	.burger-btn {
		display: none;
		background: transparent;
		border: none;
		color: var(--color-text);
		padding: var(--space-2) 0;
		margin-right: var(--space-2);
		cursor: pointer;
	}

	.mobile-nav {
		display: none;
		position: absolute;
		top: 100%;
		left: 0;
		right: 0;
		background: var(--color-surface);
		border-bottom: 1px solid var(--color-border);
		box-shadow: var(--shadow-2);
		padding: var(--space-2) 0;
		flex-direction: column;
	}

	.mobile-nav-link {
		padding: var(--space-3) var(--space-4);
		font-weight: 500;
		color: var(--color-text-muted);
		border-left: 3px solid transparent;
	}

	.mobile-nav-link:hover {
		background: var(--color-surface-2);
		color: var(--color-text);
		text-decoration: none;
	}

	.mobile-nav-link.active {
		color: var(--color-text);
		border-left-color: var(--color-accent);
		background: var(--color-surface-2);
	}

	@media (max-width: 767px) {
		.burger-btn {
			display: flex;
			align-items: center;
			justify-content: center;
		}
		.mobile-nav {
			display: flex;
		}
	}
</style>
