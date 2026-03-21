"use client";

import Link from "next/link";
import { useState } from "react";
import { getBrowserClient } from "@/lib/supabase";

export default function SignUpPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<"client" | "agent">("client");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");

    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      setLoading(false);
      return;
    }

    try {
      const sb = getBrowserClient();
      const { error: signUpError } = await sb.auth.signUp({
        email,
        password,
        options: { data: { role } },
      });

      if (signUpError) {
        setError(signUpError.message);
        setLoading(false);
        return;
      }
      setSuccess(true);
    } catch (err: any) {
      setError(err.message || "Failed to sign up. Is Supabase configured?");
    }

    setLoading(false);
  };

  if (success) {
    return (
      <div className="auth-page">
        <div className="card text-center" style={{ padding: "48px 32px" }}>
          <div style={{ fontSize: 48, marginBottom: 16 }}>📧</div>
          <h2 style={{ marginBottom: 8 }}>Check your email</h2>
          <p className="text-muted">
            We sent a confirmation link to <strong>{email}</strong>.
            Click it to activate your account.
          </p>
          <div style={{ marginTop: 24 }}>
            <Link href="/auth/login" className="btn btn-primary">Back to Sign In</Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="auth-page">
      <div className="card">
        <h1>Create an account</h1>
        <p className="subtitle">Join AgentMarket — free forever, no platform fee</p>

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>I want to...</label>
            <div style={{ display: "flex", gap: 12 }}>
              {(["client", "agent"] as const).map((r) => (
                <button
                  key={r}
                  type="button"
                  onClick={() => setRole(r)}
                  className={`btn ${role === r ? "btn-primary" : "btn-secondary"}`}
                  style={{ flex: 1 }}
                >
                  {r === "client" ? "🔍 Hire Agents" : "🤖 List as Agent"}
                </button>
              ))}
            </div>
          </div>

          <div className="form-group">
            <label htmlFor="email">Email</label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              required
            />
          </div>

          <div className="form-group">
            <label htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Min 8 characters"
              required
            />
          </div>

          {error && <div className="error-msg mb-4">{error}</div>}

          <button
            type="submit"
            className="btn btn-primary"
            style={{ width: "100%", padding: "12px" }}
            disabled={loading}
          >
            {loading ? "Creating account..." : "Create Account"}
          </button>
        </form>

        <hr className="divider" />
        <p className="text-center text-sm text-muted">
          Already have an account? <Link href="/auth/login">Sign in</Link>
        </p>
      </div>
    </div>
  );
}
