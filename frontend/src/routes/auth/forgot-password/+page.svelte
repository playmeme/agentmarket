<script lang="ts">
	import { SITE_NAME } from '$lib/config';
	let email = $state('');
	let loading = $state(false);
	let submitted = $state(false);
	let error = $state('');

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			const res = await fetch('/api/ui/auth/forgot-password', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email })
			});
			if (!res.ok) {
				const data = await res.json().catch(() => ({ error: 'Request failed' }));
				throw new Error(data.error || 'Request failed');
			}
			submitted = true;
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Something went wrong';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Forgot Password — {SITE_NAME}</title>
</svelte:head>

<div class="auth-wrap">
	<h1>Forgot password</h1>
	<div class="auth-card">
		{#if submitted}
			<div class="alert alert-success">
				If that email exists, we've sent a reset link.
			</div>
			<p style="margin-top: 1rem; text-align: center; font-size: 0.9rem; color: #666;">
				Check your inbox and follow the link to reset your password.
			</p>
		{:else}
			{#if error}
				<div class="alert alert-error">{error}</div>
			{/if}
			<form onsubmit={handleSubmit}>
				<div class="form-group">
					<label for="email">Email address</label>
					<input id="email" type="email" bind:value={email} required autocomplete="email" placeholder="you@example.com" />
				</div>
				<button type="submit" class="btn btn-primary" style="width: 100%; margin-top: 0.5rem;" disabled={loading}>
					{loading ? 'Sending…' : 'Send reset link'}
				</button>
			</form>
		{/if}
	</div>
	<p class="auth-footer">
		Remember your password? <a href="/auth/login">Sign in</a>
	</p>
</div>
