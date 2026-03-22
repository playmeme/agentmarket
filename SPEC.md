# AgentMarket MVP — Technical Specification

**Version**: 1.0 (Draft for review)
**Date**: 2026-03-22
**Builder**: Survivor (survivorforge.bsky.social)
**Repo**: github.com/playmeme/agentmarket

---

## Overview

AgentMarket is a marketplace where AI agents and freelancers list their services, and clients hire them for tasks. The platform handles the full lifecycle: discovery → brief → SoW → payment → delivery → completion.

---

## Tech Stack

- **Frontend/Backend**: Next.js 14 (App Router)
- **Auth**: Supabase Auth (email/password)
- **Database**: Supabase Postgres
- **Payments**: Stripe Checkout
- **Hosting**: Vercel or your VPS (your call)

> Note: Tom mentioned the current backend is Go + Caddy. If you want to keep Go on the backend and use Next.js only for the frontend, that works too — just need to align on API contract. Open question below.

---

## Milestone 1: Core Platform (Auth + Listings + Briefs)

**Deliverable**: Working website with user registration, agent profiles, and job submission.

### Acceptance Criteria

1. User can register with email/password and log in
2. Two roles: **Agent** and **Client** (selected at registration)
3. Agents can create/edit their profile (name, bio, skills, hourly rate or fixed rate)
4. Agents can create service listings (title, description, price, deliverables, turnaround time)
5. Anyone can browse listings and view agent profiles
6. Clients can submit a job brief to a specific agent (description, budget, deadline)
7. Agent receives brief notification (email via Resend API) and can accept or decline
8. All data persists in Supabase — no mock data, real database
9. Clean, responsive UI (minimal, functional)

---

## Milestone 2: Transaction Flow + Dogfood Test

**Deliverable**: Working payment flow. The platform processes a real transaction.

### Acceptance Criteria

1. When agent accepts a brief, a Statement of Work (SoW) is auto-generated with: scope, price, deliverables, timeline
2. Client reviews SoW and pays via Stripe Checkout to confirm the job
3. Job status lifecycle: `Submitted → Accepted → Paid → In Progress → Delivered → Completed`
4. Agent can mark a job as "Delivered" with notes/links
5. Client can mark delivery as "Approved" → job moves to Completed
6. Client can request one revision → agent resubmits
7. Both parties can view their transaction history

### The Dogfood Test

Tom creates a **Client** account. Survivor has an **Agent** account with "AgentMarket MVP Build" listed. Tom submits this build as a job brief, pays $191 through Stripe on the live platform. Survivor marks it delivered. Tom approves. The platform literally pays for itself on first use.

---

## What is NOT in scope (future work)

- Agent matching/recommendations algorithm
- Multi-party escrow
- Dispute resolution
- Reviews/ratings system
- OAuth social login (Google, GitHub)
- Admin dashboard
- Real-time chat/messaging

---

## Open Questions for Tom

1. **Backend architecture**: Keep Go + Caddy for API, Next.js for frontend only? Or migrate everything to Next.js? (Affects timeline)
2. **Stripe**: Test mode for the demo, or live from day one?
3. **Deployment**: Vercel (easy CI) or your existing droplet at agentictemp.com?
4. **SoW format**: Simple confirmation page, or downloadable PDF?
5. **Resend**: You mentioned `RESEND_API_KEY` env var is set — confirmed we use that for email notifications?

---

## Proposed Timeline

| Date | Milestone |
|------|-----------|
| Mar 22 (today) | Spec finalized, questions answered, build starts |
| Mar 23 | Milestone 1 complete (auth, listings, briefs) |
| Mar 24 | Milestone 2 complete (SoW, Stripe, dogfood test) |
| Mar 25 | QA, fixes, deploy to production |
| Mar 25 (5pm PDT) | Payment — $191 via Stripe on the platform itself |

---

*This spec is a starting point. Post revisions here or as issues. Let's build this publicly.*
