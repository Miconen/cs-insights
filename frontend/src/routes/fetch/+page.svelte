<script lang="ts">
    let steamId = '';
    let cookie = '';
    let playerName = '';
    let apiKey = '';
    let authCode = '';
    let knownCode = '';
    let limit = 10;
    
    let loadingMatches = false;
    let loadingShareCodes = false;
    let matches: any[] = [];
    let shareCodes: any[] = [];
    let error = '';

    async function fetchMatches(e: Event) {
        e.preventDefault();
        loadingMatches = true;
        error = '';
        try {
            const res = await fetch(`http://localhost:8080/api/fetch/list?steam_id=${encodeURIComponent(steamId)}&cookie=${encodeURIComponent(cookie)}`);
            if (!res.ok) throw new Error(await res.text());
            matches = await res.json();
        } catch (e: any) {
            error = e.message;
        } finally {
            loadingMatches = false;
        }
    }

    async function fetchShareCodes(e: Event) {
        e.preventDefault();
        loadingShareCodes = true;
        error = '';
        try {
            const params = new URLSearchParams({
                api_key: apiKey,
                steam_id: steamId,
                auth_code: authCode,
                known_code: knownCode,
                limit: String(limit)
            });
            const res = await fetch(`http://localhost:8080/api/fetch/sharecodes?${params.toString()}`);
            if (!res.ok) throw new Error(await res.text());
            const payload = await res.json();
            shareCodes = payload.share_codes ?? [];
        } catch (e: any) {
            error = e.message;
        } finally {
            loadingShareCodes = false;
        }
    }

    let processingLink = '';
    let processingStatuses: Record<string, string> = {};

    function setProcessingStatus(link: string, status: string) {
        processingStatuses = { ...processingStatuses, [link]: status };
    }

    async function processMatch(link: string) {
        if (!playerName) {
            alert('Please enter your Player Name before processing.');
            return;
        }
        processingLink = link;
        setProcessingStatus(link, 'Requesting download...');

        const timers = [
            setTimeout(() => setProcessingStatus(link, 'Downloading demo...'), 400),
            setTimeout(() => setProcessingStatus(link, 'Decompressing demo...'), 2500),
            setTimeout(() => setProcessingStatus(link, 'Processing demo...'), 4500),
            setTimeout(() => setProcessingStatus(link, 'Saving insights...'), 10000)
        ];

        try {
            const res = await fetch('http://localhost:8080/api/fetch/process', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ link, player_name: playerName })
            });
            if (!res.ok) throw new Error(await res.text());
            const result = await res.json();
            
            // Mark as processed in UI
            matches = matches.map(m => m.link === link ? { ...m, processed: true, downloaded: true } : m);
            setProcessingStatus(link, `Done. Saved ${result.insights ?? 0} insights.`);
        } catch (e: any) {
            setProcessingStatus(link, `Failed: ${e.message}`);
        } finally {
            timers.forEach(clearTimeout);
            processingLink = '';
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
            <p class="muted">Scrape your GCPD page to analyze Premier games.</p>
        </div>
    </div>

    <div class="fetch-layout">
        <article class="card stack form-panel recommended-panel">
            <div class="form-header">
                <span class="eyebrow">Recommended</span>
                <h2>Steam Match History Token</h2>
                <p class="muted small">
                    Uses Valve's official match-history API. Safer than using a browser-session cookie. Direct demo download from share codes is still pending.
                </p>
            </div>

            <form class="stack" onsubmit={fetchShareCodes}>
                <label class="field stack-sm" for="steamApiKey">
                    <span>Steam Web API key</span>
                    <input type="password" id="steamApiKey" bind:value={apiKey} placeholder="Steam Web API key">
                    <span class="small muted"><a href="https://steamcommunity.com/dev/apikey" target="_blank" rel="noreferrer">Get a Steam Web API key</a></span>
                </label>

                <label class="field stack-sm" for="steamIdToken">
                    <span>SteamID64</span>
                    <input type="text" id="steamIdToken" bind:value={steamId} placeholder="7656119...">
                    <span class="small muted"><a href="https://steamid.io/" target="_blank" rel="noreferrer">Find your SteamID64</a></span>
                </label>

                <label class="field stack-sm" for="authCode">
                    <span>Match history auth code</span>
                    <input type="password" id="authCode" bind:value={authCode} placeholder="steamidkey / auth code">
                    <span class="small muted"><a href="https://help.steampowered.com/en/wizard/HelpWithGameIssue/?appid=730&issueid=128" target="_blank" rel="noreferrer">Find your authentication code</a></span>
                </label>

                <label class="field stack-sm" for="knownCode">
                    <span>Known share code</span>
                    <input type="text" id="knownCode" bind:value={knownCode} placeholder="CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx">
                    <span class="small muted"><a href="https://help.steampowered.com/en/wizard/HelpWithGameIssue/?appid=730&issueid=128" target="_blank" rel="noreferrer">Find a match-history share code</a></span>
                </label>

                <div class="token-actions">
                    <label class="field compact-field stack-sm" for="limit">
                        <span>Games</span>
                        <input type="number" id="limit" bind:value={limit} min="1" max="100">
                    </label>
                    <button class="chip primary-chip" type="submit" aria-busy={loadingShareCodes}>Fetch Share Codes</button>
                </div>
            </form>

            {#if shareCodes.length > 0}
                <div class="stack-sm result-panel">
                    <div class="section-heading">Share Codes ({shareCodes.length})</div>
                    <ul>
                        {#each shareCodes as item}
                            <li><code>{item.share_code}</code></li>
                        {/each}
                    </ul>
                    <p class="muted small">These are fetched without using your Steam session cookie. Download/analyze support from share codes still needs to be implemented.</p>
                </div>
            {/if}
        </article>

        <article class="card stack form-panel legacy-panel">
            <div class="form-header">
                <span class="eyebrow warning">Legacy</span>
                <h2>GCPD Cookie Scrape</h2>
                <p class="muted small">
                    Works for direct replay downloads, but requires your <code>steamLoginSecure</code> browser cookie. Treat that cookie like a password.
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
                    <span>steamLoginSecure cookie</span>
                    <input type="password" id="cookie" bind:value={cookie} placeholder="Paste cookie value here" required>
                    <span class="small muted">Required to access your private match history.</span>
                </label>

                <div class="legacy-actions">
                    <button class="chip primary-chip" type="submit" aria-busy={loadingMatches}>Load Match History</button>
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
                                    <td data-label="Action">
                                        <button
                                            class="chip"
                                            disabled={processingLink === match.link || match.processed}
                                            aria-busy={processingLink === match.link}
                                            onclick={() => processMatch(match.link)}
                                        >
                                            {match.processed ? 'Analyzed' : (match.downloaded ? 'Analyze Again' : 'Download & Analyze')}
                                        </button>
                                        {#if processingStatuses[match.link]}
                                            <div class="small muted process-status">{processingStatuses[match.link]}</div>
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
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
        max-width: 58rem;
    }

    .form-panel {
        padding: var(--space-5);
    }

    .form-panel form {
        max-width: 42rem;
    }

    .recommended-panel {
        background: linear-gradient(135deg, var(--color-surface), color-mix(in srgb, var(--color-accent) 7%, var(--color-surface-2)));
    }

    .legacy-panel {
        background: color-mix(in srgb, var(--color-warning) 5%, var(--color-surface));
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

    .eyebrow.warning {
        border-color: color-mix(in srgb, var(--color-warning) 45%, var(--color-border));
        color: var(--color-warning);
    }

    .field {
        display: grid;
        gap: var(--space-2);
        margin-bottom: 0;
    }

    .field input {
        width: 100%;
    }

    .field > span:first-child {
        font-weight: 600;
    }

    .compact-field {
        width: 9rem;
        flex: 0 0 auto;
    }

    .token-actions,
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

    .result-panel {
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-4);
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

    .process-status {
        margin-top: var(--space-2);
    }

    @media (max-width: 639px) {
        .page-head {
            align-items: flex-start;
            flex-direction: column;
        }

        .form-panel {
            padding: var(--space-4);
        }

        .token-actions,
        .legacy-actions {
            align-items: flex-start;
            flex-direction: column;
        }

        .compact-field {
            width: 100%;
        }
    }
</style>
