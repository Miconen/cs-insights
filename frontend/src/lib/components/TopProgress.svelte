<script lang="ts">
	import { navigating } from '$app/stores';
	import { onMount } from 'svelte';

	let progress = $state(0);
	let visible = $state(false);
	let timer: ReturnType<typeof setTimeout>;

	$effect(() => {
		if ($navigating) {
			visible = true;
			progress = 0.1;
			// Trickle up to ~80% while waiting
			timer = setInterval(() => {
				if (progress < 0.8) {
					progress += (0.8 - progress) * 0.1;
				}
			}, 100);
		} else {
			clearInterval(timer);
			progress = 1;
			// Hide after animation finishes
			setTimeout(() => {
				visible = false;
				progress = 0;
			}, 300);
		}
	});
</script>

<div
	class="top-progress"
	class:visible
	style="transform: scaleX({progress})"
></div>

<style>
	.top-progress {
		position: fixed;
		top: 0;
		left: 0;
		width: 100%;
		height: 2px;
		background: var(--color-accent);
		z-index: 9999;
		transform-origin: 0 50%;
		transform: scaleX(0);
		opacity: 0;
		pointer-events: none;
		transition: transform 150ms ease-out, opacity 150ms ease-out;
	}
	.top-progress.visible {
		opacity: 1;
	}
</style>
