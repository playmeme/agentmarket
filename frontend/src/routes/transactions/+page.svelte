<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';

	// Shape returned by GET /api/ui/transactions (TransactionSummary in backend)
	interface TransactionSummary {
		job_id: string;
		title: string;
		status: string;
		total_payout: number;
		stripe_payment_intent?: string;
		created_at: string;
		updated_at: string;
	}

	let transactions: TransactionSummary[] = $state([]);
	let loading = $state(true);
	let error = $state('');

	// Static sample data shown when no real transactions exist (Issue #30)
	const SAMPLE_TRANSACTIONS: TransactionSummary[] = [
		{
			job_id: '00000000-0000-0000-0000-000000000001',
			title: 'Build a REST API endpoint for user authentication',
			status: 'COMPLETED',
			total_payout: 150.00,
			stripe_payment_intent: 'pi_sample_abc123',
			created_at: '2026-03-20T14:32:00Z',
			updated_at: '2026-03-21T09:15:00Z'
		},
		{
			job_id: '00000000-0000-0000-0000-000000000002',
			title: 'Write unit tests for the payment module',
			status: 'IN_PROGRESS',
			total_payout: 85.00,
			stripe_payment_intent: undefined,
			created_at: '2026-03-23T10:00:00Z',
			updated_at: '2026-03-23T10:00:00Z'
		}
	];

	let showMockup = $derived(transactions.length === 0 && !loading && !error);

	function formatCents(cents: number): string {
		return new Intl.NumberFormat('en-US', {
			style: 'currency',
			currency: 'USD'
		}).format(cents / 100);
	}

	function formatDollars(dollars: number): string {
		return new Intl.NumberFormat('en-US', {
			style: 'currency',
			currency: 'USD'
		}).format(dollars);
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function statusBadgeClass(status: string): string {
		const map: Record<string, string> = {
			COMPLETED: 'badge-completed',
			OPEN: 'badge-open',
			IN_PROGRESS: 'badge-in-progress',
			CANCELLED: 'badge-cancelled',
			PENDING: 'badge-pending',
			AWAITING_SOW: 'badge-awaiting',
			SOW_REVIEW: 'badge-sow',
			DELIVERED: 'badge-delivered'
		};
		return map[status?.toUpperCase()] ?? 'badge-pending';
	}

	function statusLabel(val: string): string {
		return val.replace(/_/g, ' ').toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
	}

	onMount(async () => {
		if (!$isAuthenticated) {
			loading = false;
			goto('/auth/login');
			return;
		}
		try {
			const res = await apiFetch('/api/ui/transactions');
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to load transactions' }));
				throw new Error(err.error || 'Failed to load transactions');
			}
			transactions = await res.json();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load transactions';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Transactions — {SITE_NAME}</title>
</svelte:head>

<style>
	.mockup-banner {
		background: #fef9c3;
		border: 1px solid #fde047;
		border-radius: 6px;
		padding: 0.6rem 1rem;
		margin-bottom: 1.25rem;
		font-size: 0.85rem;
		color: #713f12;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.mockup-banner strong {
		font-weight: 600;
	}
	.mockup-row td {
		opacity: 0.65;
	}
</style>

<div class="container page">
	<div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
		<div>
			<h1>Transactions</h1>
			<p>Your payment history and financial activity.</p>
		</div>
		<a
			href={$auth?.role === 'EMPLOYER' ? '/dashboard/employer' : '/dashboard/handler'}
			class="btn btn-secondary"
		>
			← Dashboard
		</a>
	</div>

	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading transactions…</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
	{:else if showMockup}
		<div class="mockup-banner">
			<span>👁</span>
			<span><strong>Sample preview</strong> — no real transactions yet. This shows what the layout will look like once jobs are paid.</span>
		</div>
		<div style="background: #fff; border: 1px solid #e0e0e0; border-radius: 8px; overflow: hidden;">
			<table>
				<thead>
					<tr>
						<th>Date</th>
						<th>Job</th>
						<th>Amount</th>
						<th>Status</th>
					</tr>
				</thead>
				<tbody>
					{#each SAMPLE_TRANSACTIONS as tx}
						<tr class="mockup-row">
							<td style="font-size: 0.88rem; color: #666; white-space: nowrap;">
								{formatDate(tx.created_at)}
							</td>
							<td style="font-size: 0.9rem; color: #444;">
								{tx.title}
							</td>
							<td style="font-variant-numeric: tabular-nums; font-weight: 500;">
								{formatDollars(tx.total_payout)}
							</td>
							<td>
								<span class="badge {statusBadgeClass(tx.status)}">{statusLabel(tx.status)}</span>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<p style="margin-top: 1rem; font-size: 0.85rem; color: #aaa;">
			No transactions recorded.
		</p>
	{:else}
		<div style="background: #fff; border: 1px solid #e0e0e0; border-radius: 8px; overflow: hidden;">
			<table>
				<thead>
					<tr>
						<th>Date</th>
						<th>Job</th>
						<th>Amount</th>
						<th>Status</th>
					</tr>
				</thead>
				<tbody>
					{#each transactions as tx}
						<tr>
							<td style="font-size: 0.88rem; color: #666; white-space: nowrap;">
								{formatDate(tx.created_at)}
							</td>
							<td>
								{#if tx.job_id}
									<a href="/jobs/{tx.job_id}" style="font-size: 0.9rem;">
										{tx.title || 'Job #' + tx.job_id.slice(0, 8)}
									</a>
								{:else}
									<span style="color: #888; font-size: 0.9rem;">—</span>
								{/if}
							</td>
							<td style="font-variant-numeric: tabular-nums; font-weight: 500;">
								{formatDollars(tx.total_payout)}
							</td>
							<td>
								<span class="badge {statusBadgeClass(tx.status)}">{statusLabel(tx.status)}</span>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<p style="margin-top: 1rem; font-size: 0.85rem; color: #888;">
			{transactions.length} transaction{transactions.length !== 1 ? 's' : ''} total
		</p>
	{/if}
</div>
