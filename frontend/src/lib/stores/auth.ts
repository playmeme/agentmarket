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
		logout: async () => {
			// Tell the Go backend to destroy the HttpOnly cookie
			try {
				await fetch('/api/ui/auth/logout', { method: 'POST' });
			} catch (e) {
				console.error("Logout request failed, clearing local state anyway", e);
			}
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
	// Attempt the initial request (with current cookie token)
	return fetch(path, {
		...options,
		headers: {
			...apiHeaders(),
			...(options.headers as Record<string, string> || {})
		}
	});

	// If it's a 401, the 15-minute JWT likely expired
	if (res.status === 401) {
		console.log("DETECTED 401 in apiFetch for:", path);

		// To avoid infinite loops, check a custom flag that gets set on the retry
		const isRetry = (options as any)._isRetry;

		if (!isRetry) {
			console.log('Access token expired, attempting silent refresh...');

			// Call the Go refresh endpoint
			// The browser automatically sends the 30-day "refresh" cookie here
			const refreshRes = await fetch('/api/ui/auth/refresh', { method: 'POST' });

			if (refreshRes.ok) {
				// Refresh succeeded! Retry original request
				return fetch(path, {
					...options,
					headers: {
						...apiHeaders(),
						...(options.headers as Record<string, string> || {})
					},
					// Mark this as a retry so we don't loop if the user is truly unauthorized
					...({ _isRetry: true } as any)
				});
			} else {
				// Refresh failed (30-day token expired or revoked)
				console.warn('Refresh token expired. Logging out.');
				auth.logout(); 
				// Optional: window.location.href = '/auth/login';
			}
		}
	}
	return res;
}

