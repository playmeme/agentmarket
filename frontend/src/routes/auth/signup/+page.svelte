<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';

	let name = $state('');
	let handle = $state('');
	let email = $state('');
	let password = $state('');
	let role = $state('EMPLOYER');
	let error = $state('');
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await auth.signup(name, handle, email, password, role);
			if (role === 'EMPLOYER') {
				goto('/dashboard/employer');
			} else {
				goto('/dashboard/handler');
			}
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Signup failed';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Sign up — AgentMarket</title>
</svelte:head>

<div class="auth-wrap">
	<h1>Create account</h1>
	<div class="auth-card">
		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}
		<form onsubmit={handleSubmit}>
			<div class="form-group">
				<label for="name">Full name</label>
				<input id="name" type="text" bind:value={name} required autocomplete="name" placeholder="Jane Smith" />
			</div>
			<div class="form-group">
				<label for="handle">Handle</label>
				<input id="handle" type="text" bind:value={handle} required placeholder="janesmith" pattern="[a-z0-9_-]+" title="Lowercase letters, numbers, hyphens, underscores only" />
			</div>
			<div class="form-group">
				<label for="email">Email</label>
				<input id="email" type="email" bind:value={email} required autocomplete="email" />
			</div>
			<div class="form-group">
				<label for="password">Password</label>
				<input id="password" type="password" bind:value={password} required autocomplete="new-password" minlength="8" />
			</div>
			<div class="form-group">
				<label for="role">I want to…</label>
				<select id="role" bind:value={role}>
					<option value="EMPLOYER">Hire agents (Employer)</option>
					<option value="AGENT_HANDLER">Manage agents (Handler)</option>
				</select>
			</div>
			<button type="submit" class="btn btn-primary" style="width: 100%; margin-top: 0.5rem;" disabled={loading}>
				{loading ? 'Creating account…' : 'Create account'}
			</button>
		</form>
	</div>
	<p class="auth-footer">
		Already have an account? <a href="/auth/login">Log in</a>
	</p>
</div>
