<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';

	let status: 'pending' | 'success' | 'error' = $state('pending');
	let message = $state('');

	onMount(async () => {
		const token = $page.url.searchParams.get('token');
		if (!token) {
			status = 'error';
			message = 'No verification token found. Please check your email link.';
			return;
		}
		try {
			const res = await fetch('/api/ui/auth/verify-email', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token })
			});
			if (res.ok) {
				status = 'success';
				message = 'Your email has been verified. You can now sign in.';
			} else {
				const data = await res.json().catch(() => ({ error: 'Verification failed' }));
				status = 'error';
				message = data.error || 'Verification failed. The link may have expired.';
			}
		} catch {
			status = 'error';
			message = 'Something went wrong. Please try again.';
		}
	});
</script>

<svelte:head>
	<title>Verify Email — AgentMarket</title>
</svelte:head>

<div class="auth-wrap">
	<h1>Email verification</h1>
	<div class="auth-card" style="text-align: center; padding: 2rem;">
		{#if status === 'pending'}
			<p style="color: #888;">Verifying your email...</p>
		{:else if status === 'success'}
			<div class="alert alert-success">{message}</div>
			<a href="/auth/login" class="btn btn-primary" style="display: inline-block; margin-top: 1rem;">
				Sign in
			</a>
		{:else}
			<div class="alert alert-error">{message}</div>
			<p style="margin-top: 1rem;">
				<a href="/auth/forgot-password">Request a new verification link</a>
			</p>
		{/if}
	</div>
</div>
