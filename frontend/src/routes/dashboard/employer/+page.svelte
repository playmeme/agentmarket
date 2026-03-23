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
			SOW_NEGOTIATION: 'badge-sow',
			AWAITING_PAYMENT: 'badge-awaiting-payment',
			IN_PROGRESS: 'badge-in-progress',
			DELIVERED: 'badge-delivered',
			COMPLETED: 'badge-completed',
			PENDING: 'badge-pending',
			PENDING_ACCEPTANCE: 'badge-pending',
			CANCELLED: 'badge-cancelled'
		};
		return map[status] ?? 'badge-pending';
	}

	function statusLabel(status: string): string {
		return status.replace(/_/g, ' ').toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
	}

	function isUnassigned(job: Job): boolean {
		return !job.agent_id || job.agent_id === '';
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
			<p>Manage your job briefs and track agent progress.</p>
		</div>
		<a href="/jobs/new" class="btn btn-primary">Enter a Job Brief</a>
	</div>

	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading jobs...</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
	{:else if jobs.length === 0}
		<div class="card" style="text-align: center; padding: 3rem; color: #888;">
			<p>No job briefs yet.</p>
			<p style="font-size: 0.9rem; margin-top: 0.5rem;">Create a job brief first, then assign it to an agent.</p>
			<a href="/jobs/new" class="btn btn-primary" style="margin-top: 0.75rem;">Enter a Job Brief</a>
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
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each jobs as job}
						<tr>
							<td>
								<a href="/jobs/{job.id}" style="font-weight: 600; color: #1a1a1a; text-decoration: none;">{job.title}</a>
								{#if job.description}
									<div style="font-size: 0.82rem; color: #888; margin-top: 0.15rem;">
										{job.description.length > 80 ? job.description.slice(0, 80) + '…' : job.description}
									</div>
								{/if}
							</td>
							<td>
								{#if isUnassigned(job)}
									<span style="color: #aaa; font-style: italic; font-size: 0.9rem;">Not assigned</span>
								{:else}
									<a href="/agents/{job.agent_id}">Agent #{job.agent_id.slice(0, 8)}</a>
								{/if}
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
							<td>
								{#if isUnassigned(job)}
									<a href="/" class="btn btn-secondary" style="font-size: 0.8rem; padding: 0.25rem 0.75rem; white-space: nowrap;">
										Submit to Agent
									</a>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
