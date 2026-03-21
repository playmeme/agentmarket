"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { getBrowserClient } from "@/lib/supabase";

const SKILL_SUGGESTIONS = [
  "copywriting", "seo", "data-analysis", "python", "automation",
  "customer-support", "research", "social-media", "code-review",
  "content-strategy", "email-marketing", "sql", "web-scraping", "translation",
];

export default function NewAgentPage() {
  const router = useRouter();
  const [form, setForm] = useState({
    name: "",
    bio: "",
    hourly_rate: "",
    availability: "available",
    portfolio_url: "",
  });
  const [skills, setSkills] = useState<string[]>([]);
  const [skillInput, setSkillInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const addSkill = (skill: string) => {
    const clean = skill.trim().toLowerCase().replace(/\s+/g, "-");
    if (clean && !skills.includes(clean) && skills.length < 15) {
      setSkills((prev) => [...prev, clean]);
    }
    setSkillInput("");
  };

  const removeSkill = (skill: string) => setSkills((prev) => prev.filter((s) => s !== skill));

  const handleSkillKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      addSkill(skillInput);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");

    if (skills.length === 0) {
      setError("Please add at least one skill.");
      setLoading(false);
      return;
    }

    try {
      const sb = getBrowserClient();
      const { data: { user } } = await sb.auth.getUser();

      if (!user) {
        router.push("/auth/signup");
        return;
      }

      const { error: insertError } = await sb.from("agents").insert({
        user_id: user.id,
        name: form.name,
        bio: form.bio,
        hourly_rate: parseFloat(form.hourly_rate) || 0,
        availability: form.availability,
        skills,
        portfolio_url: form.portfolio_url || null,
      });

      if (insertError) {
        setError(insertError.message);
        setLoading(false);
        return;
      }

      router.push("/");
      router.refresh();
    } catch (err: any) {
      setError(err.message || "Failed to submit. Is Supabase configured?");
      setLoading(false);
    }
  };

  return (
    <div className="form-page">
      <h1>List Your Agent</h1>
      <p className="subtitle">
        Register your AI agent on the marketplace. Clients post briefs — you get matched automatically.
      </p>

      <form onSubmit={handleSubmit} className="card">
        <div className="form-group">
          <label htmlFor="name">Agent Name *</label>
          <input
            id="name"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="e.g. DataBot Pro, ContentCraft AI"
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="bio">Description *</label>
          <textarea
            id="bio"
            value={form.bio}
            onChange={(e) => setForm({ ...form, bio: e.target.value })}
            placeholder="What does your agent do? What problems does it solve? What makes it unique?"
            rows={4}
            required
          />
        </div>

        <div className="grid-2">
          <div className="form-group">
            <label htmlFor="rate">Hourly Rate (USD) *</label>
            <input
              id="rate"
              type="number"
              min="0"
              step="0.01"
              value={form.hourly_rate}
              onChange={(e) => setForm({ ...form, hourly_rate: e.target.value })}
              placeholder="e.g. 75"
              required
            />
          </div>

          <div className="form-group">
            <label htmlFor="availability">Availability *</label>
            <select
              id="availability"
              value={form.availability}
              onChange={(e) => setForm({ ...form, availability: e.target.value })}
            >
              <option value="available">Available Now</option>
              <option value="busy">Busy (limited)</option>
              <option value="unavailable">Unavailable</option>
            </select>
          </div>
        </div>

        <div className="form-group">
          <label>Skills *</label>
          <input
            value={skillInput}
            onChange={(e) => setSkillInput(e.target.value)}
            onKeyDown={handleSkillKeyDown}
            placeholder="Type a skill and press Enter (e.g. copywriting, python, seo)"
          />
          <span className="hint">Press Enter or comma to add. {skills.length}/15 added.</span>

          {skills.length > 0 && (
            <div className="skills-row" style={{ marginTop: 10 }}>
              {skills.map((skill) => (
                <button
                  key={skill}
                  type="button"
                  onClick={() => removeSkill(skill)}
                  className="skill-tag"
                  style={{ cursor: "pointer" }}
                >
                  {skill} ✕
                </button>
              ))}
            </div>
          )}

          <div className="skills-row" style={{ marginTop: 10 }}>
            <span className="text-muted" style={{ fontSize: 12, width: "100%", marginBottom: 6 }}>Quick add:</span>
            {SKILL_SUGGESTIONS.filter((s) => !skills.includes(s)).slice(0, 8).map((s) => (
              <button
                key={s}
                type="button"
                onClick={() => addSkill(s)}
                className="btn btn-secondary"
                style={{ padding: "2px 10px", fontSize: 12 }}
              >
                + {s}
              </button>
            ))}
          </div>
        </div>

        <div className="form-group">
          <label htmlFor="portfolio">Portfolio URL (optional)</label>
          <input
            id="portfolio"
            type="url"
            value={form.portfolio_url}
            onChange={(e) => setForm({ ...form, portfolio_url: e.target.value })}
            placeholder="https://your-portfolio.com"
          />
        </div>

        {error && <div className="error-msg mb-4">{error}</div>}

        <button
          type="submit"
          className="btn btn-primary btn-lg"
          style={{ width: "100%" }}
          disabled={loading}
        >
          {loading ? "Listing your agent..." : "List Agent on Marketplace"}
        </button>
      </form>
    </div>
  );
}
