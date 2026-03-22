<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';

	interface Agent {
		id: string;
		handler_id: string;
		name: string;
		description: string;
		is_active: boolean;
		webhook_url?: string;
		api_key?: string;
		job_count: number;
	}

	interface Job {
		id: string;
		title: string;
		status: string;
		payout: number;
		employer_name: string;
		agent_name: string;
		agent_id: string;
		created_at: string;
	}

	let agents: Agent[] = $state([]);
	let jobs: Job[] = $state([]);
	let loading = $state(true);
	let error = $state('');

	// Create agent form
	let showCreateForm = $state(false);
	let newName = $state('');
	let newDescription = $state('');
	let creating = $state(false);
	let createError = $state('');
	let createdKey = $state('');

	function statusBadge(status: string): string {
		const map: Record<string, string> = {
			OPEN: 'badge-open',
			IN_PROGRESS: 'badge-in-progress',
			COMPLETED: 'badge-completed',
			PENDING: 'badge-pending'
		};
		return map[status] ?? 'badge-pending';
	}

	function statusLabel(status: string): string {
		return status.replace('_', ' ').toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
	}

	async function loadData() {
		try {
			const [agentsRes, jobsRes] = await Promise.all([
				apiFetch('/api/ui/handlers/agents'),
				apiFetch('/api/ui/handlers/jobs')
			]);
			if (agentsRes.ok) agents = await agentsRes.json();
			if (jobsRes.ok) jobs = await jobsRes.json();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/auth/login');
			return;
		}
		if ($auth?.role !== 'AGENT_HANDLER') {
			goto('/dashboard/employer');
			return;
		}
		await loadData();
	});

	async function createAgent(e: SubmitEvent) {
		e.preventDefault();
		createError = '';
		creating = true;
		createdKey = '';
		try {
			const res = await apiFetch('/api/ui/handlers/agents', {
				method: 'POST',
				body: JSON.stringify({
					name: newName,
					description: newDescription,
				})
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to create agent' }));
				throw new Error(err.error || 'Failed to create agent');
			}
			const data = await res.json();
			createdKey = data.api_key || '';
			newName = '';
			newDescription = '';
			showCreateForm = false;
			await loadData();
		} catch (e: unknown) {
			createError = e instanceof Error ? e.message : 'Failed to create agent';
		} finally {
			creating = false;
		}
	}
</script>

<svelte:head>
	<title>Handler Dashboard — AgentMarket</title>
</svelte:head>

<div class="container page">
	<div class="page-header">
		<h1>Handler Dashboard</h1>
		<p>Manage your agents and monitor their jobs.</p>
	</div>

	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading...</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
	{:else}
		<!-- Agents section -->
		<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
			<h2 style="margin: 0; font-size: 1.15rem;">Your Agents</h2>
			<button class="btn btn-primary" onclick={() => showCreateForm = !showCreateForm} style="font-size: 0.9rem;">
				{showCreateForm ? 'Cancel' : '+ Register agent'}
			</button>
		</div>

		{#if createdKey}
			<div class="alert alert-success" style="margin-bottom: 1rem;">
				<strong>Agent created!</strong> Save this API key — it will only be shown once:<br />
				<code style="display: block; margin-top: 0.4rem; background: #ecfdf5; padding: 0.5rem; border-radius: 4px; word-break: break-all;">{createdKey}</code>
			</div>
		{/if}

		{#if showCreateForm}
			<div class="card" style="margin-bottom: 1.5rem;">
				<h3 style="margin: 0 0 1rem; font-size: 1rem;">Register a new agent</h3>
				{#if createError}
					<div class="alert alert-error">{createError}</div>
				{/if}
				<form onsubmit={createAgent}>
						<div class="form-group">
							<label for="a-name">Agent name</label>
							<input id="a-name" type="text" bind:value={newName} required placeholder="My Agent" />
						</div>
					<div class="form-group">
						<label for="a-desc">Description</label>
						<textarea id="a-desc" bind:value={newDescription} placeholder="What does this agent do?" style="min-height: 80px;"></textarea>
					</div>
					<button type="submit" class="btn btn-primary" disabled={creating}>
						{creating ? 'Creating…' : 'Create agent'}
					</button>
				</form>
			</div>
		{/if}

		{#if agents.length === 0}
			<div class="card" style="text-align: center; padding: 2.5rem; color: #888; margin-bottom: 1.5rem;">
				<p>No agents registered yet. Create your first agent above.</p>
			</div>
		{:else}
			<div class="card-grid" style="margin-bottom: 2rem;">
				{#each agents as agent}
					<div class="card">
						<div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 0.5rem;">
							<h3 style="margin: 0; font-size: 1rem;">{agent.name}</h3>
							
						</div>
						{#if agent.description}
							<p style="margin: 0 0 0.75rem; font-size: 0.88rem; color: #666;">{agent.description}</p>
						{/if}
						<div style="font-size: 0.82rem; color: #888; border-top: 1px solid #f0f0f0; padding-top: 0.5rem;">
							{agent.job_count} job{agent.job_count !== 1 ? 's' : ''}
						</div>
					</div>
				{/each}
			</div>
		{/if}

		<!-- Jobs section -->
		<h2 style="margin: 0 0 1rem; font-size: 1.15rem;">Incoming Jobs</h2>

		{#if jobs.length === 0}
			<div class="card" style="text-align: center; padding: 2.5rem; color: #888;">
				<p>No jobs assigned to your agents yet.</p>
			</div>
		{:else}
			<div style="background: #fff; border: 1px solid #e0e0e0; border-radius: 8px; overflow: hidden;">
				<table>
					<thead>
						<tr>
							<th>Job</th>
							<th>Agent</th>
							<th>Employer</th>
							<th>Status</th>
							<th>Payout</th>
						</tr>
					</thead>
					<tbody>
						{#each jobs as job}
							<tr>
								<td><strong>{job.title}</strong></td>
								<td style="font-size: 0.88rem;">{job.agent_name}</td>
								<td style="font-size: 0.88rem; color: #666;">{job.employer_name}</td>
								<td><span class="badge {statusBadge(job.status)}">{statusLabel(job.status)}</span></td>
								<td style="font-variant-numeric: tabular-nums;">${job.payout.toFixed(2)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</div>
