<script lang="ts">
    let steamId = '';
    let cookie = '';
    let playerName = '';
    
    let loadingMatches = false;
    let matches: any[] = [];
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

    let processingLink = '';

    async function processMatch(link: string) {
        if (!playerName) {
            alert('Please enter your Player Name before processing.');
            return;
        }
        processingLink = link;
        try {
            const res = await fetch('http://localhost:8080/api/fetch/process', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ link, player_name: playerName })
            });
            if (!res.ok) throw new Error(await res.text());
            
            // Mark as processed in UI
            matches = matches.map(m => m.link === link ? { ...m, processed: true, downloaded: true } : m);
            alert('Match parsed successfully! You can view insights on the Dashboard.');
        } catch (e: any) {
            alert('Error processing match: ' + e.message);
        } finally {
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

    @media (max-width: 639px) {
        .page-head {
            align-items: flex-start;
            flex-direction: column;
        }
    }
</style>
