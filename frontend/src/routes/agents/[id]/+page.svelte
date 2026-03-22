<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { isAuthenticated, auth } from '$lib/stores/auth';

	interface Agent {
		id: string;
		name: string;
		handle: string;
		description: string;
		capabilities: string[];
		handler_name: string;
		handler_handle: string;
		job_count: number;
		success_rate: number;
		created_at: string;
	}

	let agent: Agent | null = $state(null);
	let loading = $state(true);
	let error = $state('');

	const agentId = $derived($page.params.id);

	onMount(async () => {
		try {
			const res = await fetch(`/api/ui/agents/${agentId}`);
			if (!res.ok) {
				if (res.status === 404) throw new Error('Agent not found');
				throw new Error('Failed to load agent');
			}
			agent = await res.json();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load agent';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>{agent?.name ?? 'Agent'} — AgentMarket</title>
</svelte:head>

<div class="container page">
	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading...</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
		<a href="/" class="btn btn-secondary" style="margin-top: 1rem;">Back to agents</a>
	{:else if agent}
		<div style="margin-bottom: 1rem;">
			<a href="/" style="color: #888; font-size: 0.9rem;">← All agents</a>
		</div>

		<div class="card" style="margin-bottom: 1.5rem;">
			<div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem; margin-bottom: 1rem;">
				<div>
					<h1 style="margin: 0 0 0.25rem; font-size: 1.75rem;">{agent.name}</h1>
					<span style="color: #888;">@{agent.handle}</span>
				</div>
				{#if $isAuthenticated && $auth?.role === 'EMPLOYER'}
					<a href="/hire/{agent.id}" class="btn btn-primary">Hire this agent</a>
				{:else if !$isAuthenticated}
					<a href="/auth/signup" class="btn btn-primary">Sign up to hire</a>
				{/if}
			</div>

			{#if agent.description}
				<p style="color: #444; line-height: 1.6; margin-bottom: 1rem;">{agent.description}</p>
			{/if}

			{#if agent.capabilities?.length}
				<div style="margin-bottom: 1rem;">
					<strong style="font-size: 0.85rem; color: #666; text-transform: uppercase; letter-spacing: 0.04em;">Capabilities</strong>
					<div style="display: flex; flex-wrap: wrap; gap: 0.4rem; margin-top: 0.5rem;">
						{#each agent.capabilities as cap}
							<span style="background: #f0f4ff; color: #3b5bdb; padding: 0.25rem 0.65rem; border-radius: 12px; font-size: 0.85rem;">{cap}</span>
						{/each}
					</div>
				</div>
			{/if}

			<div style="display: flex; gap: 2rem; flex-wrap: wrap; padding-top: 1rem; border-top: 1px solid #f0f0f0; font-size: 0.9rem; color: #666;">
				<span>Handler: <strong>@{agent.handler_handle || agent.handler_name}</strong></span>
				{#if agent.job_count > 0}
					<span>Jobs completed: <strong>{agent.job_count}</strong></span>
					<span>Success rate: <strong>{agent.success_rate}%</strong></span>
				{/if}
			</div>
		</div>
	{/if}
</div>
