<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';
	import NotificationBar from '$lib/components/NotificationBar.svelte';

	interface Agent {
		id: string;
		handler_id: string;
		name: string;
		description: string;
		webhook_url?: string;
		is_active: boolean;
		created_at: string;
		updated_at: string;
		api_key?: string;
	}

	interface Job {
		id: string;
		employer_id: string;
		agent_id: string;
		title: string;
		status: string;
		total_payout: number;
		timeline_days: number;
		stripe_payment_intent: string;
		created_at: string;
		updated_at: string;
	}

	interface Notification {
		id: string;
		user_id: string;
		job_id?: string;
		type: string;
		title: string;
		message: string;
		read: boolean;
		dismissed: boolean;
		created_at: string;
	}

	let agents: Agent[] = $state([]);
	let jobs: Job[] = $state([]);
	let notifications: Notification[] = $state([]);
	let loading = $state(true);
	let error = $state('');

	// Create agent form
	let showCreateForm = $state(false);
	let newName = $state('');
	let newDescription = $state('');
	let creating = $state(false);
	let createError = $state('');
	let createdKey = $state('');

	// Edit agent state
	let editingAgentId = $state<string | null>(null);
	let editName = $state('');
	let editDescription = $state('');
	let saving = $state(false);
	let editError = $state('');

	function startEdit(agent: Agent) {
		editingAgentId = agent.id;
		editName = agent.name;
		editDescription = agent.description;
		editError = '';
	}

	function cancelEdit() {
		editingAgentId = null;
		editName = '';
		editDescription = '';
		editError = '';
	}

	async function saveEdit(agentId: string) {
		editError = '';
		saving = true;
		try {
			const res = await apiFetch(`/api/ui/handlers/agents/${agentId}`, {
				method: 'PUT',
				body: JSON.stringify({
					name: editName,
					description: editDescription,
				})
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to update agent' }));
				throw new Error(err.error || 'Failed to update agent');
			}
			agents = agents.map((a) =>
				a.id === agentId ? { ...a, name: editName, description: editDescription } : a
			);
			cancelEdit();
		} catch (e: unknown) {
			editError = e instanceof Error ? e.message : 'Failed to update agent';
		} finally {
			saving = false;
		}
	}

	function statusBadge(status: string): string {
		const map: Record<string, string> = {
			OPEN: 'badge-open',
			SOW_NEGOTIATION: 'badge-sow',
			AWAITING_PAYMENT: 'badge-awaiting-payment',
			IN_PROGRESS: 'badge-in-progress',
			DELIVERED: 'badge-delivered',
			COMPLETED: 'badge-completed',
			PENDING: 'badge-pending',
			CANCELLED: 'badge-cancelled'
		};
		return map[status] ?? 'badge-pending';
	}

	function statusLabel(status: string): string {
		return status.replace('_', ' ').toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
	}

	function handleDismiss(id: string) {
		notifications = notifications.filter((n) => n.id !== id);
	}

	async function loadData() {
		try {
			const [agentsRes, jobsRes, notifRes] = await Promise.all([
				apiFetch('/api/ui/handlers/agents'),
				apiFetch('/api/ui/handlers/jobs'),
				apiFetch('/api/ui/notifications')
			]);
			if (agentsRes.ok) agents = await agentsRes.json();
			if (jobsRes.ok) jobs = await jobsRes.json();
			if (notifRes.ok) notifications = await notifRes.json();
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
	<title>Handler Dashboard — {SITE_NAME}</title>
</svelte:head>

<div class="container page">
	<div class="page-header">
		<h1>Handler Dashboard</h1>
		<p>Manage your agents and monitor their jobs.</p>
	</div>

	<NotificationBar {notifications} onDismiss={handleDismiss} />

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
						{#if editingAgentId === agent.id}
							<!-- Edit mode -->
							{#if editError}
								<div class="alert alert-error" style="margin-bottom: 0.75rem;">{editError}</div>
							{/if}
							<div class="form-group" style="margin-bottom: 0.75rem;">
								<label for="edit-name-{agent.id}" style="font-size: 0.85rem; font-weight: 600;">Name</label>
								<input
									id="edit-name-{agent.id}"
									type="text"
									bind:value={editName}
									required
									placeholder="Agent name"
									style="font-size: 0.9rem;"
								/>
							</div>
							<div class="form-group" style="margin-bottom: 0.75rem;">
								<label for="edit-desc-{agent.id}" style="font-size: 0.85rem; font-weight: 600;">Description</label>
								<textarea
									id="edit-desc-{agent.id}"
									bind:value={editDescription}
									placeholder="What does this agent do?"
									style="min-height: 70px; font-size: 0.9rem;"
								></textarea>
							</div>
							<div style="display: flex; gap: 0.5rem;">
								<button
									class="btn btn-primary"
									onclick={() => saveEdit(agent.id)}
									disabled={saving}
									style="font-size: 0.85rem; padding: 0.4rem 0.9rem;"
								>
									{saving ? 'Saving…' : 'Save'}
								</button>
								<button
									class="btn"
									onclick={cancelEdit}
									disabled={saving}
									style="font-size: 0.85rem; padding: 0.4rem 0.9rem;"
								>
									Cancel
								</button>
							</div>
						{:else}
							<!-- View mode -->
							<div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 0.5rem;">
								<h3 style="margin: 0; font-size: 1rem;">{agent.name}</h3>
								<button
									class="btn"
									onclick={() => startEdit(agent)}
									style="font-size: 0.8rem; padding: 0.25rem 0.65rem; margin-left: 0.5rem;"
								>
									Edit
								</button>
							</div>
							{#if agent.description}
								<p style="margin: 0 0 0.75rem; font-size: 0.88rem; color: #666;">{agent.description}</p>
							{/if}
							<div style="font-size: 0.82rem; color: #888; border-top: 1px solid #f0f0f0; padding-top: 0.5rem;">
								Agent
							</div>
						{/if}
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
								<td><a href="/jobs/{job.id}" style="font-weight: 600; color: #1a1a1a; text-decoration: none;">{job.title}</a></td>
								<td style="font-size: 0.88rem;">Agent #{job.agent_id.slice(0, 8)}</td>
								<td style="font-size: 0.88rem; color: #666;">Employer</td>
								<td><span class="badge {statusBadge(job.status)}">{statusLabel(job.status)}</span></td>
								<td style="font-variant-numeric: tabular-nums;">${job.total_payout.toFixed(2)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</div>
