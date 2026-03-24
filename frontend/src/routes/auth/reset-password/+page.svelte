<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';

	let token = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let loading = $state(false);
	let error = $state('');

	onMount(() => {
		token = $page.url.searchParams.get('token') ?? '';
		if (!token) {
			error = 'Invalid or missing reset token. Please request a new password reset.';
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (newPassword !== confirmPassword) {
			error = 'Passwords do not match';
			return;
		}
		if (newPassword.length < 8) {
			error = 'Password must be at least 8 characters';
			return;
		}
		loading = true;
		try {
			const res = await fetch('/api/ui/auth/reset-password', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token, new_password: newPassword })
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({ error: 'Reset failed' }));
				throw new Error(data.error || 'Reset failed');
			}
			goto('/auth/login');
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Something went wrong';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Reset Password — {SITE_NAME}</title>
</svelte:head>

<div class="auth-wrap">
	<h1>Reset password</h1>
	<div class="auth-card">
		{#if error && !token}
			<div class="alert alert-error">{error}</div>
			<p style="margin-top: 1rem; text-align: center;">
				<a href="/auth/forgot-password">Request a new reset link</a>
			</p>
		{:else}
			{#if error}
				<div class="alert alert-error">{error}</div>
			{/if}
			<form onsubmit={handleSubmit}>
				<div class="form-group">
					<label for="new-password">New password</label>
					<input id="new-password" type="password" bind:value={newPassword} required autocomplete="new-password" placeholder="At least 8 characters" />
				</div>
				<div class="form-group">
					<label for="confirm-password">Confirm new password</label>
					<input id="confirm-password" type="password" bind:value={confirmPassword} required autocomplete="new-password" placeholder="Repeat your new password" />
				</div>
				<button type="submit" class="btn btn-primary" style="width: 100%; margin-top: 0.5rem;" disabled={loading || !token}>
					{loading ? 'Resetting…' : 'Reset password'}
				</button>
			</form>
		{/if}
	</div>
	<p class="auth-footer">
		<a href="/auth/login">Back to sign in</a>
	</p>
</div>
