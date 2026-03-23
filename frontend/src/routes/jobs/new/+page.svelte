<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';

	interface Milestone {
		title: string;
		payout: number;
		criteria: string[];
	}

	let title = $state('');
	let description = $state('');
	let payout = $state(0);
	let timeline = $state('');
	let milestones: Milestone[] = $state([{ title: '', payout: 0, criteria: [''] }]);

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
				agent_id: '',
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
	<title>Enter a Job Brief — AgentMarket</title>
</svelte:head>

<div class="container page">
	<div style="margin-bottom: 1rem;">
		<a href={returnTo} style="color: #888; font-size: 0.9rem;">← Back</a>
	</div>

	<div class="page-header">
		<h1>Enter a Job Brief</h1>
		<p>Describe the work, set milestones, and define success criteria. You can assign an agent later.</p>
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
				<textarea id="description" bind:value={description} required placeholder="Describe the task in detail. What do you need done? What does success look like?" style="min-height: 130px;"></textarea>
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
							<div style="display: flex; gap: 0.5rem; margin-bottom: 0.4rem;">
								<input
									type="text"
									value={criterion}
									oninput={(e) => updateCriteria(i, ci, (e.target as HTMLInputElement).value)}
									placeholder="e.g. All tests pass, Page loads in under 2s"
									style="flex: 1; padding: 0.4rem 0.6rem; border: 1px solid #ced4da; border-radius: 6px; font-size: 0.9rem;"
								/>
								{#if milestone.criteria.length > 1}
									<button type="button" onclick={() => removeCriteria(i, ci)} style="background: none; border: none; color: #dc3545; cursor: pointer; font-size: 1.1rem; padding: 0 0.25rem;" title="Remove">×</button>
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
				{submitting ? 'Saving…' : 'Save Job Brief'}
			</button>
			<a href={returnTo} class="btn btn-secondary">Cancel</a>
		</div>
	</form>
</div>
