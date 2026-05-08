<script lang="ts">
    import { goto } from '$app/navigation';
    import { onMount } from 'svelte';

    type FetchRequest = {
        steamId: string;
        playerName: string;
        apiKey: string;
        authCode: string;
        knownCode: string;
        limit: number;
    };

    let request: FetchRequest | null = null;
    let matches: any[] = [];
    let loading = true;
    let error = '';
    let processingKey = '';
    let processingStatuses: Record<string, string> = {};

    function setProcessingStatus(key: string, status: string) {
        processingStatuses = { ...processingStatuses, [key]: status };
    }

    async function loadMatches() {
        if (!request) return;
        loading = true;
        error = '';

        try {
            const params = new URLSearchParams({
                api_key: request.apiKey,
                steam_id: request.steamId,
                auth_code: request.authCode,
                known_code: request.knownCode,
                limit: String(request.limit)
            });
            const res = await fetch(`http://localhost:8080/api/fetch/sharecodes?${params.toString()}`);
            if (!res.ok) throw new Error(await res.text());
            const payload = await res.json();
            matches = payload.share_codes ?? [];
        } catch (e: any) {
            error = e.message;
        } finally {
            loading = false;
        }
    }

    async function processMatch(match: any) {
        if (!request?.playerName) {
            error = 'Missing player name. Go back and fill the fetch form again.';
            return;
        }

        const key = match.share_code;
        processingKey = key;
        setProcessingStatus(key, 'Requesting download...');

        const timers = [
            setTimeout(() => setProcessingStatus(key, 'Downloading demo...'), 400),
            setTimeout(() => setProcessingStatus(key, 'Decompressing demo...'), 2500),
            setTimeout(() => setProcessingStatus(key, 'Processing demo...'), 4500),
            setTimeout(() => setProcessingStatus(key, 'Saving insights...'), 10000)
        ];

        try {
            const res = await fetch('http://localhost:8080/api/fetch/process', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    share_code: match.share_code,
                    player_name: request.playerName
                })
            });
            if (!res.ok) throw new Error(await res.text());
            const result = await res.json();

            matches = matches.map((candidate) => candidate.share_code === match.share_code ? { ...candidate, processed: true, downloaded: true } : candidate);
            setProcessingStatus(key, `Done. Saved ${result.insights ?? 0} insights.`);
        } catch (e: any) {
            setProcessingStatus(key, `Failed: ${e.message}`);
        } finally {
            timers.forEach(clearTimeout);
            processingKey = '';
        }
    }

    onMount(() => {
        const raw = sessionStorage.getItem('cs-insights:fetch-request');
        if (!raw) {
            goto('/fetch');
            return;
        }

        try {
            request = JSON.parse(raw);
        } catch (e) {
            goto('/fetch');
            return;
        }

        loadMatches();
    });
</script>

<svelte:head>
    <title>Matches - CS Insights</title>
</svelte:head>

<section class="stack-lg">
    <div class="row-between page-head">
        <div>
            <h1 class="display">Matches</h1>
            <p class="muted">Fetched using the Steam match-history token flow.</p>
        </div>
        <button class="chip" onclick={() => goto('/fetch')}>Back to Fetch</button>
    </div>

    {#if loading}
        <div class="card empty-state" aria-busy="true">Loading match share codes...</div>
    {:else if error}
        <div class="card error-card">
            <p>{error}</p>
        </div>
    {:else if matches.length === 0}
        <div class="card empty-state">
            <p>No matches found.</p>
            <span class="small muted">Check your API key, auth code, SteamID64 and known share code.</span>
        </div>
    {:else}
        <div class="match-grid">
            {#each matches as match}
                <article class="card stack-sm match-card">
                    <div class="row-between match-head">
                        <strong>Match Share Code</strong>
                        {#if match.processed}
                            <span class="badge status-success">Processed</span>
                        {:else if match.downloaded}
                            <span class="badge status-warning">Downloaded</span>
                        {:else}
                            <span class="small muted">Not downloaded</span>
                        {/if}
                    </div>

                    <code>{match.share_code}</code>

                    <dl class="match-meta">
                        <div>
                            <dt>Match ID</dt>
                            <dd>{match.match_id}</dd>
                        </div>
                        <div>
                            <dt>Outcome ID</dt>
                            <dd>{match.outcome_id}</dd>
                        </div>
                        <div>
                            <dt>TV Port</dt>
                            <dd>{match.tv_port}</dd>
                        </div>
                        <div>
                            <dt>File</dt>
                            <dd>{match.file_name}</dd>
                        </div>
                    </dl>

                    <div class="match-actions">
                        <button
                            class="chip primary-chip"
                            disabled={processingKey === match.share_code || match.processed}
                            aria-busy={processingKey === match.share_code}
                            onclick={() => processMatch(match)}
                        >
                            {match.processed ? 'Analyzed' : (match.downloaded ? 'Analyze Again' : 'Download & Analyze')}
                        </button>
                        {#if processingStatuses[match.share_code]}
                            <div class="small muted process-status">{processingStatuses[match.share_code]}</div>
                        {/if}
                    </div>
                </article>
            {/each}
        </div>
    {/if}
</section>

<style>
    .match-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(min(100%, 22rem), 1fr));
        gap: var(--space-4);
    }

    .match-card {
        min-width: 0;
    }

    .match-head {
        align-items: center;
    }

    .match-meta {
        display: grid;
        gap: var(--space-2);
        margin: 0;
    }

    .match-meta div {
        display: grid;
        gap: 0.15rem;
    }

    .match-meta dt {
        color: var(--color-text-muted);
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .match-meta dd {
        font-family: var(--font-mono);
        font-size: 0.85rem;
        margin: 0;
        overflow-wrap: anywhere;
    }

    .match-actions {
        align-items: center;
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-3);
    }

    .primary-chip {
        background: var(--color-accent);
        color: var(--color-accent-contrast);
        border-color: var(--color-accent);
        height: 2.25rem;
        width: fit-content;
    }

    .status-success {
        color: var(--color-success);
    }

    .status-warning {
        color: var(--color-warning);
    }

    .error-card {
        border-color: color-mix(in srgb, var(--color-danger) 45%, var(--color-border));
        color: var(--color-danger);
    }

    .process-status {
        min-width: 10rem;
    }

    @media (max-width: 639px) {
        .page-head,
        .match-head {
            align-items: flex-start;
            flex-direction: column;
        }
    }
</style>
