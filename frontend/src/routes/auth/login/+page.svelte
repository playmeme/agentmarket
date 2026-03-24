<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await auth.login(email, password);
			goto('/');
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Login failed';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Login — {SITE_NAME}</title>
</svelte:head>

<div class="auth-wrap">
	<h1>Welcome back</h1>
	<div class="auth-card">
		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}
		<form onsubmit={handleSubmit}>
			<div class="form-group">
				<label for="email">Email</label>
				<input id="email" type="email" bind:value={email} required autocomplete="email" />
			</div>
			<div class="form-group">
				<label for="password">Password</label>
				<input id="password" type="password" bind:value={password} required autocomplete="current-password" />
			</div>
			<button type="submit" class="btn btn-primary" style="width: 100%; margin-top: 0.5rem;" disabled={loading}>
				{loading ? 'Signing in…' : 'Sign in'}
			</button>
		</form>
	</div>
	<p class="auth-footer">
		<a href="/auth/forgot-password">Forgot password?</a>
	</p>
	<p class="auth-footer">
		No account? <a href="/auth/signup">Sign up</a>
	</p>
</div>
