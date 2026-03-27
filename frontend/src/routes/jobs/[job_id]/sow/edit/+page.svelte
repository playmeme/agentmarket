<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';

	// Stepped auto-resize for textareas
	const ROW_STEP = 3;
	const MIN_ROWS = 3;

	function steppedResize(node: HTMLTextAreaElement) {
		function resize() {
			node.rows = MIN_ROWS;
			const lineHeight = parseFloat(getComputedStyle(node).lineHeight) || 20;
			const paddingV =
				parseFloat(getComputedStyle(node).paddingTop) +
				parseFloat(getComputedStyle(node).paddingBottom);
			const naturalLines = Math.ceil((node.scrollHeight - paddingV) / lineHeight);
			const steppedLines = Math.max(MIN_ROWS, Math.ceil(naturalLines / ROW_STEP) * ROW_STEP);
			node.rows = steppedLines;
		}
		node.addEventListener('input', resize);
		resize();
		return {
			destroy() {
				node.removeEventListener('input', resize);
			}
		};
	}

	interface Criterion {
		id?: string;
		description: string;
	}

	interface MilestoneData {
		id?: string;
		title: string;
		amount: number;
		deliverables: string;
		criteria: Criterion[];
		status?: string;
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
		milestones: MilestoneData[];
		sow: SOWData | null;
	}

	interface EditMilestone {
		title: string;
		payout: number;
		deliverables: string;
		criteria: string[];
	}

	const jobId = $derived($page.params.job_id ?? '');

	let job: Job | null = $state(null);
	let loading = $state(true);
	let loadError = $state('');
	let saving = $state(false);
	let saveError = $state('');
	let successMsg = $state('');

	// Lock state
	let lockDenied = $state(false);
	let lockDeniedBy = $state('');
	let heartbeatInterval: ReturnType<typeof setInterval> | null = null;
	let lockHeld = $state(false);

	// SoW form fields
	let editDetailedSpec = $state('');
	let editWorkProcess = $state('');
	let editPriceDollars = $state('');
	let editTimelineDays = $state('');
	let editMilestones: EditMilestone[] = $state([{ title: '', payout: 0, deliverables: '', criteria: [''] }]);

	const isEmployer = $derived($auth?.role === 'EMPLOYER');

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/auth/login');
			return;
		}
		if ($auth?.role !== 'EMPLOYER') {
			goto('/dashboard/manager');
			return;
		}
		await loadJob();
		await acquireLock();
	});

	onDestroy(() => {
		if (heartbeatInterval !== null) {
			clearInterval(heartbeatInterval);
			heartbeatInterval = null;
		}
		if (lockHeld) {
			apiFetch(`/api/ui/jobs/${jobId}/sow/unlock`, { method: 'POST' }).catch(() => {});
		}
	});

	async function acquireLock() {
		// Only attempt to lock if the SoW exists (pre-fill edits on jobs with no SoW yet
		// don't need locking because only the employer can access this page at that stage).
		if (!job?.sow) return;

		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/sow/lock`, { method: 'POST' });
			if (res.status === 409) {
				const data = await res.json().catch(() => ({}));
				lockDenied = true;
				lockDeniedBy = data.editing_by ?? 'another user';
				return;
			}
			if (res.ok) {
				lockHeld = true;
				heartbeatInterval = setInterval(async () => {
					await apiFetch(`/api/ui/jobs/${jobId}/sow/heartbeat`, { method: 'POST' }).catch(() => {});
				}, 60_000);
			}
		} catch {
			// Non-fatal — proceed without lock on network error
		}
	}

	function handleBeforeUnload() {
		if (lockHeld) {
			navigator.sendBeacon(`/api/ui/jobs/${jobId}/sow/unlock`);
		}
	}

	async function loadJob() {
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}`);
			if (!res.ok) {
				if (res.status === 404) throw new Error('Job not found');
				throw new Error('Failed to load job');
			}
			job = await res.json();

			// Pre-populate SoW fields
			if (job?.sow) {
				// SoW already exists — load its values
				editDetailedSpec = job.sow.detailed_spec ?? '';
				editWorkProcess = job.sow.work_process ?? '';
				editPriceDollars = (job.sow.price_cents / 100).toFixed(2);
				editTimelineDays = String(job.sow.timeline_days ?? '');
			} else {
				// No SoW yet — pre-populate from Job Brief data
				editDetailedSpec = '';
				editWorkProcess = '';
				editPriceDollars = job ? String(job.total_payout) : '';
				editTimelineDays = job?.timeline_days ? String(job.timeline_days) : '';
			}

			// Pre-populate milestones from existing data if available
			if (job?.milestones && job.milestones.length > 0) {
				editMilestones = job.milestones.map((m) => ({
					title: m.title,
					payout: m.amount,
					deliverables: m.deliverables ?? '',
					criteria: m.criteria && m.criteria.length > 0
						? m.criteria.map((c) => c.description)
						: ['']
				}));
			} else {
				editMilestones = [];
			}
		} catch (e: unknown) {
			loadError = e instanceof Error ? e.message : 'Failed to load job';
		} finally {
			loading = false;
		}
	}

	// Milestone helpers
	function addMilestone() {
		editMilestones = [...editMilestones, { title: '', payout: 0, deliverables: '', criteria: [''] }];
	}

	function removeMilestone(i: number) {
		editMilestones = editMilestones.filter((_, idx) => idx !== i);
	}

	function addCriteria(milestoneIdx: number) {
		editMilestones = editMilestones.map((m, i) =>
			i === milestoneIdx ? { ...m, criteria: [...m.criteria, ''] } : m
		);
	}

	function removeCriteria(milestoneIdx: number, criteriaIdx: number) {
		editMilestones = editMilestones.map((m, i) =>
			i === milestoneIdx
				? { ...m, criteria: m.criteria.filter((_, ci) => ci !== criteriaIdx) }
				: m
		);
	}

	function updateCriteria(milestoneIdx: number, criteriaIdx: number, value: string) {
		editMilestones = editMilestones.map((m, i) =>
			i === milestoneIdx
				? { ...m, criteria: m.criteria.map((c, ci) => (ci === criteriaIdx ? value : c)) }
				: m
		);
	}

	// Derived: sum of milestone payouts in dollars
	const milestonesTotalDollars = $derived(
		editMilestones.reduce((sum, m) => sum + (Number(m.payout) || 0), 0)
	);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		saveError = '';
		successMsg = '';
		try {
			const priceCents = Math.round(parseFloat(editPriceDollars) * 100) || 0;

			// Client-side validation: milestone totals must not exceed total price
			if (priceCents > 0 && milestonesTotalDollars * 100 > priceCents) {
				const totalFormatted = (priceCents / 100).toFixed(2);
				const msFormatted = milestonesTotalDollars.toFixed(2);
				throw new Error(
					`Milestone payments total ($${msFormatted}) exceeds the SoW price ($${totalFormatted}). Please adjust the milestone payouts.`
				);
			}

			const res = await apiFetch(`/api/ui/jobs/${jobId}/sow`, {
				method: 'POST',
				body: JSON.stringify({
					detailed_spec: editDetailedSpec,
					work_process: editWorkProcess,
					price_cents: priceCents,
					timeline_days: parseInt(editTimelineDays) || 0,
					milestones: editMilestones.map((m) => ({
						title: m.title,
						amount: Math.round(Number(m.payout || 0)),
						deliverables: m.deliverables,
						criteria: m.criteria.filter((c) => c.trim())
					}))
				})
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to save SoW' }));
				throw new Error(err.error || 'Failed to save SoW');
			}

			// Stop heartbeat — lock is cleared by the save endpoint
			if (heartbeatInterval !== null) {
				clearInterval(heartbeatInterval);
				heartbeatInterval = null;
			}
			lockHeld = false;

			successMsg = 'Statement of Work saved.';
			goto(`/jobs/${jobId}`);
		} catch (e: unknown) {
			saveError = e instanceof Error ? e.message : 'Failed to save SoW';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Edit Statement of Work — {SITE_NAME}</title>
</svelte:head>

<svelte:window onbeforeunload={handleBeforeUnload} />

<div class="container page">
	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading...</p>
	{:else if loadError}
		<div class="alert alert-error">{loadError}</div>
		<a href="/dashboard/employer" class="btn btn-secondary" style="margin-top: 1rem;">Back to Dashboard</a>
	{:else if job}
		<div style="margin-bottom: 1rem;">
			<a href="/jobs/{jobId}" style="color: #888; font-size: 0.9rem;">← Back to Job</a>
		</div>

		<div class="page-header">
			<h1>Statement of Work</h1>
			<p>Pre-fill the SoW for <strong>{job.title}</strong>. The agent can review and refine it once assigned.</p>
		</div>

		<!-- Job Brief summary (read-only) -->
		<div class="card" style="margin-bottom: 1.5rem;">
			<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">Job Brief</h2>
			<div style="display: grid; gap: 0.75rem;">
				<div>
					<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Title</p>
					<p style="margin: 0; color: #1a1a1a; font-weight: 500;">{job.title}</p>
				</div>
				{#if job.description}
					<div>
						<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Brief Description</p>
						<p style="margin: 0; color: #333; white-space: pre-wrap;">{job.description}</p>
					</div>
				{/if}
				<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
					<div>
						<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Total Payout</p>
						<p style="margin: 0; font-size: 1.05rem; font-weight: 600;">${job.total_payout.toFixed(2)}</p>
					</div>
					{#if job.timeline_days}
						<div>
							<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Timeline</p>
							<p style="margin: 0;">{job.timeline_days} day{job.timeline_days !== 1 ? 's' : ''}</p>
						</div>
					{/if}
				</div>
			</div>
		</div>

		{#if lockDenied}
			<div class="alert alert-error" style="margin-bottom: 1rem;">
				This SoW is currently being edited by <strong>{lockDeniedBy}</strong>. Please wait until they finish or try again in a few minutes.
			</div>
		{/if}
		{#if saveError}
			<div class="alert alert-error">{saveError}</div>
		{/if}
		{#if successMsg}
			<div class="alert alert-success">{successMsg}</div>
		{/if}

		<form onsubmit={handleSubmit}>
			<div class="card" style="margin-bottom: 1.5rem;">
				<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">SoW Details</h2>
				<p style="color: #666; font-size: 0.9rem; margin: 0 0 1rem;">
					These fields are optional — leave blank and the agent can fill them in during negotiation.
				</p>

				<div class="form-group">
					<label for="sow-detailed-spec">
						Detailed Specification of Work
						<span style="font-weight: 400; color: #777; font-size: 0.85rem;"> — Please describe in more detail what is expected to be done.</span>
					</label>
					<textarea id="sow-detailed-spec" bind:value={editDetailedSpec} rows={MIN_ROWS} use:steppedResize placeholder="Describe in detail what work needs to be done, the expected outcomes, and any technical requirements..."></textarea>
				</div>
				<div class="form-group">
					<label for="sow-work-process">
						Work Process
						<span style="font-weight: 400; color: #777; font-size: 0.85rem;"> — Please describe procedures such as the frequency and channels of communication, what the Employer will provide to facilitate the work, and how the Agent will submit deliverables.</span>
					</label>
					<textarea id="sow-work-process" bind:value={editWorkProcess} rows={MIN_ROWS} use:steppedResize placeholder="e.g. Weekly check-ins via email; employer will provide API keys and access credentials; deliverables submitted via GitHub PR..."></textarea>
				</div>
				<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1rem;">
					<div class="form-group" style="margin-bottom: 0;">
						<label for="sow-price">Price (USD)</label>
						<input id="sow-price" type="number" bind:value={editPriceDollars} min="0" step="0.01" placeholder="0.00" />
					</div>
					<div class="form-group" style="margin-bottom: 0;">
						<label for="sow-timeline">Timeline (days)</label>
						<input id="sow-timeline" type="number" bind:value={editTimelineDays} min="1" step="1" placeholder="7" />
					</div>
				</div>
			</div>

			<!-- Milestones section -->
			<div class="card" style="margin-bottom: 1.5rem;">
				<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">Milestones</h2>

				<!-- Milestone totals summary -->
				{#if editPriceDollars && parseFloat(editPriceDollars) > 0}
					{@const totalPrice = parseFloat(editPriceDollars)}
					{@const milestonesOver = milestonesTotalDollars > totalPrice}
					<div style="margin-bottom: 1rem; padding: 0.6rem 0.9rem; border-radius: 6px; font-size: 0.9rem; background: {milestonesOver ? '#fff3cd' : '#f0f4ff'}; border: 1px solid {milestonesOver ? '#ffc107' : '#c8d8ff'}; color: {milestonesOver ? '#856404' : '#444'};">
						Milestone total: <strong>${milestonesTotalDollars.toFixed(2)}</strong> / SoW price: <strong>${totalPrice.toFixed(2)}</strong>
						{#if milestonesOver}
							— <strong>Milestone payments exceed the SoW price.</strong>
						{/if}
					</div>
				{/if}

				{#each editMilestones as milestone, i}
					<div class="milestone-row">
						<div class="milestone-header">
							<strong style="font-size: 0.9rem;">Milestone {i + 1}</strong>
							<button type="button" class="btn btn-danger" onclick={() => removeMilestone(i)} style="font-size: 0.8rem; padding: 0.2rem 0.6rem;">
								Remove
							</button>
						</div>

						<div style="display: grid; grid-template-columns: 1fr auto; gap: 0.75rem; align-items: start;">
							<div class="form-group" style="margin-bottom: 0.5rem;">
								<label for="ms-title-{i}">Title</label>
								<input id="ms-title-{i}" type="text" bind:value={milestone.title} placeholder="Milestone title" />
							</div>
							<div class="form-group" style="margin-bottom: 0.5rem;">
								<label for="ms-payout-{i}">Payout after completion (USD)</label>
								<input id="ms-payout-{i}" type="number" bind:value={milestone.payout} min="0" step="0.01" placeholder="0.00" style="width: 150px;" />
							</div>
						</div>

						<div class="form-group" style="margin-bottom: 0.75rem;">
							<label for="ms-deliverables-{i}">
								Deliverables
								<span style="font-weight: 400; color: #777; font-size: 0.85rem;"> — What will result from the work? e.g. physical objects, digital goods, time spent, information</span>
							</label>
							<textarea
								id="ms-deliverables-{i}"
								bind:value={milestone.deliverables}
								rows={MIN_ROWS}
								use:steppedResize
								placeholder="e.g. Deployed web application, source code repository, documentation..."
							></textarea>
						</div>

						<div>
							<p style="font-size: 0.9rem; font-weight: 500; color: #333; margin: 0 0 0.35rem;">
								Acceptance criteria
								<span style="font-weight: 400; color: #777; font-size: 0.85rem;"> — What does success look like?</span>
							</p>
							{#each milestone.criteria as criterion, ci}
								<div style="display: flex; gap: 0.5rem; margin-bottom: 0.4rem; align-items: flex-start;">
									<textarea
										value={criterion}
										oninput={(e) => updateCriteria(i, ci, (e.target as HTMLTextAreaElement).value)}
										placeholder="e.g. All tests pass, page loads in under 2s"
										rows={MIN_ROWS}
										use:steppedResize
										style="flex: 1; padding: 0.4rem 0.6rem; border: 1px solid #ced4da; border-radius: 6px; font-size: 0.9rem; resize: none;"
									></textarea>
									{#if milestone.criteria.length > 1}
										<button type="button" onclick={() => removeCriteria(i, ci)} style="background: none; border: none; color: #dc3545; cursor: pointer; font-size: 1.1rem; padding: 0.3rem 0.25rem;" title="Remove">×</button>
									{/if}
								</div>
							{/each}
							<button type="button" onclick={() => addCriteria(i)} style="background: none; border: none; color: #0066cc; cursor: pointer; font-size: 0.85rem; padding: 0.1rem 0; margin-top: 0.2rem;">
								+ Add criterion
							</button>
						</div>
					</div>
				{/each}

				<button type="button" class="btn btn-secondary" onclick={addMilestone} style="font-size: 0.85rem; padding: 0.35rem 0.9rem; margin-top: 0.5rem;">
					+ Add milestone
				</button>
			</div>

			<div style="display: flex; gap: 0.75rem; align-items: center;">
				<button type="submit" class="btn btn-primary" disabled={saving || lockDenied}>
					{saving ? 'Saving…' : 'Save SoW'}
				</button>
				<a
					href="/jobs/{jobId}"
					class="btn btn-secondary"
					onclick={async () => {
						if (heartbeatInterval !== null) {
							clearInterval(heartbeatInterval);
							heartbeatInterval = null;
						}
						if (lockHeld) {
							lockHeld = false;
							await apiFetch(`/api/ui/jobs/${jobId}/sow/unlock`, { method: 'POST' }).catch(() => {});
						}
					}}
				>Cancel</a>
			</div>
		</form>
	{/if}
</div>
