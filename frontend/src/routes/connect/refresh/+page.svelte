<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, isAuthenticated, auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';

	let restarting = $state(false);
	let error = $state('');

	onMount(() => {
		if (!$isAuthenticated) {
			goto('/auth/login');
		}
	});

	async function restartOnboarding() {
		restarting = true;
		error = '';
		try {
			const res = await apiFetch('/api/ui/stripe/connect/onboard', { method: 'POST' });
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Failed to start onboarding' }));
				throw new Error(err.error || 'Failed to start onboarding');
			}
			const data = await res.json();
			if (data.url) {
				window.location.href = data.url;
			} else {
				throw new Error('No onboarding URL returned');
			}
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to restart onboarding';
			restarting = false;
		}
	}
</script>

<svelte:head>
	<title>Onboarding Session Expired — {SITE_NAME}</title>
</svelte:head>

<div class="container page">
	<div class="page-header">
		<h1>Onboarding Session Expired</h1>
		<p>Your Stripe Connect onboarding link has expired or is no longer valid.</p>
	</div>

	<div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem;">
		<p style="margin-bottom: 1.25rem; color: #444;">
			This can happen if the link timed out or you navigated away before completing the process.
			Click below to get a fresh onboarding link and continue.
		</p>

		{#if error}
			<div class="alert alert-error" style="margin-bottom: 1rem;">{error}</div>
		{/if}

		<button
			class="btn btn-primary"
			onclick={restartOnboarding}
			disabled={restarting}
		>
			{restarting ? 'Redirecting…' : 'Restart Onboarding'}
		</button>
	</div>

	<div>
		<a
			href={$auth?.role === 'EMPLOYER' ? '/dashboard/employer' : '/dashboard/manager'}
			class="btn btn-secondary"
		>
			← Back to Dashboard
		</a>
	</div>
</div>
