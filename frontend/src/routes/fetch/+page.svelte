<script lang="ts">
    import { goto } from '$app/navigation';

    let steamId = '';
    let cookie = '';
    let playerName = '';
    
    let loadingMatches = false;
    let matches: any[] = [];
    let error = '';
    let processingLinks: Record<string, boolean> = {};
    let processingStatuses: Record<string, string> = {};

    const api = (path: string) => path;

    function setProcessingStatus(link: string, status: string) {
        processingStatuses = { ...processingStatuses, [link]: status };
    }

    function actionLabel(match: any) {
        if (processingStatuses[match.link]) {
            return processingStatuses[match.link];
        }
        return match.processed ? 'Analyzed' : (match.downloaded ? 'Analyze Again' : 'Download & Analyze');
    }

    function isProcessing(link: string) {
        return !!processingLinks[link];
    }

    async function fetchMatches(e: Event) {
        e.preventDefault();
        error = '';
        matches = [];
        processingStatuses = {};
        loadingMatches = true;
        try {
            const res = await fetch(api('/api/fetch/list'), {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ steam_id: steamId, cookie })
            });
            if (!res.ok) throw new Error(await res.text());
            matches = await res.json();
            cookie = '';
        } catch (e: any) {
            error = e.message;
            matches = [];
        } finally {
            loadingMatches = false;
        }
    }

    async function processMatch(match: any) {
        if (!playerName) {
            error = 'Enter your exact in-game name before downloading a match.';
            return;
        }

        processingLinks = { ...processingLinks, [match.link]: true };
        setProcessingStatus(match.link, match.downloaded ? 'Found local demo. Processing...' : 'Checking local demo...');

        const timers = [
            setTimeout(() => setProcessingStatus(match.link, match.downloaded ? 'Processing demo...' : 'Downloading if needed...'), 900),
            setTimeout(() => setProcessingStatus(match.link, 'Analyzing rounds and gunfights...'), 4500),
            setTimeout(() => setProcessingStatus(match.link, 'Saving insights...'), 10000)
        ];

        try {
            const res = await fetch(api('/api/fetch/process'), {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ link: match.link, player_name: playerName })
            });
            if (!res.ok) throw new Error(await res.text());
            const result = await res.json();

            matches = matches.map((candidate) => candidate.link === match.link ? { ...candidate, processed: true, downloaded: true } : candidate);
            const source = result.downloaded ? 'Downloaded and processed' : 'Reused local demo and processed';
            setProcessingStatus(match.link, `${source}. Saved ${result.insights ?? 0} insights.`);
        } catch (e: any) {
            setProcessingStatus(match.link, `Failed: ${e.message}`);
        } finally {
            timers.forEach(clearTimeout);
            const { [match.link]: _done, ...rest } = processingLinks;
            processingLinks = rest;
        }
    }
</script>

<svelte:head>
    <title>Fetch Demos - CS Insights</title>
</svelte:head>

