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
            localStorage.setItem('cs-insights:last-player', playerName);
            window.history.pushState({}, '', `?player=${encodeURIComponent(playerName)}`);
            fetchData();
        }
    }

    // ── Tree structure: Game → Round → Clusters (within 100 ticks) ──────────
    type TickCluster = { events: any[] };
    type RoundSection = { round: number; clusters: TickCluster[]; total: number };
    type GameSection  = { matchName: string; displayName: string; mapName: string; rounds: RoundSection[]; total: number };

    function buildTree(insights: any[]): GameSection[] {
        if (!insights.length) return [];

        // Preserve server sort order (desc round/tick) for insertion into maps.
        const gamesMap = new Map<string, Map<number, any[]>>();
        const gameMeta = new Map<string, { displayName: string; mapName: string }>();

        for (const ins of insights) {
            const mn = ins.MatchName;
            if (!gamesMap.has(mn)) {
                gamesMap.set(mn, new Map());
                gameMeta.set(mn, { displayName: ins.match_display || '', mapName: ins.map_name || 'Unknown map' });
            }
            const rm = gamesMap.get(mn)!;
            if (!rm.has(ins.Round)) rm.set(ins.Round, []);
            rm.get(ins.Round)!.push(ins);
        }

        const result: GameSection[] = [];

        for (const [matchName, roundsMap] of gamesMap) {
            const meta = gameMeta.get(matchName)!;
            const rounds: RoundSection[] = [];
            let total = 0;

            // Sort rounds descending
            const sortedRounds = [...roundsMap.entries()].sort((a, b) => b[0] - a[0]);

            for (const [round, evts] of sortedRounds) {
                // evts are already in descending tick order; cluster by 100-tick proximity
                const clusters: TickCluster[] = [];
                let cur: TickCluster = { events: [evts[0]] };

                for (let i = 1; i < evts.length; i++) {
                    if (Math.abs(evts[i].Tick - evts[i - 1].Tick) <= 100) {
                        cur.events.push(evts[i]);
                    } else {
                        clusters.push(cur);
                        cur = { events: [evts[i]] };
                    }
                }
                clusters.push(cur);
                rounds.push({ round, clusters, total: evts.length });
                total += evts.length;
            }

            result.push({ matchName, displayName: meta.displayName, mapName: meta.mapName, rounds, total });
        }

        return result;
    }

    // ── Unified open/close state ──────────────────────────────────────────────
    // Absent key or '!== false' means open by default (games & rounds).
    // Gunfight timeline keys use gfKey prefix and are closed by default (=== true).
    let openKeys: Record<string, boolean> = {};

    function isOpen(key: string, defaultOpen = true): boolean {
        return defaultOpen ? openKeys[key] !== false : openKeys[key] === true;
    }

    function toggle(key: string, defaultOpen = true) {
        openKeys = { ...openKeys, [key]: !isOpen(key, defaultOpen) };
    }

    function toggleDuel(gfKey: string) { toggle(gfKey, false); }

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

        <div class="section-heading">Incident Log</div>

        {#each buildTree(data.insights) as game}
            {@const gameKey = 'g-' + game.matchName}
            <div class="game-card card">

                <!-- ── Game header ─────────────────────────────────── -->
                <button class="game-header row-between" onclick={() => toggle(gameKey)}>
                    <div class="game-title">
                        <strong>{game.mapName}</strong>
                        <span class="small muted mono">{game.displayName}</span>
                    </div>
                    <div class="game-meta small muted">
                        {game.total} events · {game.rounds.length} rounds
                        <span class="tree-chevron">{isOpen(gameKey) ? '▲' : '▼'}</span>
                    </div>
                </button>

                {#if isOpen(gameKey)}
                    <div class="game-body">
                        {#each game.rounds as roundSection, ri}
                            {@const roundKey = 'r-' + game.matchName + '-' + roundSection.round}

                            <!-- ── Round section ───────────────────── -->
                            <div class="round-section" class:last={ri === game.rounds.length - 1}>
                                <button class="round-header" onclick={() => toggle(roundKey)}>
                                    <span class="round-label">Round {roundSection.round}</span>
                                    <span class="small muted">
                                        {roundSection.total} event{roundSection.total !== 1 ? 's' : ''}
                                        <span class="tree-chevron">{isOpen(roundKey) ? '▲' : '▼'}</span>
                                    </span>
                                </button>

                                {#if isOpen(roundKey)}
                                    <div class="round-clusters">
                                        {#each roundSection.clusters as cluster, ci}
                                            <!-- ── Incident cluster (within 100 ticks) ── -->
                                            <div class="event-list" class:cluster-gap={ci > 0}>
                                                {#each cluster.events as ev, i}
                                                    <div class="event-row">
                                                        <div class="event-gutter">
                                                            <div class="event-dot" style="background:{severityColor[ev.Severity]??'var(--color-accent)'}"></div>
                                                            {#if i < cluster.events.length - 1}
                                                                <div class="event-connector"></div>
                                                            {/if}
                                                        </div>
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
                                                                    {isOpen(gfKey, false) ? '▲ Hide' : '▼ Duel details'}
                                                                </button>
                                                                {#if isOpen(gfKey, false)}
                                                                    <div class="duel-timeline">
                                                                        <div class="timeline-row"><span class="t-time">0ms</span><span>Spotted</span></div>
                                                                        {#if ev.meta.target_shot_ms > 0}<div class="timeline-row you"><span class="t-time">{Math.round(ev.meta.target_shot_ms)}ms</span><span>You fired</span></div>{/if}
                                                                        {#if ev.meta.enemy_shot_ms > 0}<div class="timeline-row enemy"><span class="t-time">{Math.round(ev.meta.enemy_shot_ms)}ms</span><span>Enemy fired</span></div>{/if}
                                                                        {#if ev.meta.target_ttd_ms > 0}<div class="timeline-row you bold"><span class="t-time">{Math.round(ev.meta.target_ttd_ms)}ms</span><span>You dealt damage</span></div>{/if}
                                                                        {#if ev.meta.enemy_ttd_ms > 0}<div class="timeline-row enemy bold"><span class="t-time">{Math.round(ev.meta.enemy_ttd_ms)}ms</span><span>Enemy dealt damage</span></div>{/if}
                                                                        {#if ev.meta.crosshair_pitch > 0}<div class="timeline-note">Crosshair {ev.meta.crosshair_pitch.toFixed(1)}° {ev.meta.crosshair_dir} at duel start</div>{/if}
                                                                        {#if ev.meta.first_bullet_acc > 0}<div class="timeline-note">First bullet {ev.meta.first_bullet_acc.toFixed(1)}° off head ({ev.meta.was_peeking ? 'Peeking' : 'Holding'})</div>{/if}
                                                                    </div>
                                                                {/if}
                                                            {/if}
                                                        </div>
                                                    </div>
                                                {/each}
                                            </div>
                                        {/each}
                                    </div>
                                {/if}
                            </div>
                        {/each}
                    </div>
                {/if}

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

    /* ── Game card (top level) ───────────────────────────────────────── */
    .game-card {
        padding: 0;
        overflow: hidden;
    }

    .game-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: var(--space-3);
        width: 100%;
        padding: var(--space-3) var(--space-4);
        background: var(--color-surface-2);
        border: none;
        border-bottom: 1px solid var(--color-border);
        cursor: pointer;
        text-align: left;
        color: var(--color-text);
        font: inherit;
    }

    .game-header:hover {
        background: color-mix(in srgb, var(--color-accent) 6%, var(--color-surface-2));
    }

    .game-title {
        display: flex;
        flex-direction: column;
        gap: 0.1rem;
    }

    .game-meta {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        white-space: nowrap;
        flex-shrink: 0;
    }

    /* ── Round section ───────────────────────────────────────────────── */
    .game-body {
        display: flex;
        flex-direction: column;
    }

    .round-section {
        border-bottom: 1px solid var(--color-border);
    }

    .round-section.last {
        border-bottom: none;
    }

    .round-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        width: 100%;
        padding: var(--space-2) var(--space-4) var(--space-2) var(--space-5);
        background: transparent;
        border: none;
        cursor: pointer;
        text-align: left;
        color: var(--color-text-muted);
        font: inherit;
        font-size: 0.82rem;
    }

    .round-header:hover {
        color: var(--color-text);
        background: color-mix(in srgb, var(--color-accent) 4%, transparent);
    }

    .round-label {
        font-weight: 600;
        font-size: 0.8rem;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .round-clusters {
        padding: var(--space-2) var(--space-4) var(--space-3) var(--space-6);
        display: flex;
        flex-direction: column;
        gap: 0;
    }

    .cluster-gap {
        margin-top: var(--space-3);
        padding-top: var(--space-3);
        border-top: 1px dashed var(--color-border);
    }

    .tree-chevron {
        font-size: 0.65rem;
        opacity: 0.6;
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
