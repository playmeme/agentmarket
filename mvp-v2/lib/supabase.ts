import { createClient, SupabaseClient } from '@supabase/supabase-js'

/**
 * Creates a Supabase client using env vars.
 * Safe to call in server components and API routes.
 * Throws a clear error if env vars are missing.
 */
export function createSupabaseClient(): SupabaseClient {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
  if (!url || !key) {
    throw new Error(
      'Supabase is not configured. Copy .env.local.example to .env.local and add your credentials.'
    );
  }
  return createClient(url, key);
}

/**
 * Browser-safe singleton client.
 * Use this in "use client" components.
 */
let _browserClient: SupabaseClient | null = null;

export function getBrowserClient(): SupabaseClient {
  if (typeof window === 'undefined') {
    // During SSR/build, return a dummy that won't be called
    return {} as SupabaseClient;
  }
  if (!_browserClient) {
    _browserClient = createSupabaseClient();
  }
  return _browserClient;
}

// Named export for client components — only safe to USE after mount (not at module-init)
export { createClient };
