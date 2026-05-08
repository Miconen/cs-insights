<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import Chart from 'chart.js/auto';

    const initialPlayer = $page.url.searchParams.get('player') || '';
    let playerName = initialPlayer;
    let searchInput = initialPlayer;
    let data: any = null;
    let loading = !!playerName;
    let error = '';
    let chartCanvas: HTMLCanvasElement;
    let chartInstance: any;
    let selectedMatch = 'all';

    async function fetchData() {
        if (!playerName) return;
        loading = true;
        error = '';
        try {
            // Note: Make sure the Go backend is running on 8080
            const res = await fetch(`http://localhost:8080/api/insights?player=${encodeURIComponent(playerName)}`);
            if (!res.ok) throw new Error(await res.text() || 'Failed to fetch insights');
            data = await res.json();
            
            // Render chart after data is loaded
            setTimeout(renderChart, 0);
        } catch (e: any) {
            error = e.message;
        } finally {
            loading = false;
        }
    }

    function renderChart() {
        if (!chartCanvas || !data || !data.summary.counts_by_type) return;

        if (chartInstance) {
            chartInstance.destroy();
        }

        const labels = Object.keys(data.summary.counts_by_type);
        const values = Object.values(data.summary.counts_by_type);

        chartInstance = new Chart(chartCanvas, {
            type: 'polarArea',
            data: {
                labels: labels,
                datasets: [{
                    data: values,
                    backgroundColor: [
                        'rgba(239, 68, 68, 0.7)',
                        'rgba(245, 158, 11, 0.7)',
                        'rgba(59, 130, 246, 0.7)',
                        'rgba(16, 185, 129, 0.7)',
                        'rgba(139, 92, 246, 0.7)'
                    ],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right',
                        labels: { boxWidth: 12 }
                    }
                }
            }
        });
    }

    onMount(() => {
        if (playerName) {
            fetchData();
            localStorage.setItem('cs-insights:last-player', playerName);
            return;
        }

        const rememberedPlayer = localStorage.getItem('cs-insights:last-player');
        if (rememberedPlayer) {
            playerName = rememberedPlayer;
            searchInput = rememberedPlayer;
            window.history.replaceState({}, '', `?player=${encodeURIComponent(rememberedPlayer)}`);
            fetchData();
        }
    });

    function handleSubmit(e: Event) {
        e.preventDefault();
        if (searchInput.trim()) {
            playerName = searchInput.trim();
            selectedMatch = 'all';
            localStorage.setItem('cs-insights:last-player', playerName);
            window.history.pushState({}, '', `?player=${encodeURIComponent(playerName)}`);
            fetchData();
        }
    }

    function visibleInsights() {
        if (!data?.insights) return [];
        if (selectedMatch === 'all') return data.insights;
        return data.insights.filter((insight: any) => insight.MatchName === selectedMatch);
    }

    // Group consecutive insights within the same round and ≤100 ticks apart.
    type Cluster = { lead: any; rest: any[] };

    function clusterInsights(insights: any[]): Cluster[] {
        if (!insights.length) return [];
        const result: Cluster[] = [];
        let current: Cluster = { lead: insights[0], rest: [] };

        for (let i = 1; i < insights.length; i++) {
            const prev = insights[i - 1];
            const curr = insights[i];
            if (curr.Round === prev.Round && Math.abs(curr.Tick - prev.Tick) <= 100) {
                current.rest.push(curr);
            } else {
                result.push(current);
                current = { lead: curr, rest: [] };
            }
        }
        result.push(current);
        return result;
    }

    // Track which clusters are expanded. Use a plain Record for Svelte 5 reactivity.
    let openKeys: Record<string, boolean> = {};

    function clusterKey(c: Cluster) { return `${c.lead.Round}-${c.lead.Tick}`; }

    function toggleCluster(c: Cluster) {
        const key = clusterKey(c);
        openKeys = { ...openKeys, [key]: !openKeys[key] };
    }

    function isOpen(c: Cluster) { return !!openKeys[clusterKey(c)]; }

    function copytick(tick: number, btn: HTMLElement) {
        navigator.clipboard.writeText(`demo_gototick ${tick}`);
        const orig = btn.innerText;
        btn.innerText = 'Copied!';
        setTimeout(() => btn.innerText = orig, 2000);
    }

    const severityColor: Record<string, string> = {
        High:   'var(--color-danger)',
        Medium: 'var(--color-warning)',
        Low:    'var(--color-accent)',
    };

    function formatDate(value: string) {
        if (!value) return 'Unknown date';
        return new Intl.DateTimeFormat(undefined, {
            year: 'numeric',
            month: 'short',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit'
        }).format(new Date(value));
    }
