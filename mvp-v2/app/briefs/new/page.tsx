"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { getBrowserClient } from "@/lib/supabase";

const TIMELINE_OPTIONS = [
  "Less than 1 week",
  "1–2 weeks",
  "2–4 weeks",
  "1–3 months",
  "3–6 months",
  "Ongoing / no deadline",
];

const SKILL_SUGGESTIONS = [
  "copywriting", "seo", "data-analysis", "python", "automation",
  "customer-support", "research", "social-media", "code-review",
  "content-strategy", "email-marketing", "sql", "web-scraping", "translation",
];

export default function NewBriefPage() {
  const router = useRouter();
  const [form, setForm] = useState({
    title: "",
    description: "",
    budget_min: "",
    budget_max: "",
    timeline: TIMELINE_OPTIONS[1],
  });
  const [skills, setSkills] = useState<string[]>([]);
  const [skillInput, setSkillInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const addSkill = (skill: string) => {
    const clean = skill.trim().toLowerCase().replace(/\s+/g, "-");
    if (clean && !skills.includes(clean) && skills.length < 10) {
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

    const budgetMin = parseFloat(form.budget_min) || 0;
    const budgetMax = parseFloat(form.budget_max) || 0;

    if (budgetMax > 0 && budgetMin > budgetMax) {
      setError("Minimum budget cannot exceed maximum budget.");
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

      const { data: brief, error: insertError } = await sb
        .from("briefs")
        .insert({
          client_id: user.id,
          title: form.title,
          description: form.description,
          budget_min: budgetMin,
          budget_max: budgetMax,
          timeline: form.timeline,
          skills_needed: skills,
          status: "open",
        })
        .select()
        .single();

      if (insertError) {
        setError(insertError.message);
        setLoading(false);
        return;
      }

      // Auto-generate matches
      await fetch("/api/matches", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ brief_id: brief.id }),
      });

      router.push("/match");
      router.refresh();
    } catch (err: any) {
      setError(err.message || "Failed to submit. Is Supabase configured?");
      setLoading(false);
    }
  };

  return (
    <div className="form-page">
      <h1>Post a Project Brief</h1>
      <p className="subtitle">
        Describe your project and we&apos;ll match you with the best available agents automatically.
      </p>

      <form onSubmit={handleSubmit} className="card">
        <div className="form-group">
          <label htmlFor="title">Project Title *</label>
          <input
            id="title"
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
            placeholder="e.g. Automate my weekly newsletter content"
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="description">Project Description *</label>
          <textarea
            id="description"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            placeholder="Describe what you need done, what success looks like, and any constraints or requirements."
            rows={5}
            required
          />
        </div>

        <div className="grid-2">
          <div className="form-group">
            <label htmlFor="budget_min">Min Budget (USD)</label>
            <input
              id="budget_min"
              type="number"
              min="0"
              step="1"
              value={form.budget_min}
              onChange={(e) => setForm({ ...form, budget_min: e.target.value })}
              placeholder="e.g. 500"
            />
          </div>
          <div className="form-group">
            <label htmlFor="budget_max">Max Budget (USD)</label>
            <input
              id="budget_max"
              type="number"
              min="0"
              step="1"
              value={form.budget_max}
              onChange={(e) => setForm({ ...form, budget_max: e.target.value })}
              placeholder="e.g. 2000"
            />
          </div>
        </div>

        <div className="form-group">
          <label htmlFor="timeline">Timeline *</label>
          <select
            id="timeline"
            value={form.timeline}
            onChange={(e) => setForm({ ...form, timeline: e.target.value })}
          >
            {TIMELINE_OPTIONS.map((opt) => (
              <option key={opt} value={opt}>{opt}</option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label>Skills Needed</label>
          <input
            value={skillInput}
            onChange={(e) => setSkillInput(e.target.value)}
            onKeyDown={handleSkillKeyDown}
            placeholder="Type a skill and press Enter (e.g. seo, python, copywriting)"
          />
          <span className="hint">Used to match you with the right agents.</span>

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

        {error && <div className="error-msg mb-4">{error}</div>}

        <button
          type="submit"
          className="btn btn-primary btn-lg"
          style={{ width: "100%" }}
          disabled={loading}
        >
          {loading ? "Posting brief & finding matches..." : "Post Brief & Find Agents"}
        </button>
      </form>
    </div>
  );
}
