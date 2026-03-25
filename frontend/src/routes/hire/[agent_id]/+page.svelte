<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
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

	interface Agent {
		id: string;
		handler_id: string;
		name: string;
		description: string;
		is_active: boolean;
	}

	const agentId = $derived($page.params.agent_id);

	let agent: Agent | null = $state(null);
	let agentLoading = $state(true);

	let title = $state('');
	let description = $state('');
	let payout = $state(0);
	let timeline = $state('');
	let sowLink = $state('');

	let submitting = $state(false);
	let error = $state('');

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/auth/login');
			return;
		}
		if ($auth?.role !== 'EMPLOYER') {
			goto('/dashboard/employer');
			return;
		}
		try {
			const res = await fetch(`/api/ui/agents/${agentId}`);
			if (!res.ok) throw new Error('Agent not found');
			agent = await res.json();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load agent';
		} finally {
			agentLoading = false;
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		submitting = true;
		try {
			const payload = {
				agent_id: agentId,
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
			goto('/dashboard/employer');
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to submit job';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Hire {agent?.name ?? 'Agent'} — {SITE_NAME}</title>
</svelte:head>

<div class="container page">
	<div style="margin-bottom: 1rem;">
		{#if agent}
			<a href="/agents/{agentId}" style="color: #888; font-size: 0.9rem;">← {agent.name}</a>
		{:else}
			<a href="/" style="color: #888; font-size: 0.9rem;">← Agents</a>
		{/if}
	</div>

	<div class="page-header">
		<h1>Post a job{agent ? ` for ${agent.name}` : ''}</h1>
		<p>Describe the work at a high level. You can add a detailed Statement of Work together with the Agent after the offer is accepted.</p>
	</div>

	{#if agentLoading}
		<p style="color: #888;">Loading...</p>
	{:else}
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
					{submitting ? 'Posting job…' : 'Post job'}
				</button>
				<a href={agent ? `/agents/${agentId}` : '/'} class="btn btn-secondary">Cancel</a>
			</div>
		</form>
	{/if}
</div>
