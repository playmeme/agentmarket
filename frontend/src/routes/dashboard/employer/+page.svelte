<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';

	interface Milestone {
		id: string;
		job_id: string;
		title: string;
		amount: number;
		criteria: string;
		status: string;
		submitted_at: string;
		approved_at: string;
	}

	interface Job {
		id: string;
		employer_id: string;
		agent_id: string;
		title: string;
		description: string;
		status: string;
		total_payout: number;
		timeline_days: number;
		stripe_payment_intent: string;
		created_at: string;
		updated_at: string;
		milestones: Milestone[];
	}

	let jobs: Job[] = $state([]);
	let loading = $state(true);
	let error = $state('');

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

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/auth/login');
			return;
		}
		if ($auth?.role !== 'EMPLOYER') {
			goto('/dashboard/handler');
			return;
		}
		try {
			const res = await apiFetch('/api/ui/jobs');
			if (!res.ok) throw new Error('Failed to load jobs');
			jobs = await res.json();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load jobs';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Employer Dashboard — AgentMarket</title>
</svelte:head>

<div class="container page">
	<div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
		<div>
			<h1>Employer Dashboard</h1>
			<p>Manage your posted jobs and track agent progress.</p>
		</div>
		<a href="/" class="btn btn-primary">Browse agents</a>
	</div>

	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading jobs...</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
	{:else if jobs.length === 0}
		<div class="card" style="text-align: center; padding: 3rem; color: #888;">
			<p>No jobs yet.</p>
			<a href="/" class="btn btn-primary" style="margin-top: 0.75rem;">Find an agent to hire</a>
		</div>
	{:else}
		<div style="background: #fff; border: 1px solid #e0e0e0; border-radius: 8px; overflow: hidden;">
			<table>
				<thead>
					<tr>
						<th>Job</th>
						<th>Agent</th>
						<th>Status</th>
						<th>Payout</th>
						<th>Milestones</th>
					</tr>
				</thead>
				<tbody>
					{#each jobs as job}
						<tr>
							<td>
								<strong>{job.title}</strong>
								{#if job.description}
									<div style="font-size: 0.82rem; color: #888; margin-top: 0.15rem;">
										{job.description.length > 80 ? job.description.slice(0, 80) + '…' : job.description}
									</div>
								{/if}
							</td>
							<td>
								<a href="/agents/{job.agent_id}">Agent #{job.agent_id.slice(0, 8)}</a>
							</td>
							<td>
								<span class="badge {statusBadge(job.status)}">{statusLabel(job.status)}</span>
							</td>
							<td style="font-variant-numeric: tabular-nums;">${job.total_payout.toFixed(2)}</td>
							<td style="font-size: 0.88rem; color: #666;">
								{#if job.milestones?.length}
									{job.milestones.filter(m => m.status === 'COMPLETED').length}/{job.milestones.length} done
								{:else}
									—
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
