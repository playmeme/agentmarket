<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { marked } from 'marked';
	import DOMPurify from 'dompurify';
	import SOW from '$lib/components/SOW.svelte';
	import DeliverySection from '$lib/components/DeliverySection.svelte';

	function renderMarkdown(text: string): string {
		const raw = marked.parse(text, { async: false }) as string;
		if (browser) {
			return DOMPurify.sanitize(raw);
		}
		// SSR: return parsed HTML; DOMPurify sanitizes after client hydration
		return raw;
	}

	interface SOWData {
		id?: string;
		job_id: string;
		scope: string;
		deliverables: string;
		price_cents: number;
		timeline_days: number;
		employer_accepted: boolean;
		handler_accepted: boolean;
		employer_accepted_at?: string;
		handler_accepted_at?: string;
	}

	interface DeliveryData {
		delivery_notes?: string;
		delivery_url?: string;
	}

	interface Criterion {
		id: string;
		milestone_id: string;
		description: string;
		is_verified: boolean;
		created_at: string;
	}

	interface Milestone {
		id: string;
		job_id: string;
		title: string;
		amount: number;
		criteria: Criterion[];
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
		stripe_payment_intent?: string;
		created_at: string;
		updated_at: string;
		milestones: Milestone[];
		sow: SOWData | null;
		delivery: DeliveryData | null;
	}

	const jobId = $derived($page.params.job_id ?? '');

	let job: Job | null = $state(null);
	let loading = $state(true);
	let error = $state('');
	let checkoutLoading = $state(false);
	let checkoutError = $state('');

	const isEmployer = $derived($auth?.role === 'EMPLOYER');
	const isHandler = $derived($auth?.role === 'AGENT_HANDLER');

	function statusBadgeClass(status: string): string {
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
		return status.replace(/_/g, ' ').toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
	}

	async function loadJob() {
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}`);
			if (!res.ok) {
				if (res.status === 404) throw new Error('Job not found');
				throw new Error('Failed to load job');
			}
			job = await res.json();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load job';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/auth/login');
			return;
		}
		await loadJob();
	});

	async function handleCheckout() {
		checkoutLoading = true;
		checkoutError = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/checkout`, {
				method: 'POST'
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to initiate checkout' }));
				throw new Error(err.error || 'Failed to initiate checkout');
			}
			const data = await res.json();
			if (data.url) {
				window.location.href = data.url;
			} else {
				throw new Error('No checkout URL returned');
			}
		} catch (e: unknown) {
			checkoutError = e instanceof Error ? e.message : 'Failed to initiate checkout';
		} finally {
			checkoutLoading = false;
		}
	}

	function handleSowUpdate() {
		loadJob();
	}

	function handleDeliveryUpdate() {
		loadJob();
	}
</script>

<svelte:head>
	<title>{job?.title ?? 'Job'} — AgentMarket</title>
</svelte:head>

