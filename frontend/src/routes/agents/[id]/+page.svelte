<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { isAuthenticated, auth, apiFetch } from '$lib/stores/auth';
	import { SITE_NAME } from '$lib/config';

	interface Agent {
		id: string;
		manager_id: string;
		name: string;
		description: string;
		webhook_url: string;
		is_active: boolean;
		created_at: string;
		updated_at: string;
	}

	interface Job {
		id: string;
		agent_id: string;
		title: string;
		description: string;
		status: string;
		total_payout: number;
	}

	let agent: Agent | null = $state(null);
	let loading = $state(true);
	let error = $state('');

	// Hire panel state
	let hireOpen = $state(false);
	let unassignedJobs: Job[] = $state([]);
	let jobsLoading = $state(false);
	let assigningJobId = $state('');
	let assignError = $state('');
	let assignSuccess = $state('');

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

	async function openHirePanel() {
		hireOpen = true;
		assignError = '';
		assignSuccess = '';
		if ($isAuthenticated && $auth?.role === 'EMPLOYER') {
			jobsLoading = true;
			try {
				const res = await apiFetch('/api/ui/jobs');
				if (!res.ok) throw new Error('Failed to load jobs');
				const all: Job[] = await res.json();
				unassignedJobs = all.filter((j) => !j.agent_id || j.agent_id === '');
			} catch {
				unassignedJobs = [];
			} finally {
				jobsLoading = false;
			}
		}
	}

	function closeHirePanel() {
		hireOpen = false;
		assignError = '';
		assignSuccess = '';
	}

	async function sendOffer(jobId: string) {
		assigningJobId = jobId;
		assignError = '';
		assignSuccess = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/assign`, {
				method: 'POST',
				body: JSON.stringify({ agent_id: agentId })
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to send offer' }));
				throw new Error(err.error || 'Failed to send offer');
			}
			assignSuccess = 'Offer sent! The agent will review your job.';
			// Remove the job from unassigned list
			unassignedJobs = unassignedJobs.filter((j) => j.id !== jobId);
		} catch (e: unknown) {
			assignError = e instanceof Error ? e.message : 'Failed to send offer';
		} finally {
			assigningJobId = '';
		}
	}
</script>

<svelte:head>
	<title>{agent?.name ?? 'Agent'} — {SITE_NAME}</title>
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
					<button class="btn btn-primary" onclick={openHirePanel}>Hire this agent</button>
				{:else if !$isAuthenticated}
					<a href="/auth/signup" class="btn btn-primary">Sign up to hire</a>
				{/if}
			</div>

			{#if agent.description}
				<p style="color: #444; line-height: 1.6; margin-bottom: 1rem;">{agent.description}</p>
			{/if}

			<div style="display: flex; gap: 2rem; flex-wrap: wrap; padding-top: 1rem; border-top: 1px solid #f0f0f0; font-size: 0.9rem; color: #666;">
				<span>Manager</span>
			</div>
		</div>

		<!-- Hire panel -->
		{#if hireOpen}
			<div style="position: fixed; inset: 0; background: rgba(0,0,0,0.4); z-index: 100; display: flex; align-items: center; justify-content: center; padding: 1rem;">
				<div style="background: #fff; border-radius: 10px; padding: 2rem; max-width: 560px; width: 100%; max-height: 80vh; overflow-y: auto; box-shadow: 0 8px 40px rgba(0,0,0,0.18);">
					<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.25rem;">
						<h2 style="margin: 0; font-size: 1.2rem;">Hire {agent.name}</h2>
						<button onclick={closeHirePanel} style="background: none; border: none; font-size: 1.5rem; cursor: pointer; color: #888; line-height: 1;" title="Close">×</button>
					</div>

					{#if assignError}
						<div class="alert alert-error" style="margin-bottom: 1rem;">{assignError}</div>
					{/if}
					{#if assignSuccess}
						<div class="alert alert-success" style="margin-bottom: 1rem;">{assignSuccess}</div>
					{/if}

					{#if jobsLoading}
						<p style="color: #888; text-align: center; padding: 1rem 0;">Loading your job briefs...</p>
					{:else if unassignedJobs.length > 0}
						<p style="color: #555; margin-bottom: 1rem; font-size: 0.95rem;">
							Select one of your existing job briefs to send as an offer to this agent:
						</p>
						<div style="display: flex; flex-direction: column; gap: 0.75rem; margin-bottom: 1.25rem;">
							{#each unassignedJobs as job}
								<div style="border: 1px solid #e0e0e0; border-radius: 8px; padding: 0.9rem 1rem; display: flex; justify-content: space-between; align-items: center; gap: 1rem;">
									<div>
										<strong style="font-size: 0.95rem;">{job.title}</strong>
										{#if job.description}
											<div style="font-size: 0.82rem; color: #888; margin-top: 0.15rem;">
												{job.description.length > 60 ? job.description.slice(0, 60) + '…' : job.description}
											</div>
										{/if}
										<div style="font-size: 0.82rem; color: #666; margin-top: 0.2rem;">${job.total_payout.toFixed(2)}</div>
									</div>
									<button
										class="btn btn-primary"
										style="font-size: 0.85rem; padding: 0.35rem 0.9rem; white-space: nowrap;"
										disabled={assigningJobId === job.id}
										onclick={() => sendOffer(job.id)}
									>
										{assigningJobId === job.id ? 'Sending…' : 'Send Offer'}
									</button>
								</div>
							{/each}
						</div>
						<div style="border-top: 1px solid #f0f0f0; padding-top: 1rem; text-align: center;">
							<p style="color: #888; font-size: 0.9rem; margin-bottom: 0.75rem;">Or create a new job brief for this agent:</p>
							<a href="/jobs/new?return_to=/agents/{agentId}" class="btn btn-secondary">Enter a Job Brief</a>
						</div>
					{:else}
						<p style="color: #555; margin-bottom: 1.25rem; font-size: 0.95rem;">
							You don't have any unassigned job briefs yet. Create one now to send an offer to this agent.
						</p>
						<div style="text-align: center;">
							<a href="/jobs/new?return_to=/agents/{agentId}" class="btn btn-primary">Enter a Job Brief</a>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
</div>
