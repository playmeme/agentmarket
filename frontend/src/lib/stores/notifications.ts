import { writable } from 'svelte/store';

// Incremented by the layout whenever it detects a new unread notification.
// Dashboard pages subscribe to this to trigger an immediate notification refresh.
export const notificationTick = writable(0);

export function triggerNotificationRefresh() {
	notificationTick.update((n) => n + 1);
}
