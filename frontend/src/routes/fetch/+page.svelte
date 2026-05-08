<script lang="ts">
    let steamId = '';
    let cookie = '';
    let playerName = '';
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

    <div class="card stack">
        <div class="stack-sm">
            <h2>Recommended: Steam Match History Token</h2>
            <p class="muted small">
                This uses Valve's official match-history API with your Steam Web API key and CS match-history auth code.
                It is safer than pasting <code>steamLoginSecure</code>, because it does not grant browser-session access.
                The Steam Web API key is read from the backend <code>STEAM_WEB_API_KEY</code> environment variable, so you do not need to paste it into the browser.
                For now this lists share codes only; direct demo download from share codes is the next step.
            </p>
            <p class="muted small">
                Get a Steam Web API key from <a href="https://steamcommunity.com/dev/apikey" target="_blank" rel="noreferrer">steamcommunity.com/dev/apikey</a>, then start the backend with <code>STEAM_WEB_API_KEY="your_key" make</code> from the repository root.
                For a hosted multi-user deployment, users should generally provide their own key rather than sharing one server-wide key.
            </p>
        </div>

        <form class="stack" onsubmit={fetchShareCodes}>
            <div class="grid-2">
                <label class="stack-sm" for="steamIdToken">
                    SteamID64
                    <input type="text" id="steamIdToken" bind:value={steamId} placeholder="7656119...">
                </label>
            </div>
            <div class="grid-2">
                <label class="stack-sm" for="authCode">
                    Match history auth code
                    <input type="password" id="authCode" bind:value={authCode} placeholder="steamidkey / auth code">
                </label>
                <label class="stack-sm" for="knownCode">
                    Known share code
                    <input type="text" id="knownCode" bind:value={knownCode} placeholder="CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx">
                </label>
            </div>
            <label class="stack-sm" for="limit">
                Number of games
                <input type="number" id="limit" bind:value={limit} min="1" max="100">
            </label>
            <div>
                <button class="chip primary-chip" type="submit" aria-busy={loadingShareCodes}>Fetch Share Codes</button>
            </div>
        </form>

        {#if shareCodes.length > 0}
            <div class="stack-sm">
                <div class="section-heading">Share Codes ({shareCodes.length})</div>
                <ul>
                    {#each shareCodes as item}
                        <li><code>{item.share_code}</code></li>
                    {/each}
                </ul>
                <p class="muted small">These are fetched without using your Steam session cookie. Download/analyze support from share codes still needs to be implemented.</p>
            </div>
        {/if}

        <hr>

        <div class="stack-sm">
            <h2>Legacy: GCPD Cookie Scrape</h2>
            <p class="muted small">
                This still works for direct replay downloads, but it requires your <code>steamLoginSecure</code> browser cookie.
                Treat that cookie like a password and avoid sharing or storing it.
            </p>
        </div>

        <form class="stack" onsubmit={fetchMatches}>
            <div class="grid-2">
                <label class="stack-sm" for="steamId">
                    Steam ID or Custom URL
                    <input type="text" id="steamId" bind:value={steamId} placeholder="e.g. Miconen" required>
                </label>
                <label class="stack-sm" for="playerName">
                    Exact In-Game Name
                    <input type="text" id="playerName" bind:value={playerName} placeholder="e.g. s1mple" required>
                </label>
            </div>
            <label class="stack-sm" for="cookie">
                steamLoginSecure Cookie
                <input type="password" id="cookie" bind:value={cookie} placeholder="Paste cookie value here" required>
                <span class="small muted">Required to access your private match history.</span>
            </label>
            <div>
                <button class="chip primary-chip" type="submit" aria-busy={loadingMatches}>Load Match History</button>
            </div>
        </form>

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
    </div>
</section>

<style>
    input {
        max-width: none;
    }

    .primary-chip {
        background: var(--color-accent);
        color: var(--color-accent-contrast);
        border-color: var(--color-accent);
        height: 2.25rem;
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
    }
</style>
