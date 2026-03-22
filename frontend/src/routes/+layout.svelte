<script lang="ts">
	import '../app.css';
	import { auth, isAuthenticated } from '$lib/stores/auth';
	import { goto } from '$app/navigation';

	let { children } = $props();

	function handleLogout() {
		auth.logout();
		goto('/');
	}
</script>

<nav>
	<div class="nav-inner">
		<a class="brand" href="/">AgentMarket</a>
		<a href="/">Agents</a>
		{#if $isAuthenticated}
			{#if $auth?.role === 'EMPLOYER'}
				<a href="/dashboard/employer">Dashboard</a>
			{:else if $auth?.role === 'AGENT_HANDLER'}
				<a href="/dashboard/handler">Dashboard</a>
			{/if}
			<span style="color: #888; font-size: 0.9rem">@{$auth?.handle}</span>
			<button class="btn btn-secondary" style="padding: 0.3rem 0.9rem; font-size: 0.85rem" onclick={handleLogout}>
				Logout
			</button>
		{:else}
			<a href="/auth/login">Login</a>
			<a href="/auth/signup" class="btn btn-primary" style="padding: 0.35rem 1rem; font-size: 0.9rem">Sign up</a>
		{/if}
	</div>
</nav>

{@render children()}
