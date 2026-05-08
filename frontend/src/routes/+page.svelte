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

    function handleSubmit(e: Event) {
        e.preventDefault();
        window.history.pushState({}, '', `?player=${encodeURIComponent(playerName)}`);
        fetchData();
    }
</script>

<svelte:head>
    <title>CS Insights{playerName ? ` - ${playerName}` : ''}</title>
</svelte:head>

<main class="container">
{#if !playerName && !data && !loading}
    <article>
        <header>
            <hgroup>
                <h1>CS Insights</h1>
                <p>Analyze your Counter-Strike 2 habits</p>
            </hgroup>
        </header>
        <form on:submit={handleSubmit}>
            <label for="player">
                Enter Player Name
                <input type="text" bind:value={playerName} id="player" placeholder="e.g. s1mple" required>
            </label>
            <button type="submit" aria-busy={loading}>View Insights</button>
        </form>
    </article>
{:else}
    <nav>
        <ul>
            <li><strong>Performance Dashboard</strong></li>
        </ul>
        <ul>
            <li><button class="outline" on:click={() => { playerName = ''; data = null; window.history.pushState({}, '', '/'); }}>← Back to Search</button></li>
        </ul>
    </nav>
    <p>Analysis for <mark>{playerName}</mark></p>

    {#if loading}
        <article aria-busy="true"></article>
    {:else if error}
        <article>
            <header style="background-color: var(--pico-del-color); color: white;">Error loading data</header>
            <p>{error}</p>
        </article>
    {:else if !data?.insights?.length}
        <article style="text-align: center;">
            <p>No data found</p>
            <small>Run the CLI tool to parse a demo for this player first.</small>
        </article>
    {:else}
        <div class="grid">
            <!-- Advice Column -->
            <article>
                <header>Coach's Advice</header>
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
            </article>

            <!-- Chart Column -->
            <article>
                <header>Habit Profile</header>
                <div style="height: 250px; position: relative;">
                    <canvas bind:this={chartCanvas}></canvas>
                </div>
            </article>
        </div>

        <h3>Raw Incident Log</h3>
        {#each data.insights as insight}
            <article>
                <header>
                    <div style="display: flex; justify-content: space-between;">
                        <strong>{insight.Type}</strong>
                        <small>Round {insight.Round} | Tick {insight.Tick}</small>
                    </div>
                </header>
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
                <footer>
                    <button class="outline" on:click={(e) => {
                                navigator.clipboard.writeText(`demo_gototick ${insight.Tick}`);
                                const btn = e.currentTarget;
                                const originalText = btn.innerText;
                                btn.innerText = 'Copied!';
                                setTimeout(() => btn.innerText = originalText, 2000);
                            }}
                            title="Click to copy console command">
                        demo_gototick {insight.Tick}
                    </button>
                </footer>
            </article>
        {/each}
    {/if}
{/if}
</main>