<style>
	.badge-sow { background: #ede9fe; color: #5b21b6; }
	.badge-awaiting-payment { background: #fef3c7; color: #92400e; }
	.badge-delivered { background: #dbeafe; color: #1e40af; }
	.badge-cancelled { background: #fee2e2; color: #991b1b; }
</style>

<div class="container page">
	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading job...</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
		<a href={isEmployer ? '/dashboard/employer' : '/dashboard/handler'} class="btn btn-secondary" style="margin-top: 1rem;">
			Back to Dashboard
		</a>
	{:else if job}
		<div style="margin-bottom: 1rem;">
			<a
				href={isEmployer ? '/dashboard/employer' : '/dashboard/handler'}
				style="color: #888; font-size: 0.9rem;"
			>← Dashboard</a>
		</div>

		<!-- Job header -->
		<div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
			<div>
				<div style="display: flex; align-items: center; gap: 0.75rem; margin-bottom: 0.4rem; flex-wrap: wrap;">
					<h1 style="margin: 0; font-size: 1.75rem;">{job.title}</h1>
					<span class="badge {statusBadgeClass(job.status)}">{statusLabel(job.status)}</span>
				</div>
				{#if job.description}
					<p style="margin: 0; color: #666; max-width: 640px;">{job.description}</p>
				{/if}
			</div>
		</div>

		<!-- Job meta -->
		<div class="card" style="margin-bottom: 1.5rem;">
			<div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 1rem;">
				<div>
					<p style="font-size: 0.8rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Total Payout</p>
					<p style="margin: 0; font-size: 1.1rem; font-weight: 600;">${job.total_payout.toFixed(2)}</p>
				</div>
				{#if job.timeline_days}
					<div>
						<p style="font-size: 0.8rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Timeline</p>
						<p style="margin: 0;">{job.timeline_days} day{job.timeline_days !== 1 ? 's' : ''}</p>
					</div>
				{/if}
				<div>
					<p style="font-size: 0.8rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Created</p>
					<p style="margin: 0; font-size: 0.9rem; color: #666;">{new Date(job.created_at).toLocaleDateString()}</p>
				</div>
				{#if job.milestones?.length}
					<div>
						<p style="font-size: 0.8rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Milestones</p>
						<p style="margin: 0;">{job.milestones.filter(m => m.status === 'COMPLETED').length}/{job.milestones.length} done</p>
					</div>
				{/if}
			</div>
		</div>

		<!-- Awaiting payment — Pay Now button (employer only) -->
		{#if job.status === 'AWAITING_PAYMENT' && isEmployer}
			<div class="card" style="margin-bottom: 1.5rem; border-color: #fbbf24; background: #fffbeb;">
				<div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem;">
					<div>
						<h3 style="margin: 0 0 0.25rem; font-size: 1rem;">Payment Required</h3>
						<p style="margin: 0; color: #666; font-size: 0.9rem;">Both parties have agreed to the SoW. Complete payment to start the job.</p>
					</div>
					<div>
						{#if checkoutError}
							<div class="alert alert-error" style="margin-bottom: 0.5rem;">{checkoutError}</div>
						{/if}
						<button class="btn btn-primary" onclick={handleCheckout} disabled={checkoutLoading}>
							{checkoutLoading ? 'Redirecting…' : 'Pay Now'}
						</button>
					</div>
				</div>
			</div>
		{/if}

		<!-- SoW component — shown during negotiation and beyond -->
		{#if ['SOW_NEGOTIATION', 'AWAITING_PAYMENT', 'IN_PROGRESS', 'DELIVERED', 'COMPLETED'].includes(job.status)}
			<SOW
				{jobId}
				bind:sow={job.sow}
				jobStatus={job.status}
				onUpdate={handleSowUpdate}
			/>
		{/if}

		<!-- Delivery section — shown from IN_PROGRESS onward -->
		{#if ['IN_PROGRESS', 'DELIVERED', 'COMPLETED'].includes(job.status)}
			<DeliverySection
				{jobId}
				jobStatus={job.status}
				delivery={job.delivery}
				onUpdate={handleDeliveryUpdate}
			/>
		{/if}

		<!-- Milestones -->
		{#if job.milestones?.length}
			<div class="card" style="margin-bottom: 1.5rem;">
				<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">Milestones</h2>
				<div style="display: flex; flex-direction: column; gap: 0.75rem;">
					{#each job.milestones as milestone}
						<div class="milestone-row">
							<div class="milestone-header">
								<strong style="font-size: 0.95rem;">{milestone.title}</strong>
								<div style="display: flex; align-items: center; gap: 0.75rem;">
									<span style="font-size: 0.9rem; color: #555;">${milestone.amount.toFixed(2)}</span>
									<span class="badge {milestone.status === 'COMPLETED' ? 'badge-completed' : 'badge-pending'}">
										{statusLabel(milestone.status)}
									</span>
								</div>
							</div>
							{#if milestone.criteria?.length}
								<ul style="margin: 0.25rem 0 0; padding-left: 1.25rem; font-size: 0.88rem; color: #666;">
									{#each milestone.criteria as criterion}
										<li style="margin-bottom: 0.2rem;">{criterion.description}</li>
									{/each}
								</ul>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}
	{/if}
</div>
