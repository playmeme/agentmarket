<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';

	interface Transaction {
		id: string;
		job_id: string;
		amount_cents: number;
		currency: string;
		type: string;
		status: string;
		stripe_payment_intent?: string;
		created_at: string;
		updated_at: string;
	}

	let transactions: Transaction[] = $state([]);
	let loading = $state(true);
	let error = $state('');

	function formatCents(cents: number, currency: string = 'USD'): string {
		return new Intl.NumberFormat('en-US', {
			style: 'currency',
			currency: currency.toUpperCase() || 'USD'
		}).format(cents / 100);
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

	function txTypeBadgeClass(type: string): string {
		const map: Record<string, string> = {
			PAYMENT: 'badge-in-progress',
			PAYOUT: 'badge-open',
			REFUND: 'badge-cancelled',
			FEE: 'badge-pending'
		};
		return map[type?.toUpperCase()] ?? 'badge-pending';
	}

	function txStatusBadgeClass(status: string): string {
		const map: Record<string, string> = {
			SUCCEEDED: 'badge-completed',
			PENDING: 'badge-pending',
			FAILED: 'badge-cancelled',
			REFUNDED: 'badge-in-progress'
		};
		return map[status?.toUpperCase()] ?? 'badge-pending';
	}

	function txLabel(val: string): string {
		return val.replace(/_/g, ' ').toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
	}

	onMount(async () => {
		if (!$isAuthenticated) {
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
	.badge-cancelled { background: #fee2e2; color: #991b1b; }
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
	{:else if transactions.length === 0}
		<div class="card" style="text-align: center; padding: 3rem; color: #888;">
			<p>No transactions recorded.</p>
		</div>
	{:else}
		<div style="background: #fff; border: 1px solid #e0e0e0; border-radius: 8px; overflow: hidden;">
			<table>
				<thead>
					<tr>
						<th>Date</th>
						<th>Job</th>
						<th>Type</th>
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
										Job #{tx.job_id.slice(0, 8)}
									</a>
								{:else}
									<span style="color: #888; font-size: 0.9rem;">—</span>
								{/if}
							</td>
							<td>
								<span class="badge {txTypeBadgeClass(tx.type)}">{txLabel(tx.type)}</span>
							</td>
							<td style="font-variant-numeric: tabular-nums; font-weight: 500;">
								{formatCents(tx.amount_cents, tx.currency)}
							</td>
							<td>
								<span class="badge {txStatusBadgeClass(tx.status)}">{txLabel(tx.status)}</span>
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
