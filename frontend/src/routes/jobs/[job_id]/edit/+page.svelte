<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { SITE_NAME } from '$lib/config';

	// Stepped auto-resize for textareas: grows in ROW_STEP-line increments, not per keystroke.
	const ROW_STEP = 3;
	const MIN_ROWS = 3;

	function steppedResize(node: HTMLTextAreaElement) {
		function resize() {
			// Temporarily shrink to measure natural scroll height
			node.rows = MIN_ROWS;
			const lineHeight = parseFloat(getComputedStyle(node).lineHeight) || 20;
			const paddingV =
				parseFloat(getComputedStyle(node).paddingTop) +
				parseFloat(getComputedStyle(node).paddingBottom);
			const naturalLines = Math.ceil((node.scrollHeight - paddingV) / lineHeight);
			// Round up to next step boundary
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

	interface Milestone {
		id?: string;
		title: string;
		payout: number;
		criteria: string[];
	}

	const jobId = $derived($page.params.job_id ?? '');

	let title = $state('');
	let description = $state('');
	let payout = $state(0);
	let timeline = $state('');
	let milestones: Milestone[] = $state([{ title: '', payout: 0, criteria: [''] }]);

	let loading = $state(true);
	let submitting = $state(false);
	let error = $state('');
	let loadError = $state('');

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/auth/login');
			return;
		}
		if ($auth?.role !== 'EMPLOYER') {
			goto('/dashboard/employer');
			return;
		}
		await loadJob();
	});

	async function loadJob() {
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}`);
			if (!res.ok) {
				if (res.status === 404) throw new Error('Job not found');
				throw new Error('Failed to load job');
			}
			const job = await res.json();

			// Only allow editing if no agent is assigned
			if (job.agent_id && job.agent_id !== '') {
				goto(`/jobs/${jobId}`);
				return;
			}

			title = job.title ?? '';
			description = job.description ?? '';
			payout = job.total_payout ?? 0;
			timeline = job.timeline_days ? String(job.timeline_days) : '';

			if (job.milestones && job.milestones.length > 0) {
				milestones = job.milestones.map((m: { id?: string; title: string; amount: number; criteria?: Array<{ description: string }> }) => ({
					id: m.id,
					title: m.title,
					payout: m.amount,
					criteria: m.criteria && m.criteria.length > 0
						? m.criteria.map((c: { description: string }) => c.description)
						: ['']
				}));
			} else {
				milestones = [{ title: '', payout: 0, criteria: [''] }];
			}
		} catch (e: unknown) {
			loadError = e instanceof Error ? e.message : 'Failed to load job';
		} finally {
			loading = false;
		}
	}

	function addMilestone() {
		milestones = [...milestones, { title: '', payout: 0, criteria: [''] }];
	}

	function removeMilestone(i: number) {
		milestones = milestones.filter((_, idx) => idx !== i);
	}

	function addCriteria(milestoneIdx: number) {
		milestones = milestones.map((m, i) =>
			i === milestoneIdx ? { ...m, criteria: [...m.criteria, ''] } : m
		);
	}

	function removeCriteria(milestoneIdx: number, criteriaIdx: number) {
		milestones = milestones.map((m, i) =>
			i === milestoneIdx
				? { ...m, criteria: m.criteria.filter((_, ci) => ci !== criteriaIdx) }
				: m
		);
	}

	function updateCriteria(milestoneIdx: number, criteriaIdx: number, value: string) {
		milestones = milestones.map((m, i) =>
			i === milestoneIdx
				? { ...m, criteria: m.criteria.map((c, ci) => (ci === criteriaIdx ? value : c)) }
				: m
		);
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		submitting = true;
		try {
			const payload = {
				title,
				description,
				total_payout: Math.round(Number(payout)),
				timeline_days: Math.round(Number(timeline)) || 0,
				milestones: milestones.map((m) => ({
					title: m.title,
					amount: Math.round(Number(m.payout || 0)),
					criteria: m.criteria.filter((c) => c.trim())
				}))
			};
			const res = await apiFetch(`/api/ui/jobs/${jobId}`, {
				method: 'PUT',
				body: JSON.stringify(payload)
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to update job' }));
				throw new Error(err.error || 'Failed to update job');
			}
			goto(`/jobs/${jobId}`);
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to update job';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Edit Job Brief — {SITE_NAME}</title>
</svelte:head>

<div class="container page">
	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading job...</p>
	{:else if loadError}
		<div class="alert alert-error">{loadError}</div>
		<a href="/dashboard/employer" class="btn btn-secondary" style="margin-top: 1rem;">Back to Dashboard</a>
	{:else}
		<div style="margin-bottom: 1rem;">
			<a href="/jobs/{jobId}" style="color: #888; font-size: 0.9rem;">← Back to Job</a>
		</div>

		<div class="page-header">
			<h1>Edit Job Brief</h1>
			<p>Update the job details, milestones, and success criteria.</p>
		</div>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		<form onsubmit={handleSubmit}>
			<div class="card" style="margin-bottom: 1.5rem;">
				<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">Job details</h2>
				<div class="form-group">
					<label for="title">Title</label>
					<input id="title" type="text" bind:value={title} required placeholder="e.g. Build a landing page, Research competitors" />
				</div>
				<div class="form-group">
					<label for="description">Description</label>
					<textarea id="description" bind:value={description} required placeholder="Describe the task in detail. What do you need done? What does success look like?" rows={MIN_ROWS} use:steppedResize></textarea>
				</div>
				<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
					<div class="form-group">
						<label for="payout">Total payout (USD)</label>
						<input id="payout" type="number" bind:value={payout} min="0" step="0.01" required placeholder="0.00" />
					</div>
					<div class="form-group">
						<label for="timeline">Timeline (days)</label>
						<input id="timeline" type="number" bind:value={timeline} min="1" step="1" placeholder="7" />
					</div>
				</div>
			</div>

			<div class="card" style="margin-bottom: 1.5rem;">
				<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
					<h2 style="margin: 0; font-size: 1.1rem;">Milestones</h2>
					<button type="button" class="btn btn-secondary" onclick={addMilestone} style="font-size: 0.85rem; padding: 0.35rem 0.9rem;">
						+ Add milestone
					</button>
				</div>

				{#each milestones as milestone, i}
					<div class="milestone-row">
						<div class="milestone-header">
							<strong style="font-size: 0.9rem;">Milestone {i + 1}</strong>
							{#if milestones.length > 1}
								<button type="button" class="btn btn-danger" onclick={() => removeMilestone(i)} style="font-size: 0.8rem; padding: 0.2rem 0.6rem;">
									Remove
								</button>
							{/if}
						</div>
						<div style="display: grid; grid-template-columns: 1fr auto; gap: 0.75rem; align-items: start;">
							<div class="form-group" style="margin-bottom: 0.5rem;">
								<label for="m-title-{i}">Title</label>
								<input id="m-title-{i}" type="text" bind:value={milestone.title} required placeholder="Milestone title" />
							</div>
							<div class="form-group" style="margin-bottom: 0.5rem;">
								<label for="m-payout-{i}">Payout (USD)</label>
								<input id="m-payout-{i}" type="number" bind:value={milestone.payout} min="0" step="0.01" placeholder="0.00" style="width: 130px;" />
							</div>
						</div>
						<div>
							<p style="font-size: 0.9rem; font-weight: 500; color: #333; margin: 0 0 0.5rem;">
								Acceptance criteria
							</p>
							{#each milestone.criteria as criterion, ci}
								<div style="display: flex; gap: 0.5rem; margin-bottom: 0.4rem; align-items: flex-start;">
									<textarea
										value={criterion}
										oninput={(e) => updateCriteria(i, ci, (e.target as HTMLTextAreaElement).value)}
										placeholder="e.g. All tests pass, Page loads in under 2s"
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

			<div style="display: flex; gap: 1rem;">
				<button type="submit" class="btn btn-primary" disabled={submitting}>
					{submitting ? 'Saving…' : 'Save Changes'}
				</button>
				<a href="/jobs/{jobId}" class="btn btn-secondary">Cancel</a>
			</div>
		</form>
	{/if}
</div>
