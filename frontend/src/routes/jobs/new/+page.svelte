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

	let title = $state('');
	let description = $state('');
	let payout = $state(0);
	let timeline = $state('');
	let sowLink = $state('');

	let submitting = $state(false);
	let error = $state('');

	// Where to go back after saving (agent_id passed as query param to trigger assign flow)
	const returnTo = $derived($page.url.searchParams.get('return_to') ?? '/dashboard/employer');

	onMount(() => {
		if (!$isAuthenticated) {
			goto('/auth/login');
			return;
		}
		if ($auth?.role !== 'EMPLOYER') {
			goto('/dashboard/employer');
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		submitting = true;
		try {
			const payload = {
				agent_id: '',
				title,
				description,
				total_payout: Math.round(Number(payout)),
				timeline_days: Math.round(Number(timeline)) || 0,
				sow_link: sowLink,
				milestones: []
			};
			const res = await apiFetch('/api/ui/jobs/hire', {
				method: 'POST',
				body: JSON.stringify(payload)
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to create job' }));
				throw new Error(err.error || 'Failed to create job');
			}
			goto(returnTo);
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to submit job';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Enter a Job Brief — {SITE_NAME}</title>
</svelte:head>

<div class="container page">
	<div style="margin-bottom: 1rem;">
		<a href={returnTo} style="color: #888; font-size: 0.9rem;">← Back</a>
	</div>

	<div class="page-header">
		<h1>Enter a Job Brief</h1>
		<p>Describe the work at a high level. Milestones and detailed specs are negotiated with the agent via the SoW after assignment.</p>
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
				<textarea id="description" bind:value={description} required placeholder="What do you need done? Keep it brief — detailed specs go in the SoW." rows={MIN_ROWS} use:steppedResize></textarea>
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
			<h2 style="margin: 0 0 0.4rem; font-size: 1.1rem;">Statement of Work</h2>
			<p style="margin: 0 0 1rem; font-size: 0.9rem; color: #666;">
				You'll negotiate detailed specs, deliverables, and milestones with the agent once they're assigned. Optionally link an existing SoW doc to get things started.
			</p>
			<div class="form-group" style="margin-bottom: 0;">
				<label for="sow-link">SoW document URL <span style="font-weight: normal; color: #888;">(optional)</span></label>
				<input id="sow-link" type="url" bind:value={sowLink} placeholder="https://docs.google.com/..." />
			</div>
		</div>

		<div style="display: flex; gap: 1rem;">
			<button type="submit" class="btn btn-primary" disabled={submitting}>
				{submitting ? 'Saving…' : 'Save Job Brief'}
			</button>
			<a href={returnTo} class="btn btn-secondary">Cancel</a>
		</div>
	</form>
</div>
