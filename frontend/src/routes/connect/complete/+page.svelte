<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';

	interface ConnectStatus {
		connected: boolean;
		charges_enabled: boolean;
		payouts_enabled: boolean;
		account_id?: string;
	}

	let status = $state<ConnectStatus | null>(null);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/auth/login');
			return;
		}
		try {
			const res = await apiFetch('/api/ui/stripe/connect/status');
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to load status' }));
				throw new Error(err.error || 'Failed to load status');
			}
			status = await res.json();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load connection status';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Stripe Account Connected — {SITE_NAME}</title>
</svelte:head>

<div class="container page">
	<div class="page-header">
		<h1>Stripe Account Connected</h1>
		<p>Your Stripe Connect account has been linked successfully.</p>
	</div>

	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Verifying connection status…</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
	{:else if status}
		<div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem;">
			<h2 style="font-size: 1.1rem; margin-bottom: 1rem;">Account Status</h2>
			<div style="display: flex; flex-direction: column; gap: 0.75rem;">
				<div style="display: flex; align-items: center; gap: 0.75rem;">
					<span style="font-weight: 600; min-width: 140px;">Charges enabled</span>
					{#if status.charges_enabled}
						<span style="color: #16a34a; font-weight: 500;">Yes</span>
					{:else}
						<span style="color: #dc2626; font-weight: 500;">No</span>
					{/if}
				</div>
				<div style="display: flex; align-items: center; gap: 0.75rem;">
					<span style="font-weight: 600; min-width: 140px;">Payouts enabled</span>
					{#if status.payouts_enabled}
						<span style="color: #16a34a; font-weight: 500;">Yes</span>
					{:else}
						<span style="color: #dc2626; font-weight: 500;">No</span>
					{/if}
				</div>
			</div>
			{#if !status.charges_enabled || !status.payouts_enabled}
				<p style="margin-top: 1rem; font-size: 0.88rem; color: #92400e; background: #fef3c7; border: 1px solid #fde68a; border-radius: 6px; padding: 0.6rem 0.9rem;">
					Some capabilities are not yet enabled. Stripe may require additional verification. Check your Stripe dashboard for details.
				</p>
			{/if}
		</div>
	{/if}

	<div style="margin-top: 1rem;">
		<a
			href={$auth?.role === 'EMPLOYER' ? '/dashboard/employer' : '/dashboard/manager'}
			class="btn btn-primary"
		>
			← Back to Dashboard
		</a>
	</div>
</div>
