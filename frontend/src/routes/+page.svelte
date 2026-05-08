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

    // Group insights by Game -> Round
    type RoundGroup = { matchName: string; matchDisplay: string; mapName: string; round: number; events: any[] };

    function groupInsights(insights: any[]): RoundGroup[] {
        if (!insights.length) return [];
        const result: RoundGroup[] = [];
        let current: RoundGroup = { 
            matchName: insights[0].MatchName, 
            matchDisplay: insights[0].match_display,
            mapName: insights[0].map_name,
            round: insights[0].Round, 
            events: [] 
        };

        for (let i = 0; i < insights.length; i++) {
            const curr = insights[i];
            if (curr.MatchName === current.matchName && curr.Round === current.round) {
                current.events.push(curr);
            } else {
                result.push(current);
                current = { 
                    matchName: curr.MatchName, 
                    matchDisplay: curr.match_display,
                    mapName: curr.map_name,
                    round: curr.Round, 
                    events: [curr] 
                };
            }
        }
        result.push(current);
        return result;
    }

    // Track which clusters are expanded. Use a plain Record for Svelte 5 reactivity.
    let openKeys: Record<string, boolean> = {};

    function toggleDuel(gfKey: string) {
        openKeys = { ...openKeys, [gfKey]: !openKeys[gfKey] };
    }

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
        {#each groupInsights(visibleInsights()) as group}
            <div class="event-group card">
                <!-- Context header -->
                <div class="event-group-header row-between">
                    <span class="small muted">{group.mapName || 'Unknown map'} · {group.matchDisplay || ''}</span>
                    <strong class="mono">Round {group.round}</strong>
                </div>

                <!-- Vertical event list -->
                <div class="event-list">
                    {#each group.events as ev, i}
                        <div class="event-row">
                            <!-- Gutter: dot + connecting line -->
                            <div class="event-gutter">
                                <div class="event-dot" style="background: {severityColor[ev.Severity] ?? 'var(--color-accent)'}"></div>
                                {#if i < group.events.length - 1}
                                    <div class="event-connector"></div>
                                {/if}
                            </div>

                            <!-- Content -->
                            <div class="event-content">
                                <div class="event-row-head">
                                    <span class="event-type">{ev.Type}</span>
                                    <span class="mono muted" style="font-size:0.7rem">T{ev.Tick}</span>
                                    <button class="ev-copy chip" onclick={(e) => copytick(ev.Tick, e.currentTarget)}>copy</button>
                                </div>
                                <p class="event-desc">{ev.Description}</p>

                                {#if ev.Type === "Gunfight" && ev.meta}
                                    {@const gfKey = `gf-${ev.Round}-${ev.Tick}`}
                                    <button class="chip cluster-toggle" onclick={() => toggleDuel(gfKey)}>
                                        {openKeys[gfKey] ? '▲ Hide' : '▼ Duel details'}
                                    </button>
                                    {#if openKeys[gfKey]}
                                        <div class="duel-timeline">
                                            <div class="timeline-row"><span class="t-time">0ms</span><span>Spotted</span></div>
                                            {#if ev.meta.target_shot_ms > 0}
                                                <div class="timeline-row you"><span class="t-time">{Math.round(ev.meta.target_shot_ms)}ms</span><span>You fired</span></div>
                                            {/if}
                                            {#if ev.meta.enemy_shot_ms > 0}
                                                <div class="timeline-row enemy"><span class="t-time">{Math.round(ev.meta.enemy_shot_ms)}ms</span><span>Enemy fired</span></div>
                                            {/if}
                                            {#if ev.meta.target_ttd_ms > 0}
                                                <div class="timeline-row you bold"><span class="t-time">{Math.round(ev.meta.target_ttd_ms)}ms</span><span>You dealt damage</span></div>
                                            {/if}
                                            {#if ev.meta.enemy_ttd_ms > 0}
                                                <div class="timeline-row enemy bold"><span class="t-time">{Math.round(ev.meta.enemy_ttd_ms)}ms</span><span>Enemy dealt damage</span></div>
                                            {/if}
                                            {#if ev.meta.crosshair_pitch > 0}
                                                <div class="timeline-note">Crosshair {ev.meta.crosshair_pitch.toFixed(1)}° {ev.meta.crosshair_dir} at duel start</div>
                                            {/if}
                                            {#if ev.meta.first_bullet_acc > 0}
                                                <div class="timeline-note">First bullet {ev.meta.first_bullet_acc.toFixed(1)}° off head ({ev.meta.was_peeking ? 'Peeking' : 'Holding'})</div>
                                            {/if}
                                        </div>
                                    {/if}
                                {/if}
                            </div>
                        </div>
                    {/each}
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

    /* ---- Event group card ---- */
    .event-group {
        padding: 0;
        overflow: hidden;
    }

    .event-group-header {
        padding: var(--space-2) var(--space-4);
        border-bottom: 1px solid var(--color-border);
        background: var(--color-surface-2);
    }

    .event-list {
        padding: var(--space-3) var(--space-4);
        display: flex;
        flex-direction: column;
        gap: 0;
    }

    .event-row {
        display: flex;
        gap: var(--space-3);
        min-width: 0;
    }

    /* Left gutter: dot + vertical line */
    .event-gutter {
        display: flex;
        flex-direction: column;
        align-items: center;
        flex-shrink: 0;
        width: 1rem;
    }

    .event-dot {
        width: 0.55rem;
        height: 0.55rem;
        border-radius: 50%;
        flex-shrink: 0;
        margin-top: 0.3rem;
    }

    .event-connector {
        flex: 1;
        width: 1px;
        background: var(--color-border);
        margin: 0.2rem 0;
        min-height: 0.75rem;
    }

    .event-content {
        flex: 1;
        min-width: 0;
        padding-bottom: var(--space-3);
        display: flex;
        flex-direction: column;
        gap: var(--space-1);
    }

    .event-row:last-child .event-content {
        padding-bottom: 0;
    }

    .event-row-head {
        display: flex;
        align-items: baseline;
        gap: var(--space-2);
        flex-wrap: wrap;
    }

    .event-type {
        font-weight: 700;
        font-size: 0.82rem;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .ev-copy {
        font-size: 0.68rem;
        padding: 0.08rem 0.35rem;
        height: auto;
        margin-left: auto;
        color: var(--color-text-muted);
        background: transparent;
    }

    .event-desc {
        margin: 0;
        font-size: 0.84rem;
        color: var(--color-text-muted);
        line-height: 1.4;
    }

    .cluster-toggle {
        background: transparent;
        border-color: var(--color-border);
        color: var(--color-text-muted);
        font-size: 0.72rem;
        align-self: flex-start;
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
        margin-top: var(--space-1);
    }

    .timeline-row {
        display: flex;
        gap: var(--space-2);
        font-family: var(--font-mono);
        font-size: 0.76rem;
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
        font-size: 0.74rem;
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
