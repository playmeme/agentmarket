import { writable, derived, get } from 'svelte/store';

export interface User {
	id: string;
	role: string;
	name: string;
	handle: string;
	email: string;
}

function createAuthStore() {
	const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('auth_user') : null;
	const initial: User | null = stored ? JSON.parse(stored) : null;

	const { subscribe, set } = writable<User | null>(initial);

	return {
		subscribe,
		login: async (email: string, password: string): Promise<void> => {
			const res = await fetch('/api/ui/auth/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email, password })
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Login failed' }));
				throw new Error(err.error || 'Login failed');
			}
			const data = await res.json();
			const user: User = {
				id: data.id,
				role: data.role,
				name: data.name,
				handle: data.handle,
				email: data.email
			};
			localStorage.setItem('auth_user', JSON.stringify(user));
			set(user);
		},
		signup: async (
			name: string,
			handle: string,
			email: string,
			password: string,
			role: string
		): Promise<void> => {
			const res = await fetch('/api/ui/auth/signup', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, handle, email, password, role })
			});
			if (!res.ok) {
				const err = await res.json().catch(() => ({ error: 'Signup failed' }));
				throw new Error(err.error || 'Signup failed');
			}
			const data = await res.json();
			const user: User = {
				id: data.id,
				role: data.role,
				name: data.name,
				handle: data.handle,
				email: data.email
			};
			localStorage.setItem('auth_user', JSON.stringify(user));
			set(user);
		},
		logout: () => {
			// Tell the Go backend to destroy the HttpOnly cookie
			await fetch('/api/ui/auth/logout', { method: 'POST' });

			localStorage.removeItem('auth_user');
			set(null);
		}
	};
}

export const auth = createAuthStore();

export const isAuthenticated = derived(auth, ($auth) => $auth !== null);

// The browser will automatically attach the HttpOnly cookie to every fetch request.
export function apiHeaders(): Record<string, string> {
	return { 'Content-Type': 'application/json' };
}

export async function apiFetch(path: string, options: RequestInit = {}): Promise<Response> {
	return fetch(path, {
		...options,
		headers: {
			...apiHeaders(),
			...(options.headers as Record<string, string> || {})
		}
	});
}
