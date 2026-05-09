<script lang="ts">
    import { onDestroy, onMount } from 'svelte';
    import { page } from '$app/stores';
    import { afterNavigate, goto } from '$app/navigation';
    import Chart from 'chart.js/auto';

    const initialPlayer = $page.url.searchParams.get('player') || '';
    let playerName = initialPlayer;
    let searchInput = initialPlayer;
    let data: any = null;
    let loading = !!playerName;
    let error = '';
    let chartCanvas: HTMLCanvasElement;
    let chartInstance: any;
    let activeEventTypes: string[] = [];
    let lastFetchedPlayer = '';

    const api = (path: string) => path;

    async function fetchData() {
        if (!playerName) return;
        loading = true;
        error = '';
        try {
            const res = await fetch(api(`/api/insights?player=${encodeURIComponent(playerName)}`));
            if (!res.ok) throw new Error(await res.text() || 'Failed to fetch insights');
            data = await res.json();
            activeEventTypes = reconcileEventTypes(activeEventTypes, eventTypes(data.insights));
            lastFetchedPlayer = playerName;
            setTimeout(renderChart, 0);
        } catch (e: any) {
            error = e.message;
        } finally {
            loading = false;
        }
    }

    function renderChart() {
        const counts = data?.summary?.counts_by_type;
        if (!chartCanvas || !counts) return;

        if (chartInstance) {
            chartInstance.destroy();
        }

        const labels = Object.keys(counts);
        const values = Object.values(counts);

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

    function destroyChart() {
        if (chartInstance) {
            chartInstance.destroy();
            chartInstance = null;
        }
    }

    function syncPlayerFromUrl() {
        const urlPlayer = $page.url.searchParams.get('player') || '';
        if (!urlPlayer) return;
        if (urlPlayer === playerName && urlPlayer === lastFetchedPlayer) return;
        playerName = urlPlayer;
        searchInput = urlPlayer;
        localStorage.setItem('cs-insights:last-player', urlPlayer);
        fetchData();
    }

    onMount(() => {
        const urlPlayer = $page.url.searchParams.get('player') || '';
        if (urlPlayer) {
            syncPlayerFromUrl();
            return;
        }

        const rememberedPlayer = localStorage.getItem('cs-insights:last-player');
        if (rememberedPlayer) {
            goto(`?player=${encodeURIComponent(rememberedPlayer)}`, { replaceState: true, noScroll: true });
        }
    });

    afterNavigate(() => {
        syncPlayerFromUrl();
    });

    onDestroy(destroyChart);

    function handleSubmit(e: Event) {
        e.preventDefault();
        if (searchInput.trim()) {
            goto(`?player=${encodeURIComponent(searchInput.trim())}`);
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

    function eventTypes(insights: any[]): string[] {
        return [...new Set((insights || []).map((ins) => ins.Type))].sort();
    }

    function eventTypeCounts(insights: any[]): Record<string, number> {
        return (insights || []).reduce((acc, ins) => {
            acc[ins.Type] = (acc[ins.Type] || 0) + 1;
            return acc;
        }, {} as Record<string, number>);
    }

    function reconcileEventTypes(selected: string[], available: string[]): string[] {
        const availableSet = new Set(available);
        return selected.filter((type) => availableSet.has(type));
    }

    function visibleInsights(insights: any[]): any[] {
        if (!insights) return [];
        if (!activeEventTypes.length) return insights;
        return insights.filter((ins) => activeEventTypes.includes(ins.Type));
    }

    function toggleEventType(type: string) {
        activeEventTypes = activeEventTypes.includes(type)
            ? activeEventTypes.filter((t) => t !== type)
            : [...activeEventTypes, type];
    }

    function eventKey(ins: any, fallback = ''): string {
        if (ins.ID) return `id-${ins.ID}`;
        const metaKey = ins.meta?.outcome || ins.meta?.fight_type || ins.Severity || '';
        return `${ins.MatchName}-${ins.Round}-${ins.Tick}-${ins.Type}-${metaKey}-${fallback}`;
    }

    function insightDomId(ins: any, fallback = ''): string {
        const raw = eventKey(ins, fallback);
        return `event-${raw.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
    }

    function scrollToInsight(ins: any) {
        openKeys = {
            ...openKeys,
            [`g-${ins.MatchName}`]: true,
            [`r-${ins.MatchName}-${ins.Round}`]: true
        };
        setTimeout(() => {
            document.getElementById(insightDomId(ins))?.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }, 0);
    }

    function gunfightOutcome(ev: any): 'won' | 'lost' | 'reset' | '' {
        if (ev.Type !== 'Gunfight') return '';
        const outcome = (ev.meta?.outcome || '').toLowerCase();
        if (outcome === 'won') return 'won';
        if (outcome === 'lost') return 'lost';
        if (outcome === 'reset') return 'reset';
        if (ev.Description?.includes('(Won)')) return 'won';
        if (ev.Description?.includes('(Lost)')) return 'lost';
        if (ev.Description?.includes('(Reset)')) return 'reset';
        return '';
    }

    function isNumber(value: unknown): value is number {
        return typeof value === 'number' && Number.isFinite(value);
    }

    function meaningfulDegrees(value: unknown): value is number {
        return isNumber(value) && value > 0.05;
    }

    function hasAimData(meta: any) {
        return meta?.timing_confidence !== 'low' && (
            meaningfulDegrees(meta.initial_aim_offset) ||
            meaningfulDegrees(meta.crosshair_pitch) ||
            meaningfulDegrees(meta.first_bullet_acc) ||
            meaningfulDegrees(meta.adjustment_needed)
        );
    }

    function compactTagClass(tag: string) {
        return tag.includes('Confidence') ? 'chip warn-chip' : 'chip soft-chip';
    }

    function splitAdvice(item: string) {
        const idx = item.indexOf(':');
        if (idx === -1) return { title: '', body: item };
        return { title: item.slice(0, idx + 1), body: item.slice(idx + 1).trimStart() };
    }

    function duelTimelineEvents(meta: any) {
        const events: { ms: number; label: string; type: string; bold: boolean; key: string }[] = [];
        const add = (ms: any, label: string, type = 'spotted', bold = false) => {
            if (typeof ms === 'number' && ms >= 0) events.push({ ms, label, type, bold, key: `${label}-${type}-${ms}` });
        };

        add(meta.target_seen_ms, 'Enemy entered your angle');
        add(meta.enemy_seen_ms, 'You entered enemy angle', 'enemy');
        if ((meta.target_seen_ms ?? -1) < 0 && (meta.enemy_seen_ms ?? -1) < 0) {
            add(meta.combat_start_ms, meta.start_source === 'damage' ? 'Damage detected' : 'Combat detected');
        } else {
            add(meta.combat_start_ms, 'Combat started');
        }
        add(meta.target_shot_ms, 'You fired', 'you');
        add(meta.enemy_shot_ms, 'Enemy fired', 'enemy');
        add(meta.target_ttd_ms, 'You dealt damage', 'you', true);
        add(meta.enemy_ttd_ms, 'Enemy dealt damage', 'enemy', true);
        if (meta.outcome === 'Reset') add(meta.resolution_ms, 'Fight reset');

        return events.sort((a, b) => a.ms - b.ms);
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
        <button class="chip" onclick={() => { localStorage.removeItem('cs-insights:last-player'); playerName = ''; searchInput = ''; data = null; activeEventTypes = []; destroyChart(); goto('/'); }}>Back to Search</button>
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
                            {@const advice = splitAdvice(item)}
                            <li>{#if advice.title}<strong>{advice.title}</strong> {/if}{advice.body}</li>
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

        <div class="section-heading">Event Log</div>

        {@const typeCounts = eventTypeCounts(data.insights)}
        <div class="incident-toolbar" aria-label="Event type filters">
            <button class:active={!activeEventTypes.length} class="chip" onclick={() => activeEventTypes = []}>All ({data.insights.length})</button>
            {#each eventTypes(data.insights) as type}
                <button class:active={activeEventTypes.includes(type)} class="chip" onclick={() => toggleEventType(type)}>{type} ({typeCounts[type] || 0})</button>
            {/each}
        </div>

        {@const filteredInsights = visibleInsights(data.insights)}
        {#if filteredInsights.length === 0}
            <div class="empty-state">
                <p>No events match the selected filters.</p>
                <button class="chip" onclick={() => activeEventTypes = []}>Clear filters</button>
            </div>
        {:else}
        <div class="event-navigator" aria-label="Event timeline navigator">
            {#each [...filteredInsights].sort((a, b) => a.Round - b.Round || a.Tick - b.Tick) as navEvent (eventKey(navEvent))}
                <button
                    class="event-nav-dot {gunfightOutcome(navEvent)}"
                    style="background:{severityColor[navEvent.Severity]??'var(--color-accent)'}"
                    title={`Round ${navEvent.Round} · T${navEvent.Tick} · ${navEvent.Type}`}
                    aria-label={`Go to round ${navEvent.Round}, tick ${navEvent.Tick}, ${navEvent.Type}${gunfightOutcome(navEvent) ? `, ${gunfightOutcome(navEvent)}` : ''}`}
                    onclick={() => scrollToInsight(navEvent)}
                ></button>
            {/each}
        </div>

        {#each buildTree(filteredInsights) as game, gi (game.matchName)}
            {@const gameKey = 'g-' + game.matchName}
            <div class="game-tree">
                <!-- ── Game header ─────────────────────────────────── -->
                <button class="game-header-row" onclick={() => toggle(gameKey)}>
                    <div style="flex: 1; text-align: left; display: flex; align-items: baseline; gap: var(--space-2); flex-wrap: wrap;">
                        <strong style="font-size: 1.25rem;">{game.mapName}</strong>
                        <span class="muted mono">{game.displayName}</span>
                        <span class="small muted">· {game.total} events</span>
                    </div>
                    <span class="tree-chevron">{openKeys[gameKey] !== false ? '▲' : '▼'}</span>
                </button>

                {#if openKeys[gameKey] !== false}
                    <div class="game-children">
                        {#each game.rounds as roundSection, ri (roundSection.round)}
                            {@const roundKey = 'r-' + game.matchName + '-' + roundSection.round}

                            <!-- ── Round node ─────────────────────────────────── -->
                            <button class="ot-row round-row-btn" onclick={() => toggle(roundKey)}>
                                <div class="ot-gutter">
                                    <div class="ot-dot round-dot"></div>
                                    {#if ri < game.rounds.length - 1 || roundSection.clusters.length > 0}
                                        <div class="ot-connector"></div>
                                    {/if}
                                </div>
                                <span class="round-label small" style="flex: 1; text-align: left;">
                                    Round {roundSection.round} <span class="muted">· {roundSection.total} event{roundSection.total !== 1 ? 's' : ''}</span>
                                </span>
                                <span class="tree-chevron" style="padding-right: var(--space-4);">{openKeys[roundKey] !== false ? '▲' : '▼'}</span>
                            </button>

                            <!-- ── Round children (events) ──────── -->
                            {#if openKeys[roundKey] !== false}
                                <div class="round-indent" class:last-round={ri === game.rounds.length - 1}>
                                    {#each roundSection.clusters as cluster, ci}
                                        <div class="event-list" class:cluster-gap={ci > 0}>
                                            {#each cluster.events as ev, i (eventKey(ev, `${ci}-${i}`))}
                                                <div id={insightDomId(ev, `${ci}-${i}`)} class="event-row {gunfightOutcome(ev)}">
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
                                                            {#if gunfightOutcome(ev)}
                                                                <span class="outcome-pill {gunfightOutcome(ev)}">{gunfightOutcome(ev)}</span>
                                                            {/if}
                                                            <button class="ev-copy chip" onclick={(e) => copytick(ev.Tick, e.currentTarget)}>copy</button>
                                                        </div>
                                                        <p class="event-desc">{ev.Description}</p>

                                                        {#if ev.Type === "Gunfight" && ev.meta}
                                                            {@const gfKey = `gf-${eventKey(ev, `${ci}-${i}`)}`}
                                                            <button class="chip cluster-toggle" onclick={() => toggleDuel(gfKey)}>
                                                                {openKeys[gfKey] ? '▲ Hide' : '▼ Duel details'}
                                                            </button>
                                                            {#if openKeys[gfKey]}
                                                                <div class="duel-timeline">
                                                                    {#if ev.meta.fight_type}
                                                                        <div class="fight-tags" style="display: flex; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 0.5rem;">
                                                                            <span class="chip" style="background: var(--color-surface-3); border-color: transparent;">{ev.meta.fight_type}</span>
                                                                            {#if ev.meta.timing_confidence}
                                                                                <span class="chip" style="border-style: dashed;">Timing: {ev.meta.timing_confidence}</span>
                                                                            {/if}
                                                                            {#if ev.meta.classification_confidence}
                                                                                <span class="chip" style="border-style: dashed;">Movement: {ev.meta.classification_confidence}</span>
                                                                            {/if}
                                                                            {#if ev.meta.tags}
                                                                                {#each ev.meta.tags as tag}
                                                                                    <span class={compactTagClass(tag)}>{tag}</span>
                                                                                {/each}
                                                                            {/if}
                                                                        </div>
                                                                    {/if}

                                                                    {#if ev.meta.analysis}
                                                                        <div class="timeline-analysis">
                                                                            <strong>{isNumber(ev.meta.rating) ? `Rating: ${ev.meta.rating}/10` : 'Rating unavailable'}</strong><br>
                                                                            {ev.meta.analysis}
                                                                        </div>
                                                                        <hr class="timeline-divider">
                                                                    {/if}

                                                                    <div class="duel-metrics">
                                                                        {#if ev.meta.start_source}<span>Source <strong>{ev.meta.start_source}</strong></span>{/if}
                                                                        {#if isNumber(ev.meta.target_damage)}<span>You <strong>{ev.meta.target_damage}</strong> dmg</span>{/if}
                                                                        {#if isNumber(ev.meta.enemy_damage)}<span>Enemy <strong>{ev.meta.enemy_damage}</strong> dmg</span>{/if}
                                                                        {#if isNumber(ev.meta.target_movement_dist)}<span>Your move <strong>{ev.meta.target_movement_dist.toFixed(0)}u</strong></span>{/if}
                                                                        {#if isNumber(ev.meta.enemy_movement_dist)}<span>Enemy move <strong>{ev.meta.enemy_movement_dist.toFixed(0)}u</strong></span>{/if}
                                                                    </div>
                                                                    
                                                                    <div class="combat-timeline">
                                                                        {#each duelTimelineEvents(ev.meta) as tEv (tEv.key)}
                                                                            <div class="combat-step {tEv.type} {tEv.bold ? 'bold' : ''}">
                                                                                <span class="step-dot"></span>
                                                                                <span class="step-time">{Math.round(tEv.ms)}ms</span>
                                                                                <span class="step-label">{tEv.label}</span>
                                                                            </div>
                                                                        {/each}
                                                                    </div>
                                                                    
                                                                    {#if hasAimData(ev.meta)}
                                                                        <div class="aim-notes">
                                                                            {#if meaningfulDegrees(ev.meta.initial_aim_offset)}<span>Initial aim <strong>{ev.meta.initial_aim_offset.toFixed(1)}°</strong> off</span>{/if}
                                                                            {#if meaningfulDegrees(ev.meta.crosshair_pitch)}<span>Height <strong>{ev.meta.crosshair_pitch.toFixed(1)}°</strong> {ev.meta.crosshair_dir || 'from head'}</span>{/if}
                                                                            {#if meaningfulDegrees(ev.meta.first_bullet_acc)}<span>First bullet <strong>{ev.meta.first_bullet_acc.toFixed(1)}°</strong> off</span>{/if}
                                                                            {#if meaningfulDegrees(ev.meta.adjustment_needed)}<span>Adjusted <strong>{ev.meta.adjustment_needed.toFixed(1)}°</strong></span>{/if}
                                                                        </div>
                                                                    {/if}
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

                    {/each}
                </div>
                {/if}
            </div>
        {/each}
        {/if}
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

    .incident-toolbar {
        display: flex;
        gap: var(--space-2);
        flex-wrap: wrap;
        align-items: center;
        margin-top: calc(var(--space-3) * -1);
    }

    .incident-toolbar .chip.active {
        background: var(--color-accent);
        border-color: var(--color-accent);
        color: var(--color-accent-contrast);
    }

    .event-navigator {
        position: sticky;
        top: calc(3rem + var(--space-2));
        z-index: 5;
        display: flex;
        align-items: center;
        gap: 0.3rem;
        padding: var(--space-2) var(--space-3);
        overflow-x: auto;
        background: color-mix(in srgb, var(--color-surface) 86%, transparent);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        backdrop-filter: blur(10px);
    }

    .event-nav-dot {
        width: 0.8rem;
        height: 1.65rem;
        flex: 0 0 auto;
        border: 1px solid color-mix(in srgb, var(--color-text) 18%, transparent);
        border-radius: 999px;
        cursor: pointer;
        opacity: 0.72;
    }

    .event-nav-dot:hover {
        opacity: 1;
        transform: translateY(-1px);
    }

    .event-nav-dot.won {
        box-shadow: 0 0 0 1px color-mix(in srgb, var(--color-accent) 75%, transparent);
    }

    .event-nav-dot.lost {
        box-shadow: 0 0 0 1px color-mix(in srgb, var(--color-danger) 75%, transparent);
    }

    .event-nav-dot.reset {
        border-radius: var(--radius-sm);
        box-shadow: 0 0 0 1px color-mix(in srgb, var(--color-warning) 75%, transparent);
    }

    /* ── Outer tree: game → round ────────────────────────────────────── */
    .game-tree {
        margin-bottom: var(--space-5);
    }

    .game-header-row {
        display: flex;
        align-items: baseline;
        gap: var(--space-2);
        flex-wrap: wrap;
        width: 100%;
        background: transparent;
        border: none;
        cursor: pointer;
        text-align: left;
        color: var(--color-text);
        font: inherit;
        padding: var(--space-2) 0;
        margin-bottom: var(--space-2);
    }

    .game-header-row:hover {
        color: var(--color-accent);
    }

    .game-children {
        display: flex;
        flex-direction: column;
    }

    .round-row-btn {
        display: flex;
        justify-content: space-between;
        align-items: center;
        width: 100%;
        padding: var(--space-1) 0;
        background: transparent;
        border: none;
        cursor: pointer;
        text-align: left;
        color: var(--color-text-muted);
        font: inherit;
    }

    .round-row-btn:hover {
        color: var(--color-text);
    }

    .ot-row {
        display: flex;
        align-items: flex-start;
        gap: var(--space-2);
    }

    .game-header-row {
        display: flex;
        align-items: baseline;
        gap: var(--space-2);
        flex-wrap: wrap;
        font-size: 1.1rem;
        border-bottom: 1px solid var(--color-border);
        background: var(--color-surface-2);
        padding: var(--space-3) var(--space-4);
    }

    .game-children {
        display: flex;
        flex-direction: column;
        margin-bottom: var(--space-6);
    }

    .ot-row {
        display: flex;
        align-items: flex-start;
        gap: var(--space-2);
    }

    .ot-gutter {
        display: flex;
        flex-direction: column;
        align-items: center;
        flex-shrink: 0;
        width: 1rem;
    }

    .ot-dot {
        border-radius: 50%;
        flex-shrink: 0;
    }

    .round-dot {
        width: 0.45rem;
        height: 0.45rem;
        background: var(--color-text-muted);
        margin-top: 0.25rem;
    }

    .ot-connector {
        flex: 1;
        width: 1px;
        background: var(--color-border);
        margin: 0.2rem 0;
        min-height: var(--space-2);
    }

    .round-label {
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .round-indent {
        /* connect cluster cards to the round node with a left border */
        border-left: 1px solid var(--color-border);
        margin-left: calc(0.5rem - 0.5px); /* centre on .round-dot (1rem width / 2) */
        padding-left: var(--space-4);
        padding-bottom: var(--space-3);
        display: flex;
        flex-direction: column;
        gap: var(--space-3);
    }

    .round-indent.last-round {
        border-left-color: transparent;
    }

    .event-list {
        display: flex;
        flex-direction: column;
        gap: 0;
    }

    .event-row {
        display: flex;
        gap: var(--space-3);
        min-width: 0;
        border-radius: var(--radius-sm);
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

    .outcome-pill {
        border-radius: 999px;
        font-size: 0.66rem;
        font-weight: 750;
        letter-spacing: 0.05em;
        line-height: 1;
        padding: 0.2rem 0.42rem;
        text-transform: uppercase;
    }

    .outcome-pill.won {
        background: color-mix(in srgb, var(--color-success) 18%, transparent);
        color: var(--color-success);
    }

    .outcome-pill.lost {
        background: color-mix(in srgb, var(--color-danger) 18%, transparent);
        color: var(--color-danger);
    }

    .outcome-pill.reset {
        background: color-mix(in srgb, var(--color-warning) 18%, transparent);
        color: var(--color-warning);
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
        padding: var(--space-3);
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
        margin-top: var(--space-1);
    }

    .combat-timeline {
        position: relative;
        display: grid;
        gap: 0.1rem;
        padding-left: 0.25rem;
    }

    .combat-step {
        position: relative;
        display: grid;
        grid-template-columns: 0.8rem 7ch 1fr;
        align-items: center;
        gap: var(--space-2);
        font-family: var(--font-mono);
        font-size: 0.76rem;
        color: var(--color-text-muted);
        min-height: 1.35rem;
    }

    .combat-step::before {
        content: '';
        position: absolute;
        left: 0.32rem;
        top: -0.45rem;
        bottom: 0.85rem;
        width: 1px;
        background: var(--color-border);
    }

    .combat-step:first-child::before {
        display: none;
    }

    .step-dot {
        position: relative;
        z-index: 1;
        width: 0.45rem;
        height: 0.45rem;
        border-radius: 999px;
        background: var(--color-text-muted);
    }

    .combat-step.you { color: var(--color-accent); }
    .combat-step.enemy { color: var(--color-danger); }
    .combat-step.bold { font-weight: 650; }

    .combat-step.you .step-dot { background: var(--color-accent); }
    .combat-step.enemy .step-dot { background: var(--color-danger); }

    .step-time {
        text-align: right;
        white-space: nowrap;
    }

    .step-label {
        font-family: var(--font-body);
        letter-spacing: 0;
    }

    .timeline-analysis {
        font-size: 0.82rem;
        color: var(--color-text);
        line-height: 1.4;
        margin-bottom: var(--space-1);
    }

    .duel-metrics {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-1) var(--space-2);
        color: var(--color-text-muted);
        font-size: 0.74rem;
    }

    .duel-metrics span {
        background: color-mix(in srgb, var(--color-surface) 55%, transparent);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        padding: 0.1rem 0.35rem;
    }

    .duel-metrics strong {
        color: var(--color-text);
    }

    .soft-chip,
    .warn-chip {
        border-style: dashed;
    }

    .warn-chip {
        color: var(--color-warning);
        border-color: color-mix(in srgb, var(--color-warning) 45%, var(--color-border));
        background: color-mix(in srgb, var(--color-warning) 8%, transparent);
    }

    .timeline-divider {
        border: none;
        border-top: 1px solid var(--color-border);
        margin: var(--space-1) 0;
    }

    .aim-notes {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-1) var(--space-2);
        font-size: 0.74rem;
        color: var(--color-text-muted);
        border-top: 1px solid var(--color-border);
        padding-top: var(--space-2);
    }

    .aim-notes strong {
        color: var(--color-text);
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
