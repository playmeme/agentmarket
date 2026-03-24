<script lang="ts">
	import '../app.css';
	import { auth, isAuthenticated, apiFetch } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { SITE_NAME } from '$lib/config';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';

	let { children } = $props();
	let unreadCount = $state(0);

	function handleLogout() {
		auth.logout();
		unreadCount = 0;
		goto('/');
	}

	async function fetchUnreadCount() {
		if (!$isAuthenticated) return;
		try {
			const res = await apiFetch('/api/ui/notifications/count');
			if (res.ok) {
				const data = await res.json();
				unreadCount = data.count ?? 0;
			}
		} catch {
			// best effort
		}
	}

	onMount(() => {
		fetchUnreadCount();
	});

	// Refresh count when navigating to dashboard pages
	$effect(() => {
		if ($page.url.pathname.startsWith('/dashboard')) {
			fetchUnreadCount();
		}
	});
</script>

<nav>
	<div class="nav-inner">
		<a class="brand" href="/">{SITE_NAME}</a>
		<a href="/">Agents</a>
		{#if $isAuthenticated}
			{#if $auth?.role === 'EMPLOYER'}
				<a href="/dashboard/employer" class="nav-dashboard-link">
					Dashboard
					{#if unreadCount > 0}
						<span class="notif-badge" aria-label="{unreadCount} unread notifications"></span>
					{/if}
				</a>
			{:else if $auth?.role === 'AGENT_HANDLER'}
				<a href="/dashboard/handler" class="nav-dashboard-link">
					Dashboard
					{#if unreadCount > 0}
						<span class="notif-badge" aria-label="{unreadCount} unread notifications"></span>
					{/if}
				</a>
			{/if}
			<a href="/transactions">Transactions</a>
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

<style>
	.nav-dashboard-link {
		position: relative;
		display: inline-flex;
		align-items: center;
	}

	.notif-badge {
		display: inline-block;
		width: 8px;
		height: 8px;
		background: #e53e3e;
		border-radius: 50%;
		position: absolute;
		top: -2px;
		right: -10px;
	}
</style>
