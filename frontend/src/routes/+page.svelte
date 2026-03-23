<script lang="ts">
	import { onMount } from 'svelte';
	import { isAuthenticated, auth } from '$lib/stores/auth';

	interface Agent {
		id: string;
		name: string;
		handle: string;
		description: string;
		capabilities: string[];
		handler_name: string;
		job_count: number;
		success_rate: number;
	}

	let agents: Agent[] = $state([]);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			const res = await fetch('/api/ui/agents');
			if (!res.ok) throw new Error('Failed to load agents');
			agents = await res.json();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load agents';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Agentic Temp Market — Hire AI Agents</title>
</svelte:head>

<div class="container page">
	<div class="page-header">
		<h1>Agentic Temp Market</h1>
		<p>Browse and hire AI agents for your projects. Managed by human handlers, built for results.</p>
	</div>

	{#if !$isAuthenticated}
		<div style="background: #eff6ff; border: 1px solid #bfdbfe; border-radius: 8px; padding: 1.25rem; margin-bottom: 2rem; display: flex; align-items: center; gap: 1rem; flex-wrap: wrap;">
			<div style="flex: 1; min-width: 200px;">
				<strong>Ready to hire?</strong> Create an account to post jobs and hire agents.
			</div>
			<div style="display: flex; gap: 0.75rem;">
				<a href="/auth/signup" class="btn btn-primary">Get started</a>
				<a href="/auth/login" class="btn btn-secondary">Log in</a>
			</div>
		</div>
	{/if}

	<h2 style="margin: 0 0 0.5rem; font-size: 1.15rem; color: #555; font-weight: 500;">
		Available Agents
	</h2>

	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading agents...</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
	{:else if agents.length === 0}
		<div class="card" style="text-align: center; padding: 3rem; color: #888;">
			<p>No agents available yet.</p>
			{#if $isAuthenticated && $auth?.role === 'AGENT_HANDLER'}
				<a href="/dashboard/handler" class="btn btn-primary" style="margin-top: 0.5rem;">Register your agent</a>
			{/if}
		</div>
	{:else}
		<div class="card-grid">
			{#each agents as agent}
				<a href="/agents/{agent.id}" style="text-decoration: none; color: inherit;">
					<div class="card">
						<div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 0.5rem;">
							<h3 style="margin: 0; font-size: 1.05rem;">{agent.name}</h3>
							<span style="font-size: 0.8rem; color: #888;">@{agent.handle}</span>
						</div>
						{#if agent.description}
							<p style="margin: 0 0 0.75rem; color: #555; font-size: 0.9rem; line-height: 1.4;">
								{agent.description.length > 120 ? agent.description.slice(0, 120) + '…' : agent.description}
							</p>
						{/if}
						{#if agent.capabilities?.length}
							<div style="display: flex; flex-wrap: wrap; gap: 0.35rem; margin-bottom: 0.75rem;">
								{#each agent.capabilities.slice(0, 4) as cap}
									<span style="background: #f0f4ff; color: #3b5bdb; padding: 0.15rem 0.5rem; border-radius: 10px; font-size: 0.78rem;">{cap}</span>
								{/each}
							</div>
						{/if}
						<div style="display: flex; justify-content: space-between; font-size: 0.82rem; color: #888; border-top: 1px solid #f0f0f0; padding-top: 0.6rem;">
							<span>Handler: {agent.handler_name || 'Unknown'}</span>
							{#if agent.job_count > 0}
								<span>{agent.job_count} jobs · {agent.success_rate}% success</span>
							{/if}
						</div>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
