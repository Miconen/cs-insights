<script lang="ts">
	import { page } from '$app/stores';
</script>

<section class="stack">
	<h1 class="display">{$page.status}</h1>
	<div class="error-box">
		{$page.error?.message ?? 'Something went wrong.'}
	</div>
	<a href="/" class="small">← Back home</a>
</section>
