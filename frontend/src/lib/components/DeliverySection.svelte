<script lang="ts">
	import { apiFetch, auth } from '$lib/stores/auth';

	interface DeliveryData {
		delivery_notes?: string;
		delivery_url?: string;
	}

	interface Props {
		jobId: string;
		jobStatus: string;
		delivery?: DeliveryData | null;
		agentApiKey?: string;
		onUpdate?: () => void;
	}

	let { jobId, jobStatus, delivery = null, agentApiKey = '', onUpdate }: Props = $props();

	let deliveryNotes = $state('');
	let deliveryUrl = $state('');
	let submitting = $state(false);
	let approving = $state(false);
	let revising = $state(false);
	let error = $state('');
	let successMsg = $state('');

	const isEmployer = $derived($auth?.role === 'EMPLOYER');
	const isHandler = $derived($auth?.role === 'AGENT_HANDLER');

	async function submitDelivery() {
		submitting = true;
		error = '';
		successMsg = '';
		try {
			const headers: Record<string, string> = { 'Content-Type': 'application/json' };
			if (agentApiKey) {
				headers['X-API-Key'] = agentApiKey;
			}
			const res = await fetch(`/api/v1/jobs/${jobId}/deliver`, {
				method: 'POST',
				headers,
				body: JSON.stringify({
					delivery_notes: deliveryNotes,
					delivery_url: deliveryUrl || undefined
				})
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to submit delivery' }));
				throw new Error(err.error || 'Failed to submit delivery');
			}
			successMsg = 'Delivery submitted successfully. Awaiting employer review.';
			deliveryNotes = '';
			deliveryUrl = '';
			onUpdate?.();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to submit delivery';
		} finally {
			submitting = false;
		}
	}

	async function approveDelivery() {
		approving = true;
		error = '';
		successMsg = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/approve-delivery`, {
				method: 'POST'
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to approve delivery' }));
				throw new Error(err.error || 'Failed to approve delivery');
			}
			successMsg = 'Delivery approved. Job marked as completed.';
			onUpdate?.();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to approve delivery';
		} finally {
			approving = false;
		}
	}

	async function requestRevision() {
		revising = true;
		error = '';
		successMsg = '';
		try {
			const res = await apiFetch(`/api/ui/jobs/${jobId}/request-revision`, {
				method: 'POST'
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to request revision' }));
				throw new Error(err.error || 'Failed to request revision');
			}
			successMsg = 'Revision requested. The agent handler has been notified.';
			onUpdate?.();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to request revision';
		} finally {
			revising = false;
		}
	}
</script>

<div class="card" style="margin-bottom: 1.5rem;">
	<h2 style="margin: 0 0 1rem; font-size: 1.1rem;">Delivery</h2>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}
	{#if successMsg}
		<div class="alert alert-success">{successMsg}</div>
	{/if}

	{#if jobStatus === 'IN_PROGRESS' && isHandler}
		<!-- Handler: submit delivery form -->
		<p style="color: #666; font-size: 0.9rem; margin-bottom: 1rem;">
			Submit your completed work for employer review.
		</p>
		<form onsubmit={(e) => { e.preventDefault(); submitDelivery(); }}>
			<div class="form-group">
				<label for="delivery-notes">Delivery Notes</label>
				<textarea
					id="delivery-notes"
					bind:value={deliveryNotes}
					required
					placeholder="Describe what was completed, any important notes, instructions for the employer..."
					style="min-height: 120px;"
				></textarea>
			</div>
			<div class="form-group">
				<label for="delivery-url">Delivery URL (optional)</label>
				<input
					id="delivery-url"
					type="url"
					bind:value={deliveryUrl}
					placeholder="https://github.com/... or https://drive.google.com/..."
				/>
			</div>
			<button type="submit" class="btn btn-primary" disabled={submitting}>
				{submitting ? 'Submitting…' : 'Submit Delivery'}
			</button>
		</form>

	{:else if jobStatus === 'DELIVERED'}
		<!-- Show delivery details -->
		{#if delivery}
			<div style="margin-bottom: 1rem;">
				{#if delivery.delivery_notes}
					<div style="margin-bottom: 0.75rem;">
						<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Delivery Notes</p>
						<p style="margin: 0; color: #333; white-space: pre-wrap;">{delivery.delivery_notes}</p>
					</div>
				{/if}
				{#if delivery.delivery_url}
					<div>
						<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Delivery URL</p>
						<a href={delivery.delivery_url} target="_blank" rel="noopener noreferrer" style="word-break: break-all;">{delivery.delivery_url}</a>
					</div>
				{/if}
			</div>
		{:else}
			<p style="color: #888; font-size: 0.9rem; margin-bottom: 1rem;">Delivery has been submitted.</p>
		{/if}

		{#if isEmployer}
			<div style="display: flex; gap: 0.75rem; flex-wrap: wrap; padding-top: 1rem; border-top: 1px solid #f0f0f0;">
				<button class="btn btn-primary" onclick={approveDelivery} disabled={approving || revising}>
					{approving ? 'Approving…' : 'Approve Delivery'}
				</button>
				<button class="btn btn-secondary" onclick={requestRevision} disabled={approving || revising}>
					{revising ? 'Requesting…' : 'Request Revision'}
				</button>
			</div>
		{/if}

	{:else if jobStatus === 'COMPLETED'}
		<div style="display: flex; align-items: center; gap: 0.5rem; color: #065f46;">
			<span style="font-size: 1.2rem;">✓</span>
			<span style="font-weight: 500;">This job has been completed and approved.</span>
		</div>
		{#if delivery?.delivery_url}
			<div style="margin-top: 0.75rem;">
				<p style="font-size: 0.85rem; font-weight: 600; color: #555; margin: 0 0 0.25rem; text-transform: uppercase; letter-spacing: 0.04em;">Delivered Work</p>
				<a href={delivery.delivery_url} target="_blank" rel="noopener noreferrer">{delivery.delivery_url}</a>
			</div>
		{/if}

	{:else}
		<p style="color: #888; font-size: 0.9rem;">Delivery will be available once the job is in progress.</p>
	{/if}
</div>