</script>

<svelte:head>
    <title>CS Insights{playerName ? ` - ${playerName}` : ''}</title>
</svelte:head>

{#if !playerName && !data && !loading}
    <section class="stack-lg hero">
        <div class="card hero-card stack">
            <div>
                <h1 class="display">CS Insights</h1>
                <p class="muted">Analyze your Counter-Strike 2 habits and turn match data into focused practice.</p>
            </div>

            <form class="search-form" onsubmit={handleSubmit}>
                <label class="stack-sm" for="player">
                    Enter player name
                    <input type="text" bind:value={searchInput} id="player" placeholder="e.g. s1mple" required>
                </label>
                <button class="chip primary-chip" type="submit" aria-busy={loading}>View Insights</button>
            </form>
        </div>
    </section>
{:else}
    <section class="stack-lg">
    <div class="row-between dashboard-head">
        <div>
            <h1 class="display">Performance Dashboard</h1>
            <p class="muted">Analysis for <mark>{playerName}</mark></p>
        </div>
        <button class="chip" onclick={() => { playerName = ''; searchInput = ''; data = null; selectedMatch = 'all'; window.history.pushState({}, '', '/'); }}>Back to Search</button>
    </div>

    {#if loading}
        <div class="card empty-state" aria-busy="true">Loading insights...</div>
    {:else if error}
        <div class="card stack-sm error-card">
            <div class="card-header">Error loading data</div>
            <p>{error}</p>
        </div>
    {:else if !data?.insights?.length}
        <div class="empty-state">
            <p>No data found</p>
            <span class="small">Run the CLI tool to parse a demo for this player first.</span>
        </div>
    {:else}
        <div class="grid-2">
            <!-- Advice Column -->
            <div class="card stack-sm">
                <div class="card-header">Coach's Advice</div>
                {#if data.advice && data.advice.length > 0}
                    <ul>
                        {#each data.advice as item}
                            <li>
                                {@html item.replace(/^(.*?):/, '<strong>$1:</strong>')}
                            </li>
                        {/each}
                    </ul>
                {:else}
                    <p>No major habits detected yet. Keep playing!</p>
                {/if}
            </div>

            <!-- Chart Column -->
            <div class="card stack-sm">
                <div class="card-header">Habit Profile</div>
                <div class="chart-wrap">
                    <canvas bind:this={chartCanvas}></canvas>
                </div>
            </div>
        </div>

        <div class="row-between incident-toolbar">
            <div class="section-heading">Raw Incident Log</div>
            {#if data.summary?.games?.length > 1}
                <label class="game-filter" for="game-filter">
                    Filter by game
                    <select id="game-filter" bind:value={selectedMatch}>
                        <option value="all">All games</option>
                        {#each data.summary.games as game}
                            <option value={game.match_name}>{game.map_name || 'Unknown map'} · {game.display_name} ({game.incident_count})</option>
                        {/each}
                    </select>
                </label>
            {/if}
        </div>
        {#each clusterInsights(visibleInsights()) as cluster}
            <!-- Single incident or cluster lead -->
            <div class="incident-card card" style="--sev: {severityColor[cluster.lead.Severity] ?? 'var(--color-accent)'}">
                <div class="incident-strip" style="background: {severityColor[cluster.lead.Severity] ?? 'var(--color-accent)'}"></div>

                <div class="incident-body">
                    <div class="incident-head">
                        <div class="incident-title">
                            <span class="incident-type">{cluster.lead.Type}</span>
                            <span class="small muted">{cluster.lead.map_name || 'Unknown map'} · {cluster.lead.match_display || ''}</span>
                        </div>
                        <div class="incident-coords mono small muted">
                            R{cluster.lead.Round} · T{cluster.lead.Tick}
                        </div>
                    </div>

                    <p class="incident-desc">{cluster.lead.Description}</p>

                    {#if cluster.lead.Type === "Gunfight" && cluster.lead.meta}
                        <div class="duel-timeline">
                            <div class="timeline-row"><span class="t-time">0ms</span><span class="t-label">Spotted</span></div>
                            {#if cluster.lead.meta.target_shot_ms > 0}
                                <div class="timeline-row you"><span class="t-time">{Math.round(cluster.lead.meta.target_shot_ms)}ms</span><span class="t-label">You fired</span></div>
                            {/if}
                            {#if cluster.lead.meta.enemy_shot_ms > 0}
                                <div class="timeline-row enemy"><span class="t-time">{Math.round(cluster.lead.meta.enemy_shot_ms)}ms</span><span class="t-label">Enemy fired</span></div>
                            {/if}
                            {#if cluster.lead.meta.target_ttd_ms > 0}
                                <div class="timeline-row you bold"><span class="t-time">{Math.round(cluster.lead.meta.target_ttd_ms)}ms</span><span class="t-label">You dealt damage</span></div>
                            {/if}
                            {#if cluster.lead.meta.enemy_ttd_ms > 0}
                                <div class="timeline-row enemy bold"><span class="t-time">{Math.round(cluster.lead.meta.enemy_ttd_ms)}ms</span><span class="t-label">Enemy dealt damage</span></div>
                            {/if}
                            {#if cluster.lead.meta.crosshair_pitch > 0}
                                <div class="timeline-note">Crosshair {cluster.lead.meta.crosshair_pitch.toFixed(1)}° {cluster.lead.meta.crosshair_dir} at duel start</div>
                            {/if}
                            {#if cluster.lead.meta.first_bullet_acc > 0}
                                <div class="timeline-note">First bullet {cluster.lead.meta.first_bullet_acc.toFixed(1)}° off head ({cluster.lead.meta.was_peeking ? 'Peeking' : 'Holding'})</div>
                            {/if}
                        </div>
                    {/if}

                    <div class="incident-footer">
                        <button class="chip" onclick={(e) => copytick(cluster.lead.Tick, e.currentTarget)} title="Copy console command">
                            demo_gototick {cluster.lead.Tick}
                        </button>
                        {#if cluster.rest.length > 0}
                            <button class="chip cluster-toggle" onclick={() => toggleCluster(cluster)}>
                                {isOpen(cluster) ? '▲' : '▼'} {cluster.rest.length} nearby {cluster.rest.length === 1 ? 'event' : 'events'}
                            </button>
                        {/if}
                    </div>

                    <!-- Collapsed cluster items -->
                    {#if cluster.rest.length > 0 && isOpen(cluster)}
                        <div class="cluster-children">
                            {#each cluster.rest as sub}
                                <div class="cluster-child">
                                    <div class="incident-head">
                                        <div class="incident-title">
                                            <span class="incident-type small">{sub.Type}</span>
                                        </div>
                                        <div class="incident-coords mono small muted">T{sub.Tick}</div>
                                    </div>
                                    <p class="incident-desc small">{sub.Description}</p>
                                    <button class="chip small-chip" onclick={(e) => copytick(sub.Tick, e.currentTarget)}>
                                        demo_gototick {sub.Tick}
                                    </button>
                                </div>
                            {/each}
                        </div>
                    {/if}
                </div>
            </div>
        {/each}
    {/if}
    </section>
{/if}

<style>
    .hero {
        min-height: min(38rem, calc(100vh - 12rem));
        justify-content: center;
    }

    .hero-card {
        width: 100%;
        max-width: none;
        padding: var(--space-6);
        background: linear-gradient(135deg, var(--color-surface), color-mix(in srgb, var(--color-accent) 8%, var(--color-surface-2)));
    }

    .hero-card h1 {
        margin-bottom: var(--space-2);
    }

    .search-form {
        display: flex;
        gap: var(--space-3);
        align-items: end;
        flex-wrap: wrap;
        max-width: 48rem;
    }

    .search-form label {
        flex: 1 1 16rem;
    }

    .search-form input {
        max-width: none;
    }

    .primary-chip {
        background: var(--color-accent);
        color: var(--color-accent-contrast);
        border-color: var(--color-accent);
        height: 2.25rem;
        flex: 0 0 auto;
    }

    .chart-wrap {
        height: 250px;
        position: relative;
    }

    .error-card {
        border-color: color-mix(in srgb, var(--color-danger) 45%, var(--color-border));
    }

    .error-card .card-header {
        color: var(--color-danger);
    }

    /* ---- Incident toolbar ---- */
    .incident-toolbar {
        align-items: end;
        gap: var(--space-4);
    }

    .game-filter {
        min-width: min(24rem, 100%);
    }

    .game-filter select {
        margin-bottom: 0;
    }

    /* ---- Incident card ---- */
    .incident-card {
        display: flex;
        gap: 0;
        padding: 0;
        overflow: hidden;
    }

    .incident-strip {
        flex: 0 0 4px;
        min-width: 4px;
        background: var(--sev, var(--color-accent));
    }

    .incident-body {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
        padding: var(--space-3) var(--space-4);
    }

    .incident-head {
        display: flex;
        justify-content: space-between;
        align-items: baseline;
        gap: var(--space-3);
    }

    .incident-title {
        display: flex;
        align-items: baseline;
        gap: var(--space-2);
        flex-wrap: wrap;
        min-width: 0;
    }

    .incident-type {
        font-weight: 700;
        font-size: 0.88rem;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .incident-coords {
        flex-shrink: 0;
        white-space: nowrap;
        font-size: 0.72rem;
    }

    .incident-desc {
        margin: 0;
        font-size: 0.85rem;
        line-height: 1.4;
    }

    .incident-footer {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
        padding-top: var(--space-1);
    }

    .cluster-toggle {
        background: transparent;
        border-color: var(--color-border);
        color: var(--color-text-muted);
        font-size: 0.72rem;
    }

    .cluster-children {
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-2);
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }

    .cluster-child {
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
        background: var(--color-surface-2);
        border-radius: var(--radius-sm);
        padding: var(--space-2) var(--space-3);
    }

    .small-chip {
        font-size: 0.68rem;
        height: auto;
        padding: 0.1rem 0.4rem;
        margin-top: var(--space-1);
    }

    /* ---- Duel timeline ---- */
    .duel-timeline {
        background: var(--color-surface-2);
        border-radius: var(--radius-sm);
        padding: var(--space-2) var(--space-3);
        display: flex;
        flex-direction: column;
        gap: 0.2rem;
    }

    .timeline-row {
        display: flex;
        gap: var(--space-2);
        font-family: var(--font-mono);
        font-size: 0.78rem;
        color: var(--color-text-muted);
    }

    .timeline-row.you { color: var(--color-accent); }
    .timeline-row.enemy { color: var(--color-danger); }
    .timeline-row.bold { font-weight: 600; }

    .t-time {
        width: 5ch;
        flex-shrink: 0;
        text-align: right;
    }

    .timeline-note {
        font-size: 0.76rem;
        color: var(--color-text-muted);
        margin-top: 0.2rem;
        border-top: 1px solid var(--color-border);
        padding-top: 0.2rem;
    }

    @media (max-width: 639px) {
        .dashboard-head,
        .incident-head,
        .incident-toolbar {
            align-items: flex-start;
            flex-direction: column;
        }

        .hero-card {
            padding: var(--space-4);
        }

        .search-form {
            align-items: flex-start;
        }

        .primary-chip {
            width: auto;
        }
    }
</style>
