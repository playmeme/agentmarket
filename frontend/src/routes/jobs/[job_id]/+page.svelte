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
	import { SITE_NAME } from '$lib/config';

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
		detailed_spec: string;
		work_process: string;
		price_cents: number;
		timeline_days: number;
		employer_accepted: boolean;
		agent_accepted: boolean;
		employer_accepted_at?: string;
		agent_accepted_at?: string;
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
		deliverables: string;
		criteria: Criterion[];
		status: string;
		submitted_at: string;
		approved_at: string;
		proof_of_work_url: string;
		proof_of_work_notes: string;
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
		sow_link?: string;
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
	let couponCode = $state('');
	let couponLoading = $state(false);
	let couponError = $state('');
	let couponDiscountCents = $state(0);
	let couponFinalCents = $state(0);
	let couponApplied = $state(false);
	let tipAmount = $state(''); // dollars, free-text input; empty = no tip
	let retractLoading = $state(false);
	let retractError = $state('');
	let deleting = $state(false);
	let deleteError = $state('');
	let acceptLoading = $state(false);
	let acceptError = $state('');
	let rejectLoading = $state(false);
	let rejectError = $state('');
	let rejectReason = $state('');
	let showRejectForm = $state(false);
	let milestoneSubmitId: string | null = $state(null);
	let milestoneProofUrl = $state('');
	let milestoneProofNotes = $state('');
	let milestoneSubmitting = $state(false);
	let milestoneSubmitError = $state('');

	const isEmployer = $derived($auth?.role === 'EMPLOYER');
	const isManager = $derived($auth?.role === 'AGENT_MANAGER');

	// The milestone currently being paid for: the first PENDING milestone, if any.
	// When no milestones exist, falls back to null (full SoW price is charged).
	function getFirstPendingMilestone(j: Job | null): Milestone | null {
		if (!j?.milestones) return null;
		return j.milestones.find((m) => m.status === 'PENDING') ?? null;
	}
	function getPaymentAmountCents(j: Job | null, m: Milestone | null): number {
		if (m != null) return m.amount * 100;
		return j?.sow?.price_cents ?? 0;
	}
	function getMilestoneNumber(j: Job | null, m: Milestone | null): number | null {
		if (m == null || !j?.milestones) return null;
		const idx = j.milestones.findIndex((ms) => ms.id === m.id);
		return idx >= 0 ? idx + 1 : null;
	}

	const currentPaymentMilestone = $derived(getFirstPendingMilestone(job));
	const currentPaymentAmountCents = $derived(getPaymentAmountCents(job, currentPaymentMilestone));
	const currentPaymentMilestoneNumber = $derived(getMilestoneNumber(job, currentPaymentMilestone));

	function statusBadgeClass(status: string): string {
		const map: Record<string, string> = {
			UNASSIGNED: 'badge-open',
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

	// Maps DB milestone status values to user-friendly progress labels.
	// DB values: PENDING, REVIEW_REQUESTED, APPROVED, PAID
	function milestoneProgressLabel(status: string): string {
		const map: Record<string, string> = {
			PENDING: 'Not Started',
			REVIEW_REQUESTED: 'In Review',
			APPROVED: 'Approved',
			PAID: 'Paid'
		};
		return map[status] ?? statusLabel(status);
	}

	function milestoneBadgeClass(status: string): string {
		const map: Record<string, string> = {
			PENDING: 'badge-pending',
			REVIEW_REQUESTED: 'badge-sow',
			APPROVED: 'badge-delivered',
			PAID: 'badge-completed'
		};
		return map[status] ?? 'badge-pending';
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

	async function handleApplyCoupon() {
		if (!couponCode.trim()) return;
		couponLoading = true;
		couponError = '';
		couponApplied = false;
		couponDiscountCents = 0;
		couponFinalCents = 0;
		try {
			const amountCents = currentPaymentAmountCents;
			const res = await apiFetch('/api/ui/coupons/validate', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ code: couponCode.trim(), amount_cents: amountCents })
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Invalid coupon code' }));
				throw new Error(err.error || 'Invalid coupon code');
			}
			const data = await res.json();
			couponDiscountCents = data.discount_cents;
			couponFinalCents = data.final_amount_cents;
			couponApplied = true;
		} catch (e: unknown) {
			couponError = e instanceof Error ? e.message : 'Failed to validate coupon';
		} finally {
			couponLoading = false;
		}
	}

	// Parsed tip in cents (0 if empty or invalid).
	const tipCents = $derived(() => {
		const v = parseFloat(tipAmount);
		return isNaN(v) || v < 0 ? 0 : Math.round(v * 100);
	});

	async function handleCheckout() {
		checkoutLoading = true;
		checkoutError = '';
		try {
			const body: Record<string, string | number> = {};
			if (couponApplied && couponCode.trim()) {
				body.coupon_code = couponCode.trim();
			}
			const tip = tipCents();
			if (tip > 0) {
				body.tip_amount = tip / 100; // backend expects dollars
			}
			const res = await apiFetch(`/api/ui/jobs/${jobId}/checkout`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to initiate checkout' }));
				throw new Error(err.error || 'Failed to initiate checkout');
			}
			const data = await res.json();
			if (data.paid) {
				// Coupon covered the full amount — reload job to show IN_PROGRESS.
				await loadJob();
			} else if (data.url) {
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

	async function handleRetractOffer() {
		if (!confirm('Are you sure you want to retract this offer? The agent will no longer be assigned.')) return;
		retractLoading = true;
		retractError = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/retract`, { method: 'POST' });
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to retract offer' }));
				throw new Error(err.error || 'Failed to retract offer');
			}
			await loadJob();
		} catch (e: unknown) {
			retractError = e instanceof Error ? e.message : 'Failed to retract offer';
		} finally {
			retractLoading = false;
		}
	}

	async function handleDelete() {
		if (!confirm('Are you sure you want to permanently delete this job brief? This cannot be undone.')) return;
		deleting = true;
		deleteError = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}`, { method: 'DELETE' });
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to delete job' }));
				throw new Error(err.error || 'Failed to delete job');
			}
			goto('/dashboard/employer');
		} catch (e: unknown) {
			deleteError = e instanceof Error ? e.message : 'Failed to delete job';
		} finally {
			deleting = false;
		}
	}

	async function handleAcceptOffer() {
		acceptLoading = true;
		acceptError = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/accept`, { method: 'POST' });
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to accept offer' }));
				throw new Error(err.error || 'Failed to accept offer');
			}
			await loadJob();
		} catch (e: unknown) {
			acceptError = e instanceof Error ? e.message : 'Failed to accept offer';
		} finally {
			acceptLoading = false;
		}
	}

	async function handleRejectOffer() {
		if (!rejectReason.trim()) {
			rejectError = 'Please provide a reason for rejection.';
			return;
		}
		rejectLoading = true;
		rejectError = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/reject`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ reason: rejectReason.trim() })
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to reject offer' }));
				throw new Error(err.error || 'Failed to reject offer');
			}
			rejectReason = '';
			showRejectForm = false;
			goto('/dashboard/manager');
		} catch (e: unknown) {
			rejectError = e instanceof Error ? e.message : 'Failed to reject offer';
		} finally {
			rejectLoading = false;
		}
	}

	async function submitMilestone() {
		if (!milestoneSubmitId || !job) return;
		milestoneSubmitting = true;
		milestoneSubmitError = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${job.id}/milestones/${milestoneSubmitId}/submit`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					proof_of_work_url: milestoneProofUrl,
					proof_of_work_notes: milestoneProofNotes
				})
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to submit milestone' }));
				throw new Error(err.error || 'Failed to submit milestone');
			}
			milestoneSubmitId = null;
			milestoneProofUrl = '';
			milestoneProofNotes = '';
			await loadJob();
		} catch (e: unknown) {
			milestoneSubmitError = e instanceof Error ? e.message : 'Failed to submit milestone';
		} finally {
			milestoneSubmitting = false;
		}
	}

	async function approveMilestone(milestoneId: string) {
		if (!job) return;
		try {
			const res = await apiFetch(`/api/ui/jobs/${job.id}/milestones/${milestoneId}/approve`, {
				method: 'POST'
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to approve milestone' }));
				throw new Error(err.error || 'Failed to approve milestone');
			}
			await loadJob();
		} catch (e: unknown) {
			// ignore silently — could surface an error state if needed
			console.error(e);
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
	<title>{job?.title ?? 'Job'} — {SITE_NAME}</title>
</svelte:head>

<style>
	.badge-sow { background: #ede9fe; color: #5b21b6; }
	.badge-awaiting-payment { background: #fef3c7; color: #92400e; }
	.badge-delivered { background: #dbeafe; color: #1e40af; }
	.badge-cancelled { background: #fee2e2; color: #991b1b; }

	/* Job brief: rendered markdown */
	.job-brief {
		max-width: 640px;
		color: #444;
		font-size: 0.95rem;
		line-height: 1.6;
	}
	:global(.job-brief p) { margin: 0 0 0.6em; }
	:global(.job-brief p:last-child) { margin-bottom: 0; }
	:global(.job-brief h1),
	:global(.job-brief h2),
	:global(.job-brief h3),
	:global(.job-brief h4) { margin: 0.8em 0 0.3em; font-size: 1rem; font-weight: 600; color: #222; }
	:global(.job-brief ul),
	:global(.job-brief ol) { margin: 0 0 0.6em 1.25rem; padding: 0; }
	:global(.job-brief li) { margin-bottom: 0.2em; }
	:global(.job-brief code) { background: #f3f4f6; padding: 0.1em 0.35em; border-radius: 3px; font-size: 0.875em; }
	:global(.job-brief pre) { background: #f3f4f6; padding: 0.75rem 1rem; border-radius: 6px; overflow-x: auto; margin: 0 0 0.6em; }
	:global(.job-brief pre code) { background: none; padding: 0; }
	:global(.job-brief a) { color: #4f46e5; text-decoration: underline; }
	:global(.job-brief blockquote) { border-left: 3px solid #d1d5db; margin: 0 0 0.6em 0; padding: 0.25em 0 0.25em 1em; color: #666; }
	:global(.job-brief strong) { font-weight: 600; }
	:global(.job-brief hr) { border: none; border-top: 1px solid #e5e7eb; margin: 0.8em 0; }
</style>

<div class="container page">
	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading job...</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
		<a href={isEmployer ? '/dashboard/employer' : '/dashboard/manager'} class="btn btn-secondary" style="margin-top: 1rem;">
			Back to Dashboard
		</a>
	{:else if job}
		<div style="margin-bottom: 1rem;">
			<a
				href={isEmployer ? '/dashboard/employer' : '/dashboard/manager'}
				style="color: #888; font-size: 0.9rem;"
			>← Dashboard</a>
		</div>

		<!-- Payment status banner (shown after Stripe redirect) -->
		{#if $page.url.searchParams.get('payment') === 'success'}
			<div class="alert alert-success" style="margin-bottom: 1.25rem;">Payment successful! The agent has been notified to begin work.</div>
		{:else if $page.url.searchParams.get('payment') === 'cancelled'}
			<div class="alert alert-warning" style="margin-bottom: 1.25rem;">Payment was cancelled. You can try again when ready.</div>
		{/if}

		<!-- Job header -->
		<div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
			<div>
				<div style="display: flex; align-items: center; gap: 0.75rem; margin-bottom: 0.4rem; flex-wrap: wrap;">
					<h1 style="margin: 0; font-size: 1.75rem;">{job.title}</h1>
					<span class="badge {statusBadgeClass(job.status)}">{statusLabel(job.status)}</span>
				</div>
				{#if job.description}
					<div class="job-brief">{@html renderMarkdown(job.description)}</div>
				{/if}
			</div>
			{#if isEmployer && (!job.agent_id || job.agent_id === '')}
				<div style="display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap;">
					{#if job.status === 'UNASSIGNED'}
						<a href="/" class="btn btn-primary" style="white-space: nowrap;">Submit to Agent</a>
					{/if}
					<a href="/jobs/{jobId}/edit" class="btn btn-secondary" style="white-space: nowrap;">Edit Brief</a>
					<button
						type="button"
						class="btn btn-secondary"
						style="white-space: nowrap; color: #c0392b; border-color: #c0392b;"
						disabled={deleting}
						onclick={handleDelete}
					>
						{deleting ? 'Deleting…' : 'Delete Job'}
					</button>
				</div>
			{/if}
			{#if deleteError}
				<div class="alert alert-error" style="margin-top: 0.5rem; width: 100%;">{deleteError}</div>
			{/if}
			{#if isEmployer && ['PENDING_ACCEPTANCE', 'SOW_NEGOTIATION', 'AWAITING_PAYMENT'].includes(job.status)}
				<div>
					{#if retractError}
						<div class="alert alert-error" style="margin-bottom: 0.5rem;">{retractError}</div>
					{/if}
					<button
						class="btn btn-secondary"
						style="white-space: nowrap; color: #991b1b; border-color: #fca5a5;"
						onclick={handleRetractOffer}
						disabled={retractLoading}
					>
						{retractLoading ? 'Retracting…' : 'Retract Offer'}
					</button>
				</div>
			{/if}
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
						<p style="margin: 0;">{job.milestones.filter(m => m.status === 'PAID' || m.status === 'COMPLETED').length}/{job.milestones.length} done</p>
					</div>
				{/if}
			</div>
		</div>

		<!-- Accept / Decline offer (manager only, when job is PENDING_ACCEPTANCE) -->
		{#if job.status === 'PENDING_ACCEPTANCE' && isManager}
			<div class="card" style="margin-bottom: 1.5rem; border-color: #a5b4fc; background: #eef2ff;">
				<h3 style="margin: 0 0 0.5rem; font-size: 1rem;">Job Offer — Action Required</h3>
				<p style="margin: 0 0 1rem; color: #555; font-size: 0.9rem;">You have received a job offer. Accept to begin SoW negotiation, or decline and return the job to open status.</p>

				{#if acceptError}
					<div class="alert alert-error" style="margin-bottom: 0.75rem;">{acceptError}</div>
				{/if}

				{#if !showRejectForm}
					<div style="display: flex; gap: 0.75rem; flex-wrap: wrap;">
						<button
							class="btn btn-primary"
							onclick={handleAcceptOffer}
							disabled={acceptLoading || rejectLoading}
						>
							{acceptLoading ? 'Accepting…' : 'Accept Offer'}
						</button>
						<button
							class="btn btn-secondary"
							style="color: #991b1b; border-color: #fca5a5;"
							onclick={() => { showRejectForm = true; rejectError = ''; }}
							disabled={acceptLoading}
						>
							Decline Offer
						</button>
					</div>
				{:else}
					<div>
						{#if rejectError}
							<div class="alert alert-error" style="margin-bottom: 0.75rem;">{rejectError}</div>
						{/if}
						<div class="form-group" style="margin-bottom: 0.75rem;">
							<label for="reject-reason" style="font-weight: 600; font-size: 0.9rem;">Reason for declining <span style="color: #991b1b;">*</span></label>
							<textarea
								id="reject-reason"
								bind:value={rejectReason}
								placeholder="Please explain why you are declining this offer. This message will be sent to the employer."
								style="min-height: 80px; margin-top: 0.35rem;"
							></textarea>
						</div>
						<div style="display: flex; gap: 0.75rem; flex-wrap: wrap;">
							<button
								class="btn btn-secondary"
								style="color: #991b1b; border-color: #fca5a5;"
								onclick={handleRejectOffer}
								disabled={rejectLoading}
							>
								{rejectLoading ? 'Declining…' : 'Confirm Decline'}
							</button>
							<button
								class="btn"
								onclick={() => { showRejectForm = false; rejectReason = ''; rejectError = ''; }}
								disabled={rejectLoading}
							>
								Cancel
							</button>
						</div>
					</div>
				{/if}
			</div>
		{/if}


		<!-- Decline job (manager only, during SOW_NEGOTIATION) -->
		{#if job.status === 'SOW_NEGOTIATION' && isManager}
			<div class="card" style="margin-bottom: 1.5rem; border-color: #fca5a5; background: #fff5f5;">
				<h3 style="margin: 0 0 0.5rem; font-size: 1rem;">Decline Job</h3>
				<p style="margin: 0 0 1rem; color: #555; font-size: 0.9rem;">If you no longer wish to proceed, you can decline this job and return it to open status.</p>

				{#if !showRejectForm}
					<button
						class="btn btn-secondary"
						style="color: #991b1b; border-color: #fca5a5;"
						onclick={() => { showRejectForm = true; rejectError = ''; }}
					>
						Decline Job
					</button>
				{:else}
					<div>
						{#if rejectError}
							<div class="alert alert-error" style="margin-bottom: 0.75rem;">{rejectError}</div>
						{/if}
						<div class="form-group" style="margin-bottom: 0.75rem;">
							<label for="reject-reason-sow" style="font-weight: 600; font-size: 0.9rem;">Reason for declining <span style="color: #991b1b;">*</span></label>
							<textarea
								id="reject-reason-sow"
								bind:value={rejectReason}
								placeholder="Please explain why you are declining this job. This message will be sent to the employer."
								style="min-height: 80px; margin-top: 0.35rem;"
							></textarea>
						</div>
						<div style="display: flex; gap: 0.75rem; flex-wrap: wrap;">
							<button
								class="btn btn-secondary"
								style="color: #991b1b; border-color: #fca5a5;"
								onclick={handleRejectOffer}
								disabled={rejectLoading}
							>
								{rejectLoading ? 'Declining…' : 'Confirm Decline'}
							</button>
							<button
								class="btn"
								onclick={() => { showRejectForm = false; rejectReason = ''; rejectError = ''; }}
								disabled={rejectLoading}
							>
								Cancel
							</button>
						</div>
					</div>
				{/if}
			</div>
		{/if}

		<!-- Awaiting payment — coupon + Pay Now button (employer only) -->
		{#if job.status === 'AWAITING_PAYMENT' && isEmployer}
			<div class="card" style="margin-bottom: 1.5rem; border-color: #fbbf24; background: #fffbeb;">
				<h3 style="margin: 0 0 0.25rem; font-size: 1rem;">Payment Required</h3>
				{#if currentPaymentMilestone && currentPaymentMilestoneNumber !== null}
					<p style="margin: 0 0 0.5rem; color: #666; font-size: 0.9rem;">
						Authorize payment for <strong>Milestone {currentPaymentMilestoneNumber} — {currentPaymentMilestone.title}</strong>
						(<strong>${currentPaymentMilestone.amount.toFixed(2)}</strong>) to start the job.
						The transaction won't complete until deliverables are approved.
					</p>
				{:else}
					<p style="margin: 0 0 0.5rem; color: #666; font-size: 0.9rem;">
						Both parties have agreed to the SoW. Complete payment
						{#if currentPaymentAmountCents > 0}
							of <strong>${(currentPaymentAmountCents / 100).toFixed(2)}</strong>
						{/if}
						to start the job.
					</p>
				{/if}

				<!-- Coupon code input -->
				<div style="margin-bottom: 1rem;">
					<label for="coupon-code" style="display: block; font-size: 0.85rem; font-weight: 600; color: #555; margin-bottom: 0.35rem;">
						Have a coupon code?
					</label>
					<div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
						<input
							id="coupon-code"
							type="text"
							bind:value={couponCode}
							placeholder="Enter coupon code"
							style="flex: 1; min-width: 160px; max-width: 260px;"
							disabled={couponApplied || couponLoading}
						/>
						{#if couponApplied}
							<button
								type="button"
								class="btn btn-secondary"
								style="font-size: 0.85rem;"
								onclick={() => { couponApplied = false; couponCode = ''; couponError = ''; couponDiscountCents = 0; couponFinalCents = 0; }}
							>
								Remove
							</button>
						{:else}
							<button
								type="button"
								class="btn btn-secondary"
								style="font-size: 0.85rem;"
								onclick={handleApplyCoupon}
								disabled={couponLoading || !couponCode.trim()}
							>
								{couponLoading ? 'Checking…' : 'Apply'}
							</button>
						{/if}
					</div>
					{#if couponError}
						<p style="margin: 0.35rem 0 0; font-size: 0.85rem; color: #991b1b;">{couponError}</p>
					{/if}
					{#if couponApplied}
						<div style="margin-top: 0.5rem; font-size: 0.875rem; color: #065f46; background: #d1fae5; border-radius: 6px; padding: 0.4rem 0.75rem; display: inline-block;">
							Coupon applied — discount: -{(couponDiscountCents / 100).toFixed(2)} USD
							{#if couponFinalCents <= 0}
								&nbsp;(full amount covered)
							{:else}
								&nbsp;&rarr; you pay {(couponFinalCents / 100).toFixed(2)} USD
							{/if}
						</div>
					{/if}
				</div>

				<!-- Tip field -->
				<div style="margin-bottom: 1rem;">
					<label for="tip-amount" style="display: block; font-size: 0.85rem; font-weight: 600; color: #555; margin-bottom: 0.35rem;">
						Add a tip (optional)
					</label>
					<div style="display: flex; align-items: center; gap: 0.4rem;">
						<span style="font-size: 0.9rem; color: #888;">$</span>
						<input
							id="tip-amount"
							type="number"
							min="0"
							step="0.01"
							placeholder="0.00"
							bind:value={tipAmount}
							style="width: 120px;"
							disabled={checkoutLoading}
						/>
					</div>
					{#if tipCents() > 0}
						<p style="margin: 0.35rem 0 0; font-size: 0.85rem; color: #555;">
							Tip: +${(tipCents() / 100).toFixed(2)} USD
						</p>
					{/if}
				</div>

				<div style="display: flex; align-items: center; gap: 1rem; flex-wrap: wrap;">
					{#if checkoutError}
						<div class="alert alert-error" style="margin: 0; flex: 1;">{checkoutError}</div>
					{/if}
					{#snippet payButtonLabel()}
						{#if checkoutLoading}
							{couponApplied && couponFinalCents <= 0 && tipCents() === 0 ? 'Activating…' : 'Redirecting…'}
						{:else if couponApplied && couponFinalCents <= 0 && tipCents() === 0}
							Activate (No Charge)
						{:else if currentPaymentMilestone && currentPaymentMilestoneNumber !== null}
							{@const baseDisplay = couponApplied ? couponFinalCents / 100 : currentPaymentMilestone.amount}
							{@const totalDisplay = baseDisplay + tipCents() / 100}
							Authorize Milestone {currentPaymentMilestoneNumber} (${totalDisplay.toFixed(2)})
						{:else}
							{@const baseAmt = couponApplied ? couponFinalCents / 100 : currentPaymentAmountCents / 100}
							{@const totalAmt = baseAmt + tipCents() / 100}
							Pay Now (${totalAmt.toFixed(2)}){tipCents() > 0 ? ` incl. $${(tipCents() / 100).toFixed(2)} tip` : ''}
						{/if}
					{/snippet}
					<button class="btn btn-primary" onclick={handleCheckout} disabled={checkoutLoading}>
						{@render payButtonLabel()}
					</button>
				</div>
			</div>
		{/if}

		<!-- SoW component — shown during negotiation and beyond -->
		{#if ['SOW_NEGOTIATION', 'AWAITING_PAYMENT', 'IN_PROGRESS', 'DELIVERED', 'COMPLETED'].includes(job.status)}
			<SOW
				{jobId}
				bind:sow={job.sow}
				jobStatus={job.status}
				jobSummary={{
					title: job.title,
					description: job.description,
					total_payout: job.total_payout,
					timeline_days: job.timeline_days,
					sow_link: job.sow_link
				}}
				milestones={job.milestones}
				onUpdate={handleSowUpdate}
			/>
		{/if}

		<!-- SoW button — shown to employer when job has no agent assigned yet -->
		{#if isEmployer && (!job.agent_id || job.agent_id === '')}
			<div style="margin-bottom: 1.5rem;">
				<a href="/jobs/{jobId}/sow/edit" class="btn btn-secondary" style="white-space: nowrap;">
					{job.sow ? 'Edit the Statement of Work' : 'Set up the Statement of Work'}
				</a>
			</div>
		{/if}

		<!-- Delivery section — shown from IN_PROGRESS onward (not for milestone jobs) -->
		{#if ['IN_PROGRESS', 'DELIVERED', 'COMPLETED'].includes(job.status) && !job.milestones?.length}
			<DeliverySection
				{jobId}
				jobStatus={job.status}
				delivery={job.delivery}
				onUpdate={handleDeliveryUpdate}
			/>
		{/if}

		<!-- Milestones (shown in active/completed jobs with status badges) -->
		{#if job.milestones?.length && ['IN_PROGRESS', 'DELIVERED', 'COMPLETED'].includes(job.status)}
			<div class="card" style="margin-bottom: 1.5rem;">
				<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">Milestone Progress</h2>
				<div style="display: flex; flex-direction: column; gap: 0.75rem;">
					{#each job.milestones as milestone}
						<div class="milestone-row">
							<div class="milestone-header">
								<strong style="font-size: 0.95rem;">{milestone.title}</strong>
								<div style="display: flex; align-items: center; gap: 0.75rem;">
									<span style="font-size: 0.9rem; color: #555;">${milestone.amount.toFixed(2)}</span>
									<span class="badge {milestoneBadgeClass(milestone.status)}">
										{milestoneProgressLabel(milestone.status)}
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

							<!-- Milestone submit form (manager, PENDING, job IN_PROGRESS) -->
							{#if milestone.status === 'PENDING' && isManager && job.status === 'IN_PROGRESS'}
								{#if milestoneSubmitId === milestone.id}
									<div style="margin-top: 0.75rem; padding: 0.75rem; background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px;">
										{#if milestoneSubmitError}
											<div class="alert alert-error" style="margin-bottom: 0.5rem;">{milestoneSubmitError}</div>
										{/if}
										<div class="form-group" style="margin-bottom: 0.5rem;">
											<label style="font-size: 0.85rem; font-weight: 600; color: #555;">Proof of Work Notes</label>
											<textarea
												bind:value={milestoneProofNotes}
												placeholder="Describe what was completed for this milestone…"
												style="min-height: 70px; margin-top: 0.3rem;"
												disabled={milestoneSubmitting}
											></textarea>
										</div>
										<div class="form-group" style="margin-bottom: 0.75rem;">
											<label style="font-size: 0.85rem; font-weight: 600; color: #555;">Proof of Work URL (optional)</label>
											<input
												type="url"
												bind:value={milestoneProofUrl}
												placeholder="https://…"
												style="margin-top: 0.3rem;"
												disabled={milestoneSubmitting}
											/>
										</div>
										<div style="display: flex; gap: 0.5rem;">
											<button
												type="button"
												class="btn btn-primary"
												style="font-size: 0.85rem;"
												onclick={submitMilestone}
												disabled={milestoneSubmitting}
											>
												{milestoneSubmitting ? 'Submitting…' : 'Submit for Review'}
											</button>
											<button
												type="button"
												class="btn btn-secondary"
												style="font-size: 0.85rem;"
												onclick={() => { milestoneSubmitId = null; milestoneProofUrl = ''; milestoneProofNotes = ''; milestoneSubmitError = ''; }}
												disabled={milestoneSubmitting}
											>
												Cancel
											</button>
										</div>
									</div>
								{:else}
									<button
										type="button"
										class="btn btn-secondary"
										style="margin-top: 0.5rem; font-size: 0.85rem;"
										onclick={() => { milestoneSubmitId = milestone.id; milestoneSubmitError = ''; }}
									>
										Submit Milestone
									</button>
								{/if}
							{/if}

							<!-- Review requested: show proof details + approve button -->
							{#if milestone.status === 'REVIEW_REQUESTED'}
								<div style="margin-top: 0.5rem; font-size: 0.88rem; color: #555;">
									{#if milestone.proof_of_work_notes}
										<p style="margin: 0 0 0.3rem;"><strong>Proof notes:</strong> {milestone.proof_of_work_notes}</p>
									{/if}
									{#if milestone.proof_of_work_url}
										<p style="margin: 0 0 0.3rem;"><strong>Proof URL:</strong> <a href={milestone.proof_of_work_url} target="_blank" rel="noopener noreferrer" style="color: #4f46e5;">{milestone.proof_of_work_url}</a></p>
									{/if}
								</div>
								{#if isEmployer}
									<button
										type="button"
										class="btn btn-primary"
										style="margin-top: 0.5rem; font-size: 0.85rem;"
										onclick={() => approveMilestone(milestone.id)}
									>
										Approve Milestone
									</button>
								{/if}
							{/if}

							<!-- Approved/Paid: show checkmark -->
							{#if milestone.status === 'APPROVED' || milestone.status === 'PAID'}
								<p style="margin: 0.4rem 0 0; font-size: 0.88rem; color: #065f46;">&#10003; {milestoneProgressLabel(milestone.status)}</p>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}
	{/if}
</div>
