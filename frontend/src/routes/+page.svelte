<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import Chart from 'chart.js/auto';

    let playerName = $page.url.searchParams.get('player') || '';
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
        }
    });

    let searchTimeout: any;

    function handleInput() {
        clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => {
            if (playerName.trim().length > 0) {
                window.history.replaceState({}, '', `?player=${encodeURIComponent(playerName.trim())}`);
                fetchData();
            } else {
                data = null;
                window.history.replaceState({}, '', '/');
            }
        }, 400); // 400ms debounce
    }

    function handleSubmit(e: Event) {
        e.preventDefault();
        clearTimeout(searchTimeout);
        if (playerName.trim()) {
            window.history.pushState({}, '', `?player=${encodeURIComponent(playerName.trim())}`);
            fetchData();
        }
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
                    <input type="text" bind:value={playerName} oninput={handleInput} id="player" placeholder="e.g. s1mple" required>
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
        <button class="chip" onclick={() => { playerName = ''; data = null; window.history.pushState({}, '', '/'); }}>Back to Search</button>
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

        <div class="section-heading">Raw Incident Log</div>
        {#each data.insights as insight}
            <div class="card stack-sm">
                <div class="row-between incident-head">
                        <strong>{insight.Type}</strong>
                        <span class="small muted mono">Round {insight.Round} | Tick {insight.Tick}</span>
                </div>
                <p>{insight.Description}</p>
                
                {#if insight.Type === "Gunfight" && insight.meta}
                    <blockquote>
                        <p><strong>Duel Timeline</strong></p>
                        <ul>
                            <li><small>0ms:</small> Spotted</li>
                            {#if insight.meta.target_shot_ms > 0}
                                <li><small>{Math.round(insight.meta.target_shot_ms)}ms:</small> You fired</li>
                            {/if}
                            {#if insight.meta.enemy_shot_ms > 0}
                                <li><small>{Math.round(insight.meta.enemy_shot_ms)}ms:</small> Enemy fired</li>
                            {/if}
                            {#if insight.meta.target_ttd_ms > 0}
                                <li><small>{Math.round(insight.meta.target_ttd_ms)}ms:</small> You dealt damage</li>
                            {/if}
                            {#if insight.meta.enemy_ttd_ms > 0}
                                <li><small>{Math.round(insight.meta.enemy_ttd_ms)}ms:</small> Enemy dealt damage</li>
                            {/if}
                        </ul>
                        {#if insight.meta.crosshair_pitch > 0}
                            <hr>
                            <small><strong>Crosshair Placement:</strong> At the start of the duel, your crosshair was {insight.meta.crosshair_pitch.toFixed(1)}° {insight.meta.crosshair_dir}.</small>
                        {/if}
                        {#if insight.meta.first_bullet_acc > 0}
                            <br>
                            <small><strong>First Bullet Accuracy:</strong> When you fired your first shot, your crosshair was {insight.meta.first_bullet_acc.toFixed(1)}° off the enemy's head (stance: {insight.meta.was_peeking ? 'Peeking' : 'Holding'}).</small>
                        {/if}
                    </blockquote>
                {/if}
                <div>
                    <button class="chip" onclick={(e) => {
                                navigator.clipboard.writeText(`demo_gototick ${insight.Tick}`);
                                const btn = e.currentTarget;
                                const originalText = btn.innerText;
                                btn.innerText = 'Copied!';
                                setTimeout(() => btn.innerText = originalText, 2000);
                            }}
                            title="Click to copy console command">
                        demo_gototick {insight.Tick}
                    </button>
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
        max-width: 42rem;
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

    .incident-head {
        align-items: flex-start;
    }

    @media (max-width: 639px) {
        .dashboard-head,
        .incident-head {
            align-items: flex-start;
            flex-direction: column;
        }

        .hero-card {
            padding: var(--space-4);
        }

        .search-form button {
            width: 100%;
            justify-content: center;
        }
    }
</style>