<section class="stack-lg">
    <div class="row-between page-head">
        <div>
            <h1 class="display">Fetch Recent Matches</h1>
            <p class="muted">List recent Premier matches and download demos from your Steam match history.</p>
        </div>
    </div>

    <div class="fetch-layout">
        <article class="card stack form-panel legacy-panel">
            <div class="form-header">
                <span class="eyebrow">Supported</span>
                <h2>Steam GCPD Replay Links</h2>
                <p class="muted small">
                    This fetches the direct replay download links from your CS match history page. It requires <code>steamLoginSecure</code>, so treat that cookie like a password.
                </p>
            </div>

            <form class="stack" onsubmit={fetchMatches}>
                <label class="field stack-sm" for="steamId">
                    <span>Steam ID or Custom URL</span>
                    <input type="text" id="steamId" bind:value={steamId} placeholder="e.g. Miconen" required>
                </label>

                <label class="field stack-sm" for="playerName">
                    <span>Exact in-game name</span>
                    <input type="text" id="playerName" bind:value={playerName} placeholder="e.g. s1mple" required>
                </label>

                <label class="field stack-sm" for="cookie">
                    <span class="field-label">steamLoginSecure cookie</span>
                    <input type="password" id="cookie" bind:value={cookie} placeholder="Paste cookie value here" required>
                    <span class="small muted">Required to access your private match history.</span>
                </label>

                <div class="legacy-actions">
                    <button class="chip primary-chip" type="submit" aria-busy={loadingMatches} disabled={loadingMatches}>Load Match History</button>
                </div>
            </form>
        </article>
    </div>

        {#if error}
            <div class="card error-card">
                <p>{error}</p>
            </div>
        {/if}

        {#if matches && matches.length > 0}
            <div class="stack-sm">
                <div class="section-heading">Available Matches ({matches.length})</div>
                <div class="table-wrapper">
                    <table class="table table-collapse-mobile">
                        <thead>
                            <tr>
                                <th scope="col">File Name</th>
                                <th scope="col">Status</th>
                                <th scope="col">Action</th>
                            </tr>
                        </thead>
                        <tbody>
                            {#each matches as match}
                                <tr>
                                    <td data-label="File Name"><code>{match.file_name}</code></td>
                                    <td data-label="Status">
                                        {#if match.processed}
                                            <span class="badge status-success">Processed</span>
                                        {:else if match.downloaded}
                                            <span class="badge status-warning">Downloaded</span>
                                        {:else}
                                            <span class="muted">Not Downloaded</span>
                                        {/if}
                                    </td>
                                    <td class="action-cell" data-label="Action">
                                        <button
                                            class="chip"
                                            disabled={isProcessing(match.link) || match.processed}
                                            aria-busy={isProcessing(match.link)}
                                            onclick={() => processMatch(match)}
                                        >
                                            {actionLabel(match)}
                                        </button>
                                        {#if match.processed}
                                            <button class="chip chip-muted" onclick={() => goto(`/?player=${encodeURIComponent(playerName)}`)}>View dashboard</button>
                                        {/if}
                                    </td>
                                </tr>
                            {/each}
                        </tbody>
                    </table>
                </div>
            </div>
        {/if}
</section>

<style>
    input {
        max-width: none;
    }

    .fetch-layout {
        max-width: 48rem;
    }

    .form-panel {
        padding: var(--space-5);
    }

    .legacy-panel {
        background: linear-gradient(135deg, var(--color-surface), color-mix(in srgb, var(--color-accent) 7%, var(--color-surface-2)));
    }

    .form-header h2 {
        margin-bottom: var(--space-1);
    }

    .eyebrow {
        display: inline-flex;
        align-items: center;
        width: fit-content;
        border: 1px solid color-mix(in srgb, var(--color-accent) 45%, var(--color-border));
        border-radius: 999px;
        color: var(--color-accent);
        font-size: 0.75rem;
        font-weight: 650;
        letter-spacing: 0.04em;
        line-height: 1;
        padding: 0.25rem 0.5rem;
        text-transform: uppercase;
    }

    .field {
        display: grid;
        gap: var(--space-2);
        margin-bottom: 0;
    }

    .field input {
        width: 100%;
    }

    td[data-label='File Name'] code {
        overflow-wrap: anywhere;
        white-space: normal;
    }

    .action-cell {
        display: flex;
        gap: var(--space-2);
        flex-wrap: wrap;
        align-items: center;
    }

    .field-label {
        align-items: baseline;
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
        font-weight: 600;
    }

    .legacy-actions {
        display: flex;
        align-items: end;
        gap: var(--space-3);
        flex-wrap: wrap;
    }

    .primary-chip {
        background: var(--color-accent);
        color: var(--color-accent-contrast);
        border-color: var(--color-accent);
        height: 2.25rem;
        width: fit-content;
    }

    .error-card {
        border-color: color-mix(in srgb, var(--color-danger) 45%, var(--color-border));
        color: var(--color-danger);
    }

    .status-success {
        color: var(--color-success);
    }

    .status-warning {
        color: var(--color-warning);
    }

    @media (max-width: 639px) {
        .page-head {
            align-items: flex-start;
            flex-direction: column;
        }

        .form-panel {
            padding: var(--space-4);
        }

        .legacy-actions {
            align-items: flex-start;
            flex-direction: column;
        }

        .primary-chip {
            width: 100%;
            justify-content: center;
        }

    }
</style>
