<script lang="ts">
	import { apiFetch, auth } from '$lib/stores/auth';

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

	interface Props {
		jobId: string;
		sow: SOWData | null;
		jobStatus: string;
		onUpdate?: () => void;
	}

	let { jobId, sow = $bindable(), jobStatus, onUpdate }: Props = $props();

	let editing = $state(false);
	let saving = $state(false);
	let accepting = $state(false);
	let proceedLoading = $state(false);
	let error = $state('');
	let successMsg = $state('');

	// Edit form state
	let editScope = $state('');
	let editDeliverables = $state('');
	let editPriceDollars = $state('');
	let editTimelineDays = $state('');

	function formatCentsToDollars(cents: number): string {
		return (cents / 100).toFixed(2);
	}

	function startEdit() {
		if (sow) {
			editScope = sow.scope;
			editDeliverables = sow.deliverables;
			editPriceDollars = formatCentsToDollars(sow.price_cents);
			editTimelineDays = String(sow.timeline_days);
		} else {
			editScope = '';
			editDeliverables = '';
			editPriceDollars = '';
			editTimelineDays = '';
		}
		editing = true;
		error = '';
	}

	function cancelEdit() {
		editing = false;
		error = '';
	}

	async function saveSow() {
		saving = true;
		error = '';
		successMsg = '';
		try {
			const priceCents = Math.round(parseFloat(editPriceDollars) * 100);
			const res = await apiFetch(`/api/ui/jobs/${jobId}/sow`, {
				method: 'POST',
				body: JSON.stringify({
					scope: editScope,
					deliverables: editDeliverables,
					price_cents: priceCents,
					timeline_days: parseInt(editTimelineDays) || 0
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
			if (data.url) {
				window.location.href = data.url;
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
	const isHandler = $derived($auth?.role === 'AGENT_HANDLER');

	// Check if current user already accepted
	const userAccepted = $derived(
		sow && ((isEmployer && sow.employer_accepted) || (isHandler && sow.handler_accepted))
	);

	const bothAccepted = $derived(sow && sow.employer_accepted && sow.handler_accepted);
</script>

<div class="card" style="margin-bottom: 1.5rem;">
	<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
		<h2 style="margin: 0; font-size: 1.1rem;">Statement of Work</h2>
		{#if !editing && jobStatus === 'SOW_NEGOTIATION'}
			<button class="btn btn-secondary" onclick={startEdit} style="font-size: 0.85rem; padding: 0.35rem 0.9rem;">
				{sow ? 'Edit' : 'Create SoW'}
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
			<div class="form-group">
				<label for="sow-scope">Scope</label>
				<textarea id="sow-scope" bind:value={editScope} required placeholder="Describe the overall scope of work..." style="min-height: 100px;"></textarea>
			</div>
			<div class="form-group">
				<label for="sow-deliverables">Deliverables</label>
				<textarea id="sow-deliverables" bind:value={editDeliverables} required placeholder="List specific deliverables..." style="min-height: 100px;"></textarea>
			</div>
			<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
				<div class="form-group">
					<label for="sow-price">Price (USD)</label>
					<input id="sow-price" type="number" bind:value={editPriceDollars} min="0" step="0.01" required placeholder="0.00" />
				</div>
				<div class="form-group">
					<label for="sow-timeline">Timeline (days)</label>
					<input id="sow-timeline" type="number" bind:value={editTimelineDays} min="1" step="1" placeholder="7" />
				</div>
			</div>
			<div style="display: flex; gap: 0.75rem; margin-top: 0.5rem;">
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
			<div>
				<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Scope</p>
				<p style="margin: 0; color: #333; white-space: pre-wrap;">{sow.scope}</p>
			</div>
			<div>
				<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Deliverables</p>
				<p style="margin: 0; color: #333; white-space: pre-wrap;">{sow.deliverables}</p>
			</div>
			<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
				<div>
					<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Price</p>
					<p style="margin: 0; font-size: 1.1rem; font-weight: 600; color: #1a1a1a;">${formatCentsToDollars(sow.price_cents)}</p>
				</div>
				<div>
					<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Timeline</p>
					<p style="margin: 0; color: #333;">{sow.timeline_days} day{sow.timeline_days !== 1 ? 's' : ''}</p>
				</div>
			</div>

			<!-- Acceptance status -->
			<div style="border-top: 1px solid #f0f0f0; padding-top: 1rem;">
				<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.5rem; text-transform: uppercase; letter-spacing: 0.04em;">Acceptance Status</p>
				<div style="display: flex; gap: 1rem; flex-wrap: wrap;">
					<div style="display: flex; align-items: center; gap: 0.4rem; font-size: 0.9rem;">
						<span style="width: 10px; height: 10px; border-radius: 50%; background: {sow.employer_accepted ? '#10b981' : '#e5e7eb'}; display: inline-block;"></span>
						<span>Employer: {sow.employer_accepted ? 'Accepted' : 'Pending'}</span>
					</div>
					<div style="display: flex; align-items: center; gap: 0.4rem; font-size: 0.9rem;">
						<span style="width: 10px; height: 10px; border-radius: 50%; background: {sow.handler_accepted ? '#10b981' : '#e5e7eb'}; display: inline-block;"></span>
						<span>Handler: {sow.handler_accepted ? 'Accepted' : 'Pending'}</span>
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
