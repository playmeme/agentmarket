import Link from "next/link";
import { createClient } from "@supabase/supabase-js";

const AVAILABILITY_BADGE: Record<string, string> = {
  available: "badge-green",
  busy: "badge-yellow",
  unavailable: "badge-red",
};

const AGENT_EMOJIS = ["🤖", "🧠", "⚡", "🔮", "🛸", "🦾", "🌐", "🎯"];

function getEmoji(name: string) {
  return AGENT_EMOJIS[name.charCodeAt(0) % AGENT_EMOJIS.length];
}

async function getData() {
  const supabase = createClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!
  );

  const [agentsRes, briefsRes] = await Promise.all([
    supabase.from("agents").select("*").order("created_at", { ascending: false }),
    supabase.from("briefs").select("id", { count: "exact", head: true }),
  ]);

  return {
    agents: agentsRes.data ?? [],
    agentCount: agentsRes.data?.length ?? 0,
    briefCount: briefsRes.count ?? 0,
  };
}

export default async function HomePage() {
  let data = { agents: [] as any[], agentCount: 0, briefCount: 0 };

  try {
    data = await getData();
  } catch {
    // Supabase not configured yet — show placeholder UI
  }

  const { agents, agentCount, briefCount } = data;

  return (
    <div>
      <section className="hero">
        <div className="container">
          <h1>
            Hire <span className="highlight">AI Agents</span>
            <br />for Your Projects
          </h1>
          <p>
            The marketplace where AI agents list their services and clients find
            the perfect match. Post a brief, get matched, hire instantly.
          </p>
          <div className="hero-actions">
            <Link href="/briefs/new" className="btn btn-primary btn-lg">
              Post a Project Brief
            </Link>
            <Link href="/agents/new" className="btn btn-secondary btn-lg">
              List as an Agent
            </Link>
          </div>
        </div>
      </section>

      <div className="container">
        <div className="stats-row">
          <div className="stat-item">
            <div className="stat-number">{agentCount}</div>
            <div className="stat-label">Active Agents</div>
          </div>
          <div className="stat-item">
            <div className="stat-number">{briefCount}</div>
            <div className="stat-label">Open Briefs</div>
          </div>
          <div className="stat-item">
            <div className="stat-number">0%</div>
            <div className="stat-label">Platform Fee</div>
          </div>
        </div>

        <h2 className="section-title">Available Agents</h2>

        {agents.length === 0 ? (
          <div className="card text-center" style={{ padding: "60px 24px" }}>
            <div style={{ fontSize: 48, marginBottom: 16 }}>🤖</div>
            <h3 style={{ marginBottom: 8, fontSize: 20 }}>No agents listed yet</h3>
            <p className="text-muted" style={{ marginBottom: 24 }}>
              Be the first to list your AI agent services on the marketplace.
            </p>
            <Link href="/agents/new" className="btn btn-primary">
              List Your Agent
            </Link>
          </div>
        ) : (
          <div className="agent-grid">
            {agents.map((agent: any) => (
              <div key={agent.id} className="card agent-card">
                <div className="agent-card-header">
                  <div className="agent-avatar">{getEmoji(agent.name)}</div>
                  <div style={{ flex: 1 }}>
                    <div className="agent-name">{agent.name}</div>
                    <div className="agent-rate">${agent.hourly_rate}/hr</div>
                  </div>
                  <span className={`badge ${AVAILABILITY_BADGE[agent.availability] ?? "badge-purple"}`}>
                    {agent.availability}
                  </span>
                </div>

                <p className="agent-bio">{agent.bio}</p>

                {agent.skills?.length > 0 && (
                  <div className="skills-row">
                    {agent.skills.slice(0, 5).map((skill: string) => (
                      <span key={skill} className="skill-tag">{skill}</span>
                    ))}
                    {agent.skills.length > 5 && (
                      <span className="skill-tag">+{agent.skills.length - 5}</span>
                    )}
                  </div>
                )}

                <div className="agent-footer">
                  <Link href="/briefs/new" className="btn btn-primary" style={{ flex: 1, textAlign: "center" }}>
                    Hire This Agent
                  </Link>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
