<script lang="ts">
    import { goto } from '$app/navigation';
    import { onMount } from 'svelte';

    type FetchRequest = {
        steamId: string;
        playerName: string;
        apiKey: string;
        authCode: string;
        knownCode: string;
    };

    const pageSize = 10;
    let request: FetchRequest | null = null;
    let matches: any[] = [];
    let loading = true;
    let loadingMore = false;
    let error = '';
    let cursor = '';
    let hasMore = true;

    async function loadMatches({ append = false } = {}) {
        if (!request) return;
        if (append) {
            loadingMore = true;
        } else {
            loading = true;
        }
        error = '';

        try {
            const params = new URLSearchParams({
                api_key: request.apiKey,
                steam_id: request.steamId,
                auth_code: request.authCode,
                known_code: cursor || request.knownCode,
                limit: String(pageSize)
            });
            const res = await fetch(`http://localhost:8080/api/fetch/sharecodes?${params.toString()}`);
            if (!res.ok) throw new Error(await res.text());
            const payload = await res.json();
            const nextMatches = payload.share_codes ?? [];
            matches = append ? [...matches, ...nextMatches] : nextMatches;

            if (nextMatches.length > 0) {
                cursor = nextMatches[nextMatches.length - 1].share_code;
            }

            hasMore = nextMatches.length === pageSize;
        } catch (e: any) {
            error = e.message;
        } finally {
            loading = false;
            loadingMore = false;
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

        cursor = request.knownCode;
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
                        <div>
                            <dt>Replay URL</dt>
                            <dd>{match.demo_url || 'Unavailable via Steam Web API'}</dd>
                        </div>
                    </dl>

                    {#if match.details}
                        <p class="small muted">{match.details}</p>
                    {/if}

                    <div class="match-actions">
                        <button
                            class="chip primary-chip"
                            disabled
                        >
                            Download unavailable
                        </button>
                    </div>
                </article>
            {/each}
        </div>

        {#if hasMore}
            <div class="pagination-actions">
                <button class="chip" aria-busy={loadingMore} disabled={loadingMore} onclick={() => loadMatches({ append: true })}>Load more</button>
            </div>
        {/if}
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

    .pagination-actions {
        display: flex;
        justify-content: center;
        padding-top: var(--space-4);
    }

    @media (max-width: 639px) {
        .page-head,
        .match-head {
            align-items: flex-start;
            flex-direction: column;
        }
    }
</style>
