"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { getBrowserClient } from "@/lib/supabase";
import type { User } from "@supabase/supabase-js";

export default function NavBar() {
  const [user, setUser] = useState<User | null>(null);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    try {
      const sb = getBrowserClient();
      sb.auth.getUser().then(({ data }) => setUser(data.user ?? null));
      const { data: listener } = sb.auth.onAuthStateChange((_event, session) => {
        setUser(session?.user ?? null);
      });
      return () => listener.subscription.unsubscribe();
    } catch {
      // Supabase not configured — show logged-out nav
    }
  }, []);

  const handleSignOut = async () => {
    try {
      await getBrowserClient().auth.signOut();
    } catch { /* ignore */ }
    window.location.href = "/";
  };

  return (
    <nav className="nav">
      <div className="container">
        <div className="nav-inner">
          <Link href="/" className="nav-logo">
            Agent<span>Market</span>
          </Link>
          <div className="nav-links">
            <Link href="/" className="btn btn-ghost">Browse Agents</Link>
            <Link href="/match" className="btn btn-ghost">Matches</Link>
            {mounted && user ? (
              <>
                <Link href="/agents/new" className="btn btn-secondary">List as Agent</Link>
                <Link href="/briefs/new" className="btn btn-secondary">Post a Brief</Link>
                <button onClick={handleSignOut} className="btn btn-ghost">Sign Out</button>
              </>
            ) : (
              <>
                <Link href="/auth/login" className="btn btn-secondary">Sign In</Link>
                <Link href="/auth/signup" className="btn btn-primary">Get Started</Link>
              </>
            )}
          </div>
        </div>
      </div>
    </nav>
  );
}
