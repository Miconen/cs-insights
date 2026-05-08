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

{#if !playerName && !data && !loading}
    <div class="bg-slate-900 min-h-screen flex items-center justify-center">
        <div class="bg-slate-800 p-8 rounded-xl shadow-2xl w-96 border border-slate-700">
            <h1 class="text-3xl font-bold mb-6 text-white text-center">CS Insights</h1>
            <form on:submit={handleSubmit} class="space-y-4">
                <div>
                    <label for="player" class="block text-sm font-medium text-slate-300 mb-2">Enter Player Name</label>
                    <input type="text" bind:value={playerName} id="player" class="block w-full rounded-md border-0 py-2.5 px-3 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6 bg-slate-100" placeholder="e.g. s1mple">
                </div>
                <button type="submit" class="w-full flex justify-center py-2.5 px-4 border border-transparent rounded-md shadow-sm text-sm font-bold text-white bg-indigo-600 hover:bg-indigo-500 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500">
                    View Insights
                </button>
            </form>
        </div>
    </div>
{:else}
    <div class="bg-slate-50 min-h-screen text-slate-900 font-sans">
        <div class="max-w-6xl mx-auto py-10 px-4 sm:px-6 lg:px-8">
            <div class="mb-8 flex justify-between items-end border-b pb-4">
                <div>
                    <h1 class="text-4xl font-extrabold tracking-tight text-slate-900">Performance Dashboard</h1>
                    <p class="text-lg text-slate-500 mt-2">Analysis for <span class="font-bold text-indigo-600">{playerName}</span></p>
                </div>
                <button on:click={() => { playerName = ''; data = null; window.history.pushState({}, '', '/'); }} class="text-sm font-medium text-indigo-600 hover:text-indigo-500">← Back to Search</button>
            </div>

            {#if loading}
                <div class="text-center py-20">
                    <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-600 mx-auto"></div>
                    <p class="mt-4 text-slate-500 font-medium">Fetching insights from engine...</p>
                </div>
            {:else if error}
                <div class="bg-red-50 border-l-4 border-red-500 p-4 rounded-md">
                    <div class="flex">
                        <div class="ml-3">
                            <h3 class="text-sm font-medium text-red-800">Error loading data</h3>
                            <div class="mt-2 text-sm text-red-700">
                                <p>{error}</p>
                            </div>
                        </div>
                    </div>
                </div>
            {:else if !data?.insights?.length}
                <div class="bg-white shadow rounded-lg p-10 text-center text-slate-500 border border-slate-200">
                    <svg class="mx-auto h-12 w-12 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                        <path vector-effect="non-scaling-stroke" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 13h6m-3-3v6m-9 1V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" />
                    </svg>
                    <h3 class="mt-2 text-sm font-medium text-slate-900">No data found</h3>
                    <p class="mt-1 text-sm text-slate-500">Run the CLI tool to parse a demo for this player first.</p>
                </div>
            {:else}
                <div class="grid grid-cols-1 md:grid-cols-3 gap-8 mb-10">
                    <!-- Advice Column -->
                    <div class="md:col-span-2 space-y-6">
                        <div class="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden">
                            <div class="bg-indigo-600 px-6 py-4">
                                <h2 class="text-xl font-bold text-white">Coach's Advice</h2>
                            </div>
                            <div class="p-6">
                                {#if data.advice && data.advice.length > 0}
                                    <ul class="space-y-4">
                                        {#each data.advice as item}
                                            <li class="flex gap-4 items-start bg-indigo-50/50 p-4 rounded-lg border border-indigo-100">
                                                <div class="text-slate-800 leading-relaxed text-sm">
                                                    <!-- Basic parsing of the string to bold the title part -->
                                                    {@html item.replace(/^(.*?):/, '<b>$1:</b>')}
                                                </div>
                                            </li>
                                        {/each}
                                    </ul>
                                {:else}
                                    <p class="text-slate-500 text-center py-4">No major habits detected yet. Keep playing!</p>
                                {/if}
                            </div>
                        </div>
                    </div>

                    <!-- Chart Column -->
                    <div class="bg-white rounded-xl shadow-sm border border-slate-200 p-6 flex flex-col items-center justify-center">
                        <h3 class="text-lg font-bold text-slate-800 w-full text-center mb-4">Habit Profile</h3>
                        <div class="w-full relative" style="height: 250px;">
                            <canvas bind:this={chartCanvas}></canvas>
                        </div>
                    </div>
                </div>

                <h2 class="text-2xl font-bold mb-6 text-slate-800">Raw Incident Log</h2>
                <div class="space-y-4">
                    {#each data.insights as insight}
                        <div class="bg-white shadow-sm rounded-lg p-5 border-l-4 border border-y-slate-200 border-r-slate-200
                            {insight.Severity === 'High' ? 'border-l-red-500' : insight.Severity === 'Medium' ? 'border-l-amber-500' : 'border-l-blue-500'} hover:shadow-md transition-shadow">
                            <div class="flex justify-between items-start">
                                <div>
                                    <span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold uppercase tracking-wider
                                        {insight.Severity === 'High' ? 'bg-red-100 text-red-800' : insight.Severity === 'Medium' ? 'bg-amber-100 text-amber-800' : 'bg-blue-100 text-blue-800'}">
                                        {insight.Type}
                                    </span>
                                    <h3 class="mt-3 text-lg font-medium text-slate-900">{insight.Description}</h3>
                                    
                                    {#if insight.Type === "Gunfight" && insight.meta}
                                        <div class="mt-4 bg-slate-50 p-3 rounded border border-slate-200">
                                            <div class="text-xs font-bold text-slate-500 mb-2 uppercase tracking-wide">Duel Timeline</div>
                                            <div class="flex flex-col space-y-1 text-sm font-mono text-slate-700">
                                                <div class="flex"><span class="w-16 text-slate-400">0ms:</span> Spotted</div>
                                                {#if insight.meta.target_shot_ms > 0}
                                                    <div class="flex"><span class="w-16 text-indigo-500">{Math.round(insight.meta.target_shot_ms)}ms:</span> You fired</div>
                                                {/if}
                                                {#if insight.meta.enemy_shot_ms > 0}
                                                    <div class="flex"><span class="w-16 text-rose-500">{Math.round(insight.meta.enemy_shot_ms)}ms:</span> Enemy fired</div>
                                                {/if}
                                                {#if insight.meta.target_ttd_ms > 0}
                                                    <div class="flex"><span class="w-16 text-indigo-500 font-bold">{Math.round(insight.meta.target_ttd_ms)}ms:</span> You dealt damage</div>
                                                {/if}
                                                {#if insight.meta.enemy_ttd_ms > 0}
                                                    <div class="flex"><span class="w-16 text-rose-500 font-bold">{Math.round(insight.meta.enemy_ttd_ms)}ms:</span> Enemy dealt damage</div>
                                                {/if}
                                            </div>
                                            {#if insight.meta.crosshair_pitch > 0}
                                            <div class="mt-3 pt-3 border-t border-slate-200 text-sm text-slate-600">
                                                <span class="font-bold">Crosshair Placement:</span> At the start of the duel, your crosshair was {insight.meta.crosshair_pitch.toFixed(1)}° {insight.meta.crosshair_dir}.
                                            </div>
                                            {/if}
                                            {#if insight.meta.first_bullet_acc > 0}
                                            <div class="mt-2 text-sm text-slate-600">
                                                <span class="font-bold">First Bullet Accuracy:</span> When you fired your first shot, your crosshair was {insight.meta.first_bullet_acc.toFixed(1)}° off the enemy's head (stance: {insight.meta.was_peeking ? 'Peeking' : 'Holding'}).
                                            </div>
                                            {/if}
                                        </div>
                                    {/if}
                                </div>
                                <div class="text-sm text-slate-500 text-right font-medium flex flex-col items-end">
                                    <div class="bg-slate-100 px-3 py-1 rounded-md mb-1 border border-slate-200">Round {insight.Round}</div>
                                    <div class="text-xs text-slate-400 mb-2">Tick {insight.Tick}</div>
                                    <button on:click={(e) => {
                                                navigator.clipboard.writeText(`demo_gototick ${insight.Tick}`);
                                                const btn = e.currentTarget;
                                                const originalText = btn.innerText;
                                                btn.innerText = 'Copied!';
                                                setTimeout(() => btn.innerText = originalText, 2000);
                                            }}
                                            class="text-xs font-mono bg-slate-800 text-slate-300 px-2 py-1.5 rounded hover:bg-slate-700 hover:text-white transition-colors border border-slate-700 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                                            title="Click to copy console command">
                                        demo_gototick {insight.Tick}
                                    </button>
                                </div>
                            </div>
                        </div>
                    {/each}
                </div>
            {/if}
        </div>
    </div>
{/if}