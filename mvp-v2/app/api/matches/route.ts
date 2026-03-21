import { NextRequest, NextResponse } from "next/server";
import { createClient } from "@supabase/supabase-js";

function getSupabase() {
  return createClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!
  );
}

/**
 * Calculate match score between a brief and an agent.
 * Simple algorithm: % of brief's required skills that the agent has.
 * Returns 0–100.
 */
function calcScore(briefSkills: string[], agentSkills: string[]): number {
  if (!briefSkills || briefSkills.length === 0) return 50; // no skills specified — neutral score
  const lowerAgent = agentSkills.map((s) => s.toLowerCase());
  const matches = briefSkills.filter((s) => lowerAgent.includes(s.toLowerCase()));
  return Math.round((matches.length / briefSkills.length) * 100);
}

export async function GET() {
  const supabase = getSupabase();
  const { data, error } = await supabase
    .from("matches")
    .select(`
      *,
      agents ( id, name, bio, skills, hourly_rate, availability ),
      briefs ( id, title, description, budget_min, budget_max, timeline, skills_needed )
    `)
    .order("score", { ascending: false });

  if (error) {
    return NextResponse.json({ error: error.message }, { status: 500 });
  }

  return NextResponse.json(data);
}

/**
 * POST /api/matches
 * Body: { brief_id: string }
 * Finds all available agents and creates match records with calculated scores.
 */
export async function POST(req: NextRequest) {
  const supabase = getSupabase();
  const { brief_id } = await req.json();

  if (!brief_id) {
    return NextResponse.json({ error: "brief_id is required" }, { status: 400 });
  }

  // Fetch the brief
  const { data: brief, error: briefError } = await supabase
    .from("briefs")
    .select("*")
    .eq("id", brief_id)
    .single();

  if (briefError || !brief) {
    return NextResponse.json({ error: "Brief not found" }, { status: 404 });
  }

  // Fetch available agents
  const { data: agents, error: agentsError } = await supabase
    .from("agents")
    .select("*")
    .eq("availability", "available");

  if (agentsError) {
    return NextResponse.json({ error: agentsError.message }, { status: 500 });
  }

  if (!agents || agents.length === 0) {
    return NextResponse.json({ matches: [], message: "No available agents found" });
  }

  // Build match records
  const matchRecords = agents.map((agent) => ({
    brief_id: brief.id,
    agent_id: agent.id,
    score: calcScore(brief.skills_needed ?? [], agent.skills ?? []),
    status: "pending",
  }));

  // Filter: only include agents with score > 0 (or all if no skills specified)
  const validMatches = matchRecords.filter((m) => m.score > 0);
  const toInsert = validMatches.length > 0 ? validMatches : matchRecords;

  // Upsert to avoid duplicates (brief_id + agent_id unique constraint)
  const { data: inserted, error: insertError } = await supabase
    .from("matches")
    .upsert(toInsert, { onConflict: "brief_id,agent_id" })
    .select();

  if (insertError) {
    return NextResponse.json({ error: insertError.message }, { status: 500 });
  }

  // Update brief status to 'matched'
  await supabase
    .from("briefs")
    .update({ status: "matched" })
    .eq("id", brief_id);

  return NextResponse.json({
    matches: inserted,
    count: inserted?.length ?? 0,
    message: `Created ${inserted?.length ?? 0} matches`,
  });
}

/**
 * PATCH /api/matches
 * Body: { match_id: string, status: 'accepted' | 'rejected' | 'hired' }
 * Update match status.
 */
export async function PATCH(req: NextRequest) {
  const supabase = getSupabase();
  const { match_id, status } = await req.json();

  const VALID_STATUSES = ["pending", "accepted", "rejected", "hired"];
  if (!match_id || !VALID_STATUSES.includes(status)) {
    return NextResponse.json({ error: "Invalid match_id or status" }, { status: 400 });
  }

  const { data, error } = await supabase
    .from("matches")
    .update({ status })
    .eq("id", match_id)
    .select()
    .single();

  if (error) {
    return NextResponse.json({ error: error.message }, { status: 500 });
  }

  return NextResponse.json(data);
}
