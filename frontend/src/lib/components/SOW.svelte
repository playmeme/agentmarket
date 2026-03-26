<script lang="ts">
	import { apiFetch, auth } from '$lib/stores/auth';

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

	interface MilestoneData {
		id?: string;
		title: string;
		amount: number;
		deliverables: string;
		criteria: Array<{ id?: string; description: string }>;
		status?: string;
	}

	interface JobSummary {
		title: string;
		description: string;
		total_payout: number;
		timeline_days: number;
		sow_link?: string;
	}

	interface Props {
		jobId: string;
		sow: SOWData | null;
		jobStatus: string;
		jobSummary?: JobSummary | null;
		milestones?: MilestoneData[];
		onUpdate?: () => void;
	}

	let { jobId, sow = $bindable(), jobStatus, jobSummary = null, milestones = [], onUpdate }: Props = $props();

	let editing = $state(false);
	let saving = $state(false);
	let accepting = $state(false);
	let proceedLoading = $state(false);
	let error = $state('');
	let successMsg = $state('');

	// Edit form state — SoW fields
	let editDetailedSpec = $state('');
	let editWorkProcess = $state('');
	let editPriceDollars = $state('');
	let editTimelineDays = $state('');

	// Edit form state — milestones
	interface EditMilestone {
		title: string;
		payout: number;
		deliverables: string;
		criteria: string[];
	}
	let editMilestones: EditMilestone[] = $state([]);

	function formatCentsToDollars(cents: number): string {
		return (cents / 100).toFixed(2);
	}

	function startEdit() {
		if (sow) {
			editDetailedSpec = sow.detailed_spec;
			editWorkProcess = sow.work_process;
			editPriceDollars = formatCentsToDollars(sow.price_cents);
			editTimelineDays = String(sow.timeline_days);
		} else {
			editDetailedSpec = '';
			editWorkProcess = '';
			editPriceDollars = '';
			editTimelineDays = '';
		}
		// Populate milestones from existing data
		if (milestones && milestones.length > 0) {
			editMilestones = milestones.map((m) => ({
				title: m.title,
				payout: m.amount,
				deliverables: m.deliverables ?? '',
				criteria: m.criteria && m.criteria.length > 0
					? m.criteria.map((c) => c.description)
					: ['']
			}));
		} else {
			editMilestones = [{ title: '', payout: 0, deliverables: '', criteria: [''] }];
		}
		editing = true;
		error = '';
	}

	function cancelEdit() {
		editing = false;
		error = '';
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

	function addDeliverable(milestoneIdx: number) {
		// Deliverables is a free-text textarea, handled directly
	}

	// Derived: sum of milestone payouts in dollars
	const milestonesTotalDollars = $derived(
		editMilestones.reduce((sum, m) => sum + (Number(m.payout) || 0), 0)
	);

	async function saveSow() {
		saving = true;
		error = '';
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
			sow = await res.json();
			editing = false;
			successMsg = 'Statement of Work saved.';
			onUpdate?.();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to save SoW';
		} finally {
			saving = false;
		}
	}

	async function acceptSow() {
		accepting = true;
		error = '';
		successMsg = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/sow/accept`, {
				method: 'POST'
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to accept SoW' }));
				throw new Error(err.error || 'Failed to accept SoW');
			}
			sow = await res.json();
			successMsg = 'You have accepted the Statement of Work.';
			onUpdate?.();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to accept SoW';
		} finally {
			accepting = false;
		}
	}

	async function proceedToPayment() {
		proceedLoading = true;
		error = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/checkout`, {
				method: 'POST'
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to initiate checkout' }));
				throw new Error(err.error || 'Failed to initiate checkout');
			}
			const data = await res.json();
			if (data.checkout_url) {
				window.location.href = data.checkout_url;
			} else {
				throw new Error('No checkout URL returned');
			}
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to proceed to payment';
		} finally {
			proceedLoading = false;
		}
	}

	const isEmployer = $derived($auth?.role === 'EMPLOYER');
	const isManager = $derived($auth?.role === 'AGENT_MANAGER');

	// Check if current user already accepted
	const userAccepted = $derived(
		sow && ((isEmployer && sow.employer_accepted) || (isManager && sow.agent_accepted))
	);

	const bothAccepted = $derived(sow && sow.employer_accepted && sow.agent_accepted);
</script>

