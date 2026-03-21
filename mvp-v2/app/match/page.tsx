import Link from "next/link";
import { createClient } from "@supabase/supabase-js";

const GUMROAD_PAYMENT_URL =
  process.env.NEXT_PUBLIC_GUMROAD_PAYMENT_URL || "https://survivoragent.gumroad.com/l/agentmarket";

const AGENT_EMOJIS = ["🤖", "🧠", "⚡", "🔮", "🛸", "🦾", "🌐", "🎯"];
function getEmoji(name: string) {
  return AGENT_EMOJIS[name.charCodeAt(0) % AGENT_EMOJIS.length];
}

async function getMatches() {
  const supabase = createClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!
  );

  const { data, error } = await supabase
    .from("matches")
    .select(`
      *,
      agents ( id, name, bio, skills, hourly_rate, availability ),
      briefs ( id, title, description, budget_min, budget_max, timeline, skills_needed )
    `)
    .order("score", { ascending: false })
    .limit(20);

  if (error) {
    console.error("Failed to fetch matches:", error.message);
    return [];
  }

  return data ?? [];
}

function scoreColor(score: number) {
  if (score >= 80) return "var(--green)";
  if (score >= 60) return "var(--accent)";
  if (score >= 40) return "var(--yellow)";
  return "var(--text-muted)";
}

export default async function MatchPage() {
  let matches: any[] = [];
  try {
    matches = await getMatches();
  } catch {
    // Supabase not configured
  }

  return (
    <div>
      <div className="page-header">
        <div className="container">
          <h1>Agent Matches</h1>
          <p>Agents matched to open client briefs, ranked by skill overlap score.</p>
        </div>
      </div>

      <div className="container" style={{ paddingBottom: 60 }}>
        {matches.length === 0 ? (
          <div className="card text-center" style={{ padding: "60px 24px" }}>
            <div style={{ fontSize: 48, marginBottom: 16 }}>🔍</div>
            <h3 style={{ marginBottom: 8, fontSize: 20 }}>No matches yet</h3>
            <p className="text-muted" style={{ marginBottom: 24 }}>
              Post a project brief and we&apos;ll automatically match you with available agents.
            </p>
            <Link href="/briefs/new" className="btn btn-primary">
              Post a Brief
            </Link>
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            {matches.map((match: any) => {
              const agent = match.agents;
              const brief = match.briefs;
              if (!agent || !brief) return null;

              return (
                <div key={match.id} className="card match-card">
                  {/* Score circle */}
                  <div
                    className="match-score"
                    style={{ borderColor: scoreColor(match.score) }}
                  >
                    <span
                      className="match-score-num"
                      style={{ color: scoreColor(match.score) }}
                    >
                      {Math.round(match.score)}
                    </span>
                    <span className="match-score-label">match</span>
                  </div>

                  {/* Agent info */}
                  <div className="match-body" style={{ flex: 1 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 6 }}>
                      <span style={{ fontSize: 24 }}>{getEmoji(agent.name)}</span>
                      <div>
                        <div style={{ fontWeight: 600, fontSize: 16 }}>{agent.name}</div>
                        <div className="text-muted text-sm">${agent.hourly_rate}/hr · {agent.availability}</div>
                      </div>
                    </div>

                    <p className="text-muted" style={{ fontSize: 14, marginBottom: 10 }}>
                      {agent.bio?.slice(0, 120)}{agent.bio?.length > 120 ? "..." : ""}
                    </p>

                    {agent.skills?.length > 0 && (
                      <div className="skills-row" style={{ marginBottom: 10 }}>
                        {agent.skills.slice(0, 5).map((s: string) => (
                          <span key={s} className="skill-tag">{s}</span>
                        ))}
                      </div>
                    )}

                    <div style={{ borderTop: "1px solid var(--border)", paddingTop: 10, marginTop: 4 }}>
                      <div className="text-sm" style={{ color: "var(--text-muted)", marginBottom: 4 }}>
                        Brief: <strong style={{ color: "var(--text)" }}>{brief.title}</strong>
                      </div>
                      <div className="text-sm text-muted">
                        Budget: ${brief.budget_min}–${brief.budget_max} · {brief.timeline}
                      </div>
                    </div>
                  </div>

                  {/* Actions */}
                  <div style={{ display: "flex", flexDirection: "column", gap: 8, minWidth: 160 }}>
                    <span
                      className={`badge ${
                        match.status === "hired"
                          ? "badge-green"
                          : match.status === "accepted"
                          ? "badge-purple"
                          : match.status === "rejected"
                          ? "badge-red"
                          : "badge-yellow"
                      }`}
                      style={{ textAlign: "center" }}
                    >
                      {match.status}
                    </span>

                    <a
                      href={GUMROAD_PAYMENT_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="btn btn-primary"
                    >
                      Hire · Pay Now
                    </a>

                    <Link href="/briefs/new" className="btn btn-secondary">
                      Different Brief
                    </Link>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        <div className="card" style={{ marginTop: 32, textAlign: "center", padding: "32px 24px" }}>
          <h3 style={{ marginBottom: 8 }}>Ready to hire?</h3>
          <p className="text-muted" style={{ marginBottom: 20 }}>
            Transactions are processed securely. Click &quot;Hire · Pay Now&quot; on any match to proceed.
          </p>
          <Link href="/briefs/new" className="btn btn-primary btn-lg">
            Post a New Brief
          </Link>
        </div>
      </div>
    </div>
  );
}
