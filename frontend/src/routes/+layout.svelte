<script lang="ts">
	import '@fontsource-variable/inter';
	import '@fontsource-variable/jetbrains-mono';
	import '../app.css';
	import { onNavigate } from '$app/navigation';
	import { navigating } from '$app/stores';
	import Header from '$lib/components/Header.svelte';
	import Footer from '$lib/components/Footer.svelte';
	import TopProgress from '$lib/components/TopProgress.svelte';

	interface Props {
		children?: import('svelte').Snippet;
	}
	let { children }: Props = $props();

	// Progressive enhancement: view transitions API
	onNavigate((navigation) => {
		if (!document.startViewTransition) return;

		return new Promise((resolve) => {
			try {
				const transition = document.startViewTransition(async () => {
					resolve();
					await navigation.complete;
				});
				transition.finished.catch(() => {});
				transition.ready.catch(() => {});
			} catch (e) {
				resolve();
			}
		});
	});
</script>

<TopProgress />
<Header />

<!-- Dim content while navigating -->
<main class="container" class:navigating={!!$navigating}>
	{@render children?.()}
</main>

<Footer />

<style>
	main {
		transition: opacity 120ms ease;
	}
	main.navigating {
		opacity: 0.55;
		pointer-events: none;
	}
</style>