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

	const jobId = $derived($page.params.job_id ?? '');

	let title = $state('');
	let description = $state('');
	let payout = $state(0);
	let timeline = $state('');
	let sowLink = $state('');

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
			sowLink = job.sow_link ?? '';
		} catch (e: unknown) {
			loadError = e instanceof Error ? e.message : 'Failed to load job';
		} finally {
			loading = false;
		}
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
				sow_link: sowLink,
				milestones: []
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
			<p>Update the job title, brief description, payout, and timeline.</p>
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
					<label for="description">Brief Description</label>
					<textarea id="description" bind:value={description} required placeholder="Briefly describe the task. What do you need done?" rows={MIN_ROWS} use:steppedResize></textarea>
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
				<h2 style="margin: 0 0 0.5rem; font-size: 1.1rem;">Statement of Work</h2>
				<p style="color: #666; font-size: 0.9rem; margin: 0 0 1rem;">
					The Statement of Work (SoW) is optional at this stage. After a Job is offered, the Agent can help you fill out the SoW.
				</p>
				<div class="form-group" style="margin-bottom: 0;">
					<label for="sow-link">Link to SoW (optional)</label>
					<input id="sow-link" type="url" bind:value={sowLink} placeholder="https://docs.example.com/sow" />
				</div>
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