<!-- Job section (read-only summary) -->
{#if jobSummary}
<div class="card" style="margin-bottom: 1.5rem;">
	<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">Job</h2>
	<div style="display: grid; gap: 0.75rem;">
		<div>
			<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Title</p>
			<p style="margin: 0; color: #1a1a1a; font-weight: 500;">{jobSummary.title}</p>
		</div>
		{#if jobSummary.description}
			<div>
				<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Brief Description</p>
				<p style="margin: 0; color: #333; white-space: pre-wrap;">{jobSummary.description}</p>
			</div>
		{/if}
		<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
			<div>
				<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Total Payout</p>
				<p style="margin: 0; font-size: 1.05rem; font-weight: 600;">${jobSummary.total_payout.toFixed(2)}</p>
			</div>
			{#if jobSummary.timeline_days}
				<div>
					<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">Timeline</p>
					<p style="margin: 0;">{jobSummary.timeline_days} day{jobSummary.timeline_days !== 1 ? 's' : ''}</p>
				</div>
			{/if}
		</div>
		{#if jobSummary.sow_link}
			<div>
				<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.04em;">SoW Reference</p>
				<a href={jobSummary.sow_link} target="_blank" rel="noopener noreferrer" style="color: #4f46e5; word-break: break-all;">{jobSummary.sow_link}</a>
			</div>
		{/if}
	</div>
</div>
{/if}

<!-- SoW section -->
<div class="card" style="margin-bottom: 1.5rem;">
	<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
		<h2 style="margin: 0; font-size: 1.1rem;">Statement of Work</h2>
		{#if !editing && jobStatus === 'SOW_NEGOTIATION'}
			<button class="btn btn-secondary" onclick={startEdit} style="font-size: 0.85rem; padding: 0.35rem 0.9rem;">
				{sow ? 'Edit SoW' : 'Create SoW'}
			</button>
		{/if}
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}
	{#if successMsg}
		<div class="alert alert-success">{successMsg}</div>
	{/if}

	{#if editing}
		<form onsubmit={(e) => { e.preventDefault(); saveSow(); }}>

			<!-- SoW fields -->
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

			<!-- Milestones section -->
			<div style="border-top: 1px solid #f0f0f0; margin-top: 1rem; padding-top: 1rem;">
				<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
					<h3 style="margin: 0; font-size: 1rem; font-weight: 600;">Milestones</h3>
					<button type="button" class="btn btn-secondary" onclick={addMilestone} style="font-size: 0.85rem; padding: 0.35rem 0.9rem;">
						+ Add milestone
					</button>
				</div>

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
							{#if editMilestones.length > 1}
								<button type="button" class="btn btn-danger" onclick={() => removeMilestone(i)} style="font-size: 0.8rem; padding: 0.2rem 0.6rem;">
									Remove
								</button>
							{/if}
						</div>

						<div style="display: grid; grid-template-columns: 1fr auto; gap: 0.75rem; align-items: start;">
							<div class="form-group" style="margin-bottom: 0.5rem;">
								<label for="ms-title-{i}">Title</label>
								<input id="ms-title-{i}" type="text" bind:value={milestone.title} required placeholder="Milestone title" />
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
			</div>

			<div style="display: flex; gap: 0.75rem; margin-top: 1.25rem;">
				<button type="submit" class="btn btn-primary" disabled={saving}>
					{saving ? 'Saving…' : 'Save SoW'}
				</button>
				<button type="button" class="btn btn-secondary" onclick={cancelEdit} disabled={saving}>
					Cancel
				</button>
			</div>
		</form>
	{:else if sow}
		<div style="display: grid; gap: 1rem;">
			{#if sow.detailed_spec}
				<div>
					<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Detailed Specification of Work</p>
					<p style="margin: 0; color: #333; white-space: pre-wrap;">{sow.detailed_spec}</p>
				</div>
			{/if}
			{#if sow.work_process}
				<div>
					<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Work Process</p>
					<p style="margin: 0; color: #333; white-space: pre-wrap;">{sow.work_process}</p>
				</div>
			{/if}
			{#if sow.price_cents > 0 || sow.timeline_days > 0}
				<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
					{#if sow.price_cents > 0}
						<div>
							<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Price</p>
							<p style="margin: 0; font-size: 1.1rem; font-weight: 600; color: #1a1a1a;">${formatCentsToDollars(sow.price_cents)}</p>
						</div>
					{/if}
					{#if sow.timeline_days > 0}
						<div>
							<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Timeline</p>
							<p style="margin: 0; color: #333;">{sow.timeline_days} day{sow.timeline_days !== 1 ? 's' : ''}</p>
						</div>
					{/if}
				</div>
			{/if}

			<!-- Acceptance status -->
			<div style="border-top: 1px solid #f0f0f0; padding-top: 1rem;">
				<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.5rem; text-transform: uppercase; letter-spacing: 0.04em;">Acceptance Status</p>
				<div style="display: flex; gap: 1rem; flex-wrap: wrap;">
					<div style="display: flex; align-items: center; gap: 0.4rem; font-size: 0.9rem;">
						<span style="width: 10px; height: 10px; border-radius: 50%; background: {sow.employer_accepted ? '#10b981' : '#e5e7eb'}; display: inline-block;"></span>
						<span>Employer: {sow.employer_accepted ? 'Accepted' : 'Pending'}</span>
					</div>
					<div style="display: flex; align-items: center; gap: 0.4rem; font-size: 0.9rem;">
						<span style="width: 10px; height: 10px; border-radius: 50%; background: {sow.agent_accepted ? '#10b981' : '#e5e7eb'}; display: inline-block;"></span>
						<span>Agent: {sow.agent_accepted ? 'Accepted' : 'Pending'}</span>
					</div>
				</div>
			</div>

			<!-- Action buttons -->
			{#if jobStatus === 'SOW_NEGOTIATION'}
				{#if !userAccepted}
					<div>
						<button class="btn btn-primary" onclick={acceptSow} disabled={accepting}>
							{accepting ? 'Accepting…' : 'Accept SoW'}
						</button>
					</div>
				{/if}
				{#if bothAccepted && isEmployer}
					<div style="padding: 1rem; background: #ecfdf5; border: 1px solid #6ee7b7; border-radius: 6px;">
						<p style="margin: 0 0 0.75rem; color: #065f46; font-weight: 500;">Both parties have accepted the SoW. You can now proceed to payment.</p>
						<button class="btn btn-primary" onclick={proceedToPayment} disabled={proceedLoading}>
							{proceedLoading ? 'Redirecting…' : 'Proceed to Payment'}
						</button>
					</div>
				{/if}
			{/if}
		</div>
	{:else}
		<p style="color: #888; font-size: 0.9rem;">No Statement of Work has been created yet.</p>
		{#if jobStatus === 'SOW_NEGOTIATION'}
			<button class="btn btn-secondary" onclick={startEdit} style="margin-top: 0.5rem; font-size: 0.85rem;">
				Create SoW
			</button>
		{/if}
	{/if}
</div>

<!-- Milestones read-only view (shown outside edit mode when milestones exist) -->
{#if !editing && milestones && milestones.length > 0}
	<div class="card" style="margin-bottom: 1.5rem;">
		<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">Milestones</h2>
		<div style="display: flex; flex-direction: column; gap: 0.75rem;">
			{#each milestones as milestone, i}
				<div class="milestone-row">
					<div class="milestone-header">
						<strong style="font-size: 0.95rem;">Milestone {i + 1}: {milestone.title}</strong>
						<span style="font-size: 0.9rem; color: #555;">${milestone.amount.toFixed(2)}</span>
					</div>
					{#if milestone.deliverables}
						<div style="margin-top: 0.5rem;">
							<p style="font-size: 0.8rem; font-weight: 600; color: #666; margin: 0 0 0.2rem; text-transform: uppercase; letter-spacing: 0.03em;">Deliverables</p>
							<p style="margin: 0; color: #444; font-size: 0.9rem; white-space: pre-wrap;">{milestone.deliverables}</p>
						</div>
					{/if}
					{#if milestone.criteria && milestone.criteria.length > 0}
						<div style="margin-top: 0.5rem;">
							<p style="font-size: 0.8rem; font-weight: 600; color: #666; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.03em;">Acceptance Criteria</p>
							<ul style="margin: 0; padding-left: 1.25rem; font-size: 0.88rem; color: #555;">
								{#each milestone.criteria as criterion}
									<li style="margin-bottom: 0.2rem;">{criterion.description}</li>
								{/each}
							</ul>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	</div>
{/if}
