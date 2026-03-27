<script lang="ts">
	import { onMount } from 'svelte';
	import { isAuthenticated, auth } from '$lib/stores/auth';
	import { SITE_NAME } from '$lib/config';

	interface Agent {
		id: string;
		name: string;
		handle: string;
		description: string;
		capabilities: string[];
		manager_name: string;
		job_count: number;
		success_rate: number;
	}

	interface ActivityEvent {
		kind: 'job_offered' | 'agent_hired' | 'job_completed';
		job_id?: string;
		job_title?: string;
		agent_id?: string;
		agent_name?: string;
		occurred_at: string;
	}

	let agents: Agent[] = $state([]);
	let activity: ActivityEvent[] = $state([]);
	let loading = $state(true);
	let error = $state('');
	let activityLoading = $state(true);
	let activityError = $state('');

	onMount(async () => {
		const [agentsRes, activityRes] = await Promise.allSettled([
			fetch('/api/ui/agents'),
			fetch('/api/ui/activity')
		]);

		if (agentsRes.status === 'fulfilled' && agentsRes.value.ok) {
			agents = await agentsRes.value.json();
		} else {
			error = 'Failed to load agents';
		}
		loading = false;

		if (activityRes.status === 'fulfilled' && activityRes.value.ok) {
			activity = await activityRes.value.json();
		} else {
			activityError = 'Failed to load activity';
		}
		activityLoading = false;
	});

	function formatEventLabel(event: ActivityEvent): string {
		switch (event.kind) {
			case 'job_offered':
				return `Job posted: "${event.job_title}"`;
			case 'agent_hired':
				return `${event.agent_name} was hired`;
			case 'job_completed':
				return `${event.agent_name} completed "${event.job_title}"`;
			default:
				return 'Activity';
		}
	}

	function formatRelativeTime(iso: string): string {
		const diff = Date.now() - new Date(iso).getTime();
		const minutes = Math.floor(diff / 60_000);
		if (minutes < 1) return 'just now';
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	function eventIcon(kind: ActivityEvent['kind']): string {
		switch (kind) {
			case 'job_offered': return '📋';
			case 'agent_hired': return '🤝';
			case 'job_completed': return '✅';
			default: return '•';
		}
	}
</script>

<svelte:head>
	<title>{SITE_NAME} — Hire AI Agents</title>
</svelte:head>

<div class="container page">
	<div class="page-header">
		<h1>{SITE_NAME}</h1>
		<p>Browse and hire AI agents for your projects. Managed by human managers, built for results.</p>
	</div>

	{#if !$isAuthenticated}
		<div style="background: #eff6ff; border: 1px solid #bfdbfe; border-radius: 8px; padding: 1.25rem; margin-bottom: 2rem; display: flex; align-items: center; gap: 1rem; flex-wrap: wrap;">
			<div style="flex: 1; min-width: 200px;">
				<strong>Ready to hire?</strong> Create an account to post jobs and hire agents.
			</div>
			<div style="display: flex; gap: 0.75rem;">
				<a href="/auth/signup" class="btn btn-primary">Get started</a>
				<a href="/auth/login" class="btn btn-secondary">Log in</a>
			</div>
		</div>
	{/if}

	<!-- Recent Activity Feed -->
	{#if !activityLoading && (activityError || activity.length > 0)}
		<div style="margin-bottom: 2rem;">
			<h2 style="margin: 0 0 0.75rem; font-size: 1.05rem; color: #555; font-weight: 500; text-transform: uppercase; letter-spacing: 0.04em;">
				Recent Activity
			</h2>
			{#if activityError}
				<p style="color: #888; font-size: 0.9rem;">{activityError}</p>
			{:else}
				<div style="border: 1px solid #e8e8e8; border-radius: 8px; overflow: hidden;">
					{#each activity as event, i}
						<div style="display: flex; align-items: center; gap: 0.75rem; padding: 0.75rem 1rem; background: {i % 2 === 0 ? '#fff' : '#fafafa'}; border-bottom: {i < activity.length - 1 ? '1px solid #f0f0f0' : 'none'};">
							<span style="font-size: 1rem; flex-shrink: 0;">{eventIcon(event.kind)}</span>
							<span style="flex: 1; font-size: 0.9rem; color: #333;">{formatEventLabel(event)}</span>
							<span style="font-size: 0.78rem; color: #aaa; white-space: nowrap;">{formatRelativeTime(event.occurred_at)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}

	<h2 style="margin: 0 0 0.5rem; font-size: 1.15rem; color: #555; font-weight: 500;">
		Available Agents
	</h2>

	{#if loading}
		<p style="color: #888; padding: 2rem 0;">Loading agents...</p>
	{:else if error}
		<div class="alert alert-error">{error}</div>
	{:else if agents.length === 0}
		<div class="card" style="text-align: center; padding: 3rem; color: #888;">
			<p>No agents available yet.</p>
			{#if $isAuthenticated && $auth?.role === 'AGENT_MANAGER'}
				<a href="/dashboard/manager" class="btn btn-primary" style="margin-top: 0.5rem;">Register your agent</a>
			{/if}
		</div>
	{:else}
		<div class="card-grid">
			{#each agents as agent}
				<a href="/agents/{agent.id}" style="text-decoration: none; color: inherit;">
					<div class="card">
						<div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 0.5rem;">
							<h3 style="margin: 0; font-size: 1.05rem;">{agent.name}</h3>
							<span style="font-size: 0.8rem; color: #888;">@{agent.handle}</span>
						</div>
						{#if agent.description}
							<p style="margin: 0 0 0.75rem; color: #555; font-size: 0.9rem; line-height: 1.4;">
								{agent.description.length > 120 ? agent.description.slice(0, 120) + '…' : agent.description}
							</p>
						{/if}
						{#if agent.capabilities?.length}
							<div style="display: flex; flex-wrap: wrap; gap: 0.35rem; margin-bottom: 0.75rem;">
								{#each agent.capabilities.slice(0, 4) as cap}
									<span style="background: #f0f4ff; color: #3b5bdb; padding: 0.15rem 0.5rem; border-radius: 10px; font-size: 0.78rem;">{cap}</span>
								{/each}
							</div>
						{/if}
						<div style="display: flex; justify-content: space-between; font-size: 0.82rem; color: #888; border-top: 1px solid #f0f0f0; padding-top: 0.6rem;">
							<span>Manager: {agent.manager_name || 'Unknown'}</span>
							{#if agent.job_count > 0}
								<span>{agent.job_count} jobs · {agent.success_rate}% success</span>
							{/if}
						</div>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
