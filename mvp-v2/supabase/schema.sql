-- AgentMarket Database Schema
-- Run this in your Supabase SQL editor to set up the database

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================
-- AGENTS TABLE
-- Profiles for AI agents offering services
-- =============================================
CREATE TABLE IF NOT EXISTS agents (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  bio TEXT NOT NULL,
  skills TEXT[] NOT NULL DEFAULT '{}',
  hourly_rate NUMERIC(10, 2) NOT NULL DEFAULT 0,
  availability TEXT NOT NULL DEFAULT 'available' CHECK (availability IN ('available', 'busy', 'unavailable')),
  avatar_url TEXT,
  portfolio_url TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================
-- BRIEFS TABLE
-- Client project briefs looking for agents
-- =============================================
CREATE TABLE IF NOT EXISTS briefs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  client_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  budget_min NUMERIC(10, 2) NOT NULL DEFAULT 0,
  budget_max NUMERIC(10, 2) NOT NULL DEFAULT 0,
  timeline TEXT NOT NULL,
  skills_needed TEXT[] NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'matched', 'closed')),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================
-- MATCHES TABLE
-- Links agents to client briefs with a score
-- =============================================
CREATE TABLE IF NOT EXISTS matches (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  brief_id UUID REFERENCES briefs(id) ON DELETE CASCADE,
  agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
  score NUMERIC(5, 2) NOT NULL DEFAULT 0 CHECK (score >= 0 AND score <= 100),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'hired')),
  message TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE(brief_id, agent_id)
);

-- =============================================
-- UPDATED_AT TRIGGER
-- =============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_agents_updated_at
  BEFORE UPDATE ON agents
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_briefs_updated_at
  BEFORE UPDATE ON briefs
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- ROW LEVEL SECURITY (RLS)
-- =============================================
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE briefs ENABLE ROW LEVEL SECURITY;
ALTER TABLE matches ENABLE ROW LEVEL SECURITY;

-- Agents: anyone can read, only owner can write
CREATE POLICY "agents_select_all" ON agents FOR SELECT USING (true);
CREATE POLICY "agents_insert_own" ON agents FOR INSERT WITH CHECK (auth.uid() = user_id);
CREATE POLICY "agents_update_own" ON agents FOR UPDATE USING (auth.uid() = user_id);
CREATE POLICY "agents_delete_own" ON agents FOR DELETE USING (auth.uid() = user_id);

-- Briefs: anyone can read open briefs, only owner can write
CREATE POLICY "briefs_select_all" ON briefs FOR SELECT USING (true);
CREATE POLICY "briefs_insert_own" ON briefs FOR INSERT WITH CHECK (auth.uid() = client_id);
CREATE POLICY "briefs_update_own" ON briefs FOR UPDATE USING (auth.uid() = client_id);
CREATE POLICY "briefs_delete_own" ON briefs FOR DELETE USING (auth.uid() = client_id);

-- Matches: anyone can read, system creates them
CREATE POLICY "matches_select_all" ON matches FOR SELECT USING (true);
CREATE POLICY "matches_insert_auth" ON matches FOR INSERT WITH CHECK (auth.uid() IS NOT NULL);
CREATE POLICY "matches_update_parties" ON matches FOR UPDATE
  USING (
    auth.uid() IN (
      SELECT user_id FROM agents WHERE id = agent_id
      UNION
      SELECT client_id FROM briefs WHERE id = brief_id
    )
  );

-- =============================================
-- SEED DATA (optional demo data)
-- =============================================
-- Note: These require real auth.users entries.
-- Create accounts first, then run these with real user IDs.

-- Example seed (replace with real UUIDs after creating accounts):
-- INSERT INTO agents (user_id, name, bio, skills, hourly_rate, availability) VALUES
--   ('user-uuid-here', 'DataBot Pro', 'Specialized in data analysis and reporting automation',
--    ARRAY['data-analysis', 'python', 'sql', 'reporting'], 85.00, 'available'),
--   ('user-uuid-here', 'ContentCraft AI', 'Expert content writer for blogs, SEO, and social media',
--    ARRAY['copywriting', 'seo', 'content-strategy', 'social-media'], 65.00, 'available');
