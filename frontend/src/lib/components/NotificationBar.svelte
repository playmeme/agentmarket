<script lang="ts">
	import { apiFetch } from '$lib/stores/auth';

	interface Notification {
		id: string;
		user_id: string;
		job_id?: string;
		type: string;
		title: string;
		message: string;
		read: boolean;
		dismissed: boolean;
		created_at: string;
	}

	interface Props {
		notifications: Notification[];
		onDismiss?: (id: string) => void;
	}

	let { notifications = [], onDismiss }: Props = $props();

	async function dismiss(id: string) {
		try {
			await apiFetch(`/api/ui/notifications/${id}/dismiss`, { method: 'POST' });
			if (onDismiss) onDismiss(id);
		} catch {
			// best effort
		}
	}
</script>

{#if notifications.length > 0}
	<div class="notification-bar-list">
		{#each notifications as notif (notif.id)}
			<div class="notification-bar">
				<div class="notification-bar-content">
					<strong>{notif.title}</strong>
					<span class="notification-bar-message">{notif.message}</span>
					{#if notif.job_id}
						<a href="/jobs/{notif.job_id}" class="notification-bar-link">View job</a>
					{/if}
				</div>
				<button
					class="notification-bar-dismiss"
					onclick={() => dismiss(notif.id)}
					aria-label="Dismiss notification"
				>
					&times;
				</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.notification-bar-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		margin-bottom: 1.25rem;
	}

	.notification-bar {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		background: #fce7f3;
		border: 1px solid #f9a8d4;
		border-radius: 6px;
		padding: 0.75rem 1rem;
		gap: 1rem;
	}

	.notification-bar-content {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.4rem;
		font-size: 0.9rem;
	}

	.notification-bar-message {
		color: #555;
	}

	.notification-bar-link {
		color: #0066cc;
		font-size: 0.85rem;
		white-space: nowrap;
	}

	.notification-bar-dismiss {
		background: none;
		border: none;
		cursor: pointer;
		font-size: 1.25rem;
		line-height: 1;
		color: #888;
		padding: 0;
		flex-shrink: 0;
		margin-top: -0.1rem;
	}

	.notification-bar-dismiss:hover {
		color: #333;
	}
</style>
