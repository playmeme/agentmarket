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

	const isEmployer = $derived($auth?.role === 'EMPLOYER');
	const isManager = $derived($auth?.role === 'AGENT_MANAGER');

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
			CANCELLED: 'badge-cancelled',
			RETRACTED: 'badge-cancelled'
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
			await loadJob();
		} catch (e: unknown) {
			rejectError = e instanceof Error ? e.message : 'Failed to reject offer';
		} finally {
			rejectLoading = false;
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
					<a href="/jobs/{jobId}/edit" class="btn btn-secondary" style="white-space: nowrap;">Edit Brief</a>
					<a href="/jobs/{jobId}/sow/edit" class="btn btn-secondary" style="white-space: nowrap;">Set up SoW</a>
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
						<p style="margin: 0;">{job.milestones.filter(m => m.status === 'COMPLETED').length}/{job.milestones.length} done</p>
					</div>
				{/if}
			</div>
		</div>

		<!-- Accept / Reject offer (handler only, when job is PENDING_ACCEPTANCE) -->
		{#if job.status === 'PENDING_ACCEPTANCE' && isManager}
			<div class="card" style="margin-bottom: 1.5rem; border-color: #a5b4fc; background: #eef2ff;">
				<h3 style="margin: 0 0 0.5rem; font-size: 1rem;">Job Offer — Action Required</h3>
				<p style="margin: 0 0 1rem; color: #555; font-size: 0.9rem;">You have received a job offer. Accept to begin SoW negotiation, or reject and return the job to open status.</p>

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
							Reject Offer
						</button>
					</div>
				{:else}
					<div>
						{#if rejectError}
							<div class="alert alert-error" style="margin-bottom: 0.75rem;">{rejectError}</div>
						{/if}
						<div class="form-group" style="margin-bottom: 0.75rem;">
							<label for="reject-reason" style="font-weight: 600; font-size: 0.9rem;">Reason for rejection <span style="color: #991b1b;">*</span></label>
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
								{rejectLoading ? 'Rejecting…' : 'Confirm Rejection'}
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

		<!-- Delivery section — shown from IN_PROGRESS onward -->
		{#if ['IN_PROGRESS', 'DELIVERED', 'COMPLETED'].includes(job.status)}
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
