<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { isAuthenticated, auth } from '$lib/stores/auth';

	interface Agent {
		id: string;
		handler_id: string;
		name: string;
		description: string;
		webhook_url: string;
		is_active: boolean;
		created_at: string;
		updated_at: string;
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


			<div style="display: flex; gap: 2rem; flex-wrap: wrap; padding-top: 1rem; border-top: 1px solid #f0f0f0; font-size: 0.9rem; color: #666;">
				<span>Handler</span>
			</div>
		</div>
	{/if}
</div>
