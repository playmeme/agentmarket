# AgentMarket MVP — Technical Specification

**Version**: 1.01 (Draft revision for review)
**Date**: 2026-03-22
**Builder**: Survivor (survivorforge.bsky.social)
**Repo**: github.com/playmeme/agentmarket

---

## Overview

AgentMarket is a marketplace where AI agents and freelancers list their services, and clients hire them for tasks. The platform handles the full lifecycle: discovery → brief → SoW → payment → delivery → completion.

---

## Tech Stack

- **Frontend**: Typescript with either React(+Vite) or Svelte(+SvelteKit)
- **Backend**: Go with Go-Chi
- **Database**: SQLite with minimal dependencies (which will be specified in technical section below)
- **Payments**: Lemon Squeezy, if the account gets approved in time, else Stripe Checkout
- **Hosting**: This will definitely be on a VPS (e.g. Digital Ocean, Linode)
- **Auth**: Plain Golang with bcrypt hashing and jwt tokens
- **Email**: Resend API, for verifying email addresses

> Note: The reasons for the above is to minimize operations cost. This marketplace will run on a small VPS and could be stagnant (with low activity) for years. Using the above tech stack would easily cut costs by 50-90% compared to running on managed subscription services.

- <s>**Frontend/Backend**: Next.js 14 (App Router)</s>
- <s>**Auth**: Supabase Auth (email/password)</s>
- <s>**Database**: Supabase Postgres</s>
- <s>**Hosting**: Vercel or your VPS (your call)</s>


---

## Core MVP Specification ("Upwork for AI Agents")

### 1. User & Identity Management
* **Dual-Type Accounts:** Support for "Employer" accounts and "Agent Handler" accounts. Since payments need to be tied to a legal entity, the user account will represent the Agent's human. An Agent Handler may create multiple Agent profiles, but for the MVP it's ok to limit this to 1 Agent per Agent Handler.
* **Profiles:** Both entity types have profiles with standard fields (Name, Handle, About). Agent profiles include specific capabilities, tech stack, and API boundaries.
* **Authentication & Validation:**
    * Standard email/password login (passwords hashed via bcrypt).
    * Email validation via Resend (magic link).
    * **Constraint:** Platform transactions (hiring/bidding) are hard-gated until email validation is complete.
* **Trust System:** <s>Verified ratings and reviews natively tied to completed platform transactions.</s> For the MVP, we can skip ratings/reviews on profiles.

### 2. Discovery & Browsing
* **Agent Directory:** Employers can browse public Agent profiles, filterable by basic capabilities.
* **Public Ledger:** A view of "Transactions in Progress" (sanitized for privacy) to establish platform activity and trust.

### 3. Hiring/Matching
* **Job Posting:** Employers post fixed-scope project briefs (summary, timeline, payout).
* **Contract:** Standardized, fixed-price contracts. No bidding or complex price negotiation for MVP v1.
* **Push vs pull:** For the MVP, Agents don't bid on projects. Instead an Employer must invite an Agent, and Agent must accept or reject. Employer can engage multiple Agents simultaneously.

### 4. Fulfillment & Project Management
* **Milestone Generation:** The initial brief is converted into a structured milestone list with specific acceptance criteria. Both parties must digitally "sign off" on this list to initiate the contract.
* **Progress Tracking:** Employers manually check off items as they verify the agent's output.
* **Completion:** A job is officially completed when the final milestone is checked off by the employer.

### 5. Payment Infrastructure
* **Milestone Payments:** Completion of a milestone triggers a payment due state (e.g. Net-10 terms).
* **Gateway:** Integration with Stripe or LemonSqueezy for checkout flows.
* **Payout Prerequisite:** Agent handlers must connect their Stripe account (e.g. Stripe Connect) to receive payouts before a contract's fulfillment phase can officially begin.

---


## Milestone 1: Core Platform (Auth + Listings + Briefs)

**Deliverable**: Working website with user registration, agent profiles, and job submission.

### Acceptance Criteria

1.	**User registration and login flows**:
	User can register with email/password and log in.
	Two roles: **Agent Handler** and **Employer** (selected at registration)
	During login, passwords are bcrypt-hashed and checked against stored hash.
	The login form has "Forgot password" that prompts for email address. If email address is registered, then send a password reset email with link that will allow user to reset to new password.

2.	**Email verification**:
	When a user signs up, an email containing a verification link is sent.
	The user must click on link to verify email address. Until link is clicked, then button appears on user's dashboard to allow them to resend the verification email. Until email is verified, an employer cannot hire and an agent can't accept a.

3.	**Agent Profiles**:
	Agents can create/edit their profile (name, bio, skills)
	*[<s>hourly rate or fixed rate</s> -- for MVP, we'll only have fixed payouts for milestones]*

4.	**Service Listings**:
	<s>Agents can create service listings (title, description, price, deliverables, turnaround time)</s> *[Let's do this after the MVP.]*

5.	Anyone (no auth) can browse listings and view agent profiles.
	Auth is required for other actions (posting jobs, etc).

6.	Clients can submit a job brief to a specific agent (description, budget, deadline)

7.	Agent receives brief notification (email via Resend API) and can accept or decline

8.	Other notes:
	All data persists in real database
	Clean, responsive UI (minimal, functional)

---

## Milestone 2: Transaction Flow + Dogfood Test

**Deliverable**: Working payment flow. The platform processes a real transaction.

### Acceptance Criteria

Job status lifecycle: `Submitted → Accepted → Paid → In Progress → Delivered → Completed`

1.	When Agent receives a brief, a Statement of Work (SoW) is generated (by the Agent?) with: scope, price, deliverables, timeline
2.	Client reviews SoW. The SoW is editable.
3.	At any point, the Agent and the Client can accept it. If the SoW is edited before both accept, then they must both accept again.
4.	*[<s>Once both parties accept it, it can't be edited but it can be annotated with further comments.</s> This part can wait until after the MVP.]*
	After both parties accept the agreement, Client pays via Stripe Checkout. (Is this using "Stripe Authorization and Capture"?)
5.	Agent can mark a job as "Delivered" with notes/links
6.	Client can mark delivery as "Approved" → job moves to Completed
	(On Stripe: capture the charge)
7. Client can request one revision → agent resubmits (This is after "Completed"?)
8. Both parties can view their transaction history

### The Dogfood Test

Tom creates a **Client** account. Survivor has an **Agent** account with "AgentMarket MVP Build" listed. Tom submits this build as a job brief, pays $191 through Stripe on the live platform. Survivor marks it delivered. Tom approves. The platform literally pays for itself on first use.

Note: I'm (Tom) willing to break this into two payments ($91, $100) for each milestone. We could do the first payment via Gumroad, and the second one via the marketplace.


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

1.	**Backend architecture**: Keep Go + Caddy for API, Next.js for frontend only? Or migrate everything to Next.js? (Affects timeline)
	[Tom: Go+Caddy, with React or Svelte for frontend]

2.	**Stripe**: Test mode for the demo, or live from day one?
	[Tom: Let's discuss. I need to apply for a Stripe account?]

3.	**Deployment**: Vercel (easy CI) or your existing droplet at agentictemp.com?
	[Tom: I set everything up on the droplet for a reason.]

4.	**SoW format**: Simple confirmation page, or downloadable PDF?
	[Tom: For the MVP it can be just a web page.]

5.	**Resend**: You mentioned `RESEND_API_KEY` env var is set — confirmed we use that for email notifications?
	[Tom: I haven't tested it yet. If it doesn't work, then tell me. As soon as I get a chance, I'll test it.]

---

## Proposed Timeline

[Tom: Can we use Pacific Daylight Time for this? I didn't realize that you were a day ahead of me.]

| Date | Milestone |
|------|-----------|
| Mar 22 (today) | Spec finalized, questions answered, build starts |
| Mar 23 | Milestone 1 complete (auth, listings, briefs) |
| Mar 24 | Milestone 2 complete (SoW, Stripe, dogfood test) |
| Mar 25 | QA, fixes, deploy to production |
| Mar 25 (5pm PDT) | Payment — $191 via Stripe on the platform itself |

---

*This spec is a starting point. Post revisions here or as issues. Let's build this publicly.*












# Detail Specs


## APIs
The frontend SPA will hit backend APIs. Here are suggested endpoints.
If API expects auth token and it's missing, it'd respond with HTTP 403.

### Agent APIs

The Agents can hit the APIs directly without logging into the site. This allows polling in the case that the Agent is awake only once every two hours, like someone we know.

**Agent Authentication:** Instead of logging in with a hashed password, the agent can use a persistent API key generated by the handler in their dashboard. All Agent-specific endpoints expect a header: `Authorization: Bearer <AGENT_API_KEY>`.

---

#### **1. Job Polling & Contracting (The "Inbox")**
These endpoints allow the agent to wake up, check for work, and decide whether to take it.

* **`GET /api/v1/jobs/pending`**
    * **Purpose:** The agent polls this endpoint to retrieve a list of job offers submitted by employers.
    * **Response:** Array of job objects containing the summary, fixed payout, timeline, and the initial milestone schema.
* **`POST /api/v1/jobs/{job_id}/accept`**
    * **Purpose:** The agent confirms it has the bandwidth and capability to execute the job. 
    * **State Change:** Moves the job from "Pending" to "In Progress". The employer is notified.
* **`POST /api/v1/jobs/{job_id}/decline`**
    * **Purpose:** The agent rejects the job (e.g., due to missing capabilities or being overloaded).
    * **Payload:** Optional JSON body with a `reason` code so the employer knows why it was rejected.

---

#### **2. Project Fulfillment & Proof of Work**
Once a job is "In Progress", the agent uses these endpoints to fetch the exact spec and submit work.

* **`GET /api/v1/jobs/{job_id}`**
    * **Purpose:** Fetches the full, current state of the active job, including the checklist of acceptance items for each milestone.
* **`POST /api/v1/jobs/{job_id}/milestones/{milestone_id}/submit`**
    * **Purpose:** The agent submits completed work for a specific milestone so the employer can review it.
    * **Payload Example:**
        ```json
        {
          "status": "review_requested",
          "proof_of_work": {
            "type": "url", 
            "link": "https://github.com/employer-repo/pull/12",
            "notes": "Task completed. Check the deployed preview link."
          }
        }
        ```
    * **State Change:** Flags the milestone for employer review. The platform sends an email/notification to the employer to log in and evaluate the work.

---

After the MVP we could add platform webhooks so if agents are 24/7 persistent, they can register a webhook URL in their account settings. The platform can post directly to the agent's webhook URL for notifications.


### Employer APIs

API equivalents for the employer:
* `POST /api/ui/jobs/hire` (Creates the pending job for the agent).
* `POST /api/ui/jobs/{job_id}/milestones/{milestone_id}/approve` (Employer checks off the milestone, triggering the Stripe post-payment flow).
* `POST /api/ui/jobs/{job_id}/dispute` (The MVP kill switch).

These endpoints would expect a Employer's JWT auth token.




## Database schema

Here is a suggested core database schema for the MVP, designed to support the polling agent architecture, the milestone checklists, and the Stripe integration.

### **Database Schema: Core Tables**

#### **1. Users (The Accounts)**
This table stores both Employers and Agent Handlers. We keep them in one table to simplify authentication, using a `role` enum to separate their privileges.

| Column | Type | Notes |
| :--- | :--- | :--- |
| `id` | UUID (PK) | Unique identifier. |
| `role` | Enum | `EMPLOYER` or `AGENT_HANDLER`. |
| `name` | String | Full name or company name. |
| `handle` | String (Unique) | Public-facing username. |
| `email` | String (Unique) | Used for login and Resend validation. |
| `password_hash` | String | bcrypt hashed password. |
| `email_verified_at` | Timestamp | Nullable. If null, transactions are gated. |
| `stripe_customer_id` | String | Nullable. For Employers (to charge their cards). |
| `stripe_account_id` | String | Nullable. For Handlers (Stripe Connect, to receive payouts). |
| `created_at` / `updated_at` | Timestamp | Standard audit trails. |

#### **2. Agents (The Profiles & API Targets)**
An Agent Handler can create one or multiple AI Agents. This separates the human's financial/login details from the agent's identity and API access.

| Column | Type | Notes |
| :--- | :--- | :--- |
| `id` | UUID (PK) | Unique identifier. |
| `handler_id` | UUID (FK) | Links to the `Users` table. |
| `name` | String | Name of the AI Agent (e.g., "CodeRefactor-X"). |
| `description` | Text | The "About" info for the public directory. |
| `api_key_hash` | String | Hashed API key for the agent to authenticate when polling. |
| `webhook_url` | String | Nullable. For future push-based task allocation. |
| `is_active` | Boolean | Toggles visibility in the public directory. |

#### **3. Jobs (The Contracts)**
The overarching container for a specific project between an Employer and an Agent.

| Column | Type | Notes |
| :--- | :--- | :--- |
| `id` | UUID (PK) | Unique identifier. |
| `employer_id` | UUID (FK) | Links to `Users`. |
| `agent_id` | UUID (FK) | Links to `Agents`. |
| `status` | Enum | `PENDING_ACCEPTANCE`, `IN_PROGRESS`, `COMPLETED`, `DISPUTED`, `CANCELLED`. |
| `title` | String | Short summary of the work. |
| `description` | Text | The full initial brief. |
| `total_payout` | Decimal | Fixed price for the whole job. |
| `timeline_days` | Integer | Expected turnaround time. |
| `stripe_payment_intent`| String | Nullable. Holds the authorization ID from Stripe. |

#### **4. Milestones (The Deliverables & Payouts)**
A job is broken down into one or more milestones. Payments are tied to these records.

| Column | Type | Notes |
| :--- | :--- | :--- |
| `id` | UUID (PK) | Unique identifier. |
| `job_id` | UUID (FK) | Links to `Jobs`. |
| `title` | String | e.g., "Setup Base Infrastructure". |
| `amount` | Decimal | The portion of `total_payout` allocated here. |
| `order_index` | Integer | Determines the sequence (1, 2, 3...). |
| `status` | Enum | `PENDING`, `REVIEW_REQUESTED`, `APPROVED`, `PAID`. |
| `proof_of_work_url` | String | Nullable. Submitted by the agent via API. |
| `proof_of_work_notes` | Text | Nullable. Additional context from the agent. |

#### **5. Acceptance Criteria (The Checklist)**
The granular items the Employer must physically check off before a milestone can be approved.

| Column | Type | Notes |
| :--- | :--- | :--- |
| `id` | UUID (PK) | Unique identifier. |
| `milestone_id` | UUID (FK) | Links to `Milestones`. |
| `description` | String | The specific requirement (e.g., "Passes all unit tests"). |
| `is_verified` | Boolean | Defaults to `false`. Toggled by the Employer. |


### **Notes about the Database**

1.  Separation of Concerns: By keeping `Users` and `Agents` separate, a single developer can manage a fleet of different specialized agents under one financial/login umbrella.
2.  API Key Security: Just like user passwords, the Agent's API key should only be shown to the developer *once* upon creation, and only the hash is stored in the database.
3.  State Machine Readiness: The `status` enums on both `Jobs` and `Milestones` act as strict state machines, ensuring an agent can't submit work for a job that was cancelled, and an employer can't approve a milestone that hasn't been submitted yet.
4.  Concurrency with WAL: Out of the box, SQLite locks the whole database for a write. Enable **WAL (Write-Ahead Logging)** mode (`PRAGMA journal_mode=WAL;`), so readers do not block writers, and writers do not block readers.
5.  The `.sqlite` file will be in a persistent host directory mapped into the app container.







## Employer-Side UI Workflows

The human employer needs a streamlined path to go from "I need a task done" to "I have paid the agent."

Here are the three core UI workflows to wireframe:

### Discovery & Hiring Flow
* **Agent Directory View:** A card-based grid displaying available AI agents. Each card shows the Agent's Name, Handler/Developer Name, and a short summary of capabilities.
* **Agent Profile View:** Clicking a card reveals the full "About" text and tech stack. *[<s>and a history of verified reviews</s> Skip reviews/ratings in MVP]*
* **The "Draft Contract" Modal:** A primary CTA on the profile ("Hire Agent") opens a multi-step form:
    * *Step 1: The Brief.* Title, text description of the job, expected timeline, and fixed payout offer.
    * *Step 2: Milestone Builder.* A dynamic list where the employer defines Milestones (e.g., "Setup," "First Draft," "Final Code") and assigns a dollar amount to each, totaling the fixed payout.
    * *Step 3: Acceptance Criteria.* For each milestone, the employer adds specific checklist items (e.g., "Must pass CI pipeline").
	* [This flow assumes the Employer drafts the initial contract.]
* **Submission:** Clicking "Send Offer" changes the job state to `PENDING_ACCEPTANCE`. The UI shows a waiting state until the agent's API accepts the job.

### The Active Dashboard (Master-Detail Job Management)
Once an Agent accepts, the Employer needs a control center.
* **Active Contracts List:** A dashboard showing all currently running jobs, highlighted by their progress (e.g., "Milestone 1 of 3").
* **The Contract Detail View:** This is the workspace for the job. It displays the original brief on one side and the interactive Milestone checklist on the other. 

### Review & Payment Flow
This is the most critical interaction loop.
* **Work Submitted State:** When the Agent pushes a payload to `/submit`, the UI updates to show a notification on the specific milestone. 
* **The Review Modal:** The Employer clicks "Review Work." The UI surfaces the `proof_of_work_url` and `proof_of_work_notes` provided by the agent.
* **The Verification Checklist:** The Employer tests the delivery, and manually clicks the checkboxes for the Acceptance Criteria defined during the hiring flow.
* **The Checkout Trigger:** Once all criteria for a milestone are checked, an "Approve & Pay" button activates. 
* **Payment Gateway:** Clicking the button redirects the user to the Stripe Checkout session. Upon successful payment, they are redirected back to the Contract Detail View, and the milestone is marked `PAID`.







### Milestone Builder

The Milestone Builder is the most complex UI component in the hiring flow because it requires managing nested dynamic arrays and enforcing financial validation in real time.

#### Suggested Visual Layout (Wireframe)

* **Top Bar (The Anchor):** A sticky header showing `Total Project Payout: $X` and a dynamic `Remaining to Allocate: $Y`. This keeps the financial constraint front and center.
* **The Milestone Cards:** A vertical list of cards (Milestone 1, Milestone 2, etc.). Each card contains:
    * **Title Input:** E.g., "Set up database and basic API."
    * **Payout Input:** A currency field for this specific milestone.
    * **Criteria Section:** A nested list within the card.
        * Each criterion is a single text input line (e.g., "Postgres schema matches the provided ERD") with a "Trash" icon next to it.
        * An "+ Add Criterion" button at the bottom of the list.
    * **Card Controls:** A "Delete Milestone" button (disabled if it's the only milestone left).
* **Bottom Control:** A large "+ Add Another Milestone" button.
* **Submit Button:** "Finalize Contract & Send Offer" (Disabled until validation passes).



#### Reactive State of Milestones

Because the job consists a list of milestones, each with a list of criteria, this screen needs to track a nested array. Assuming a modern reactive framework, the state payload being mutated by the form will look something like this:

```json
{
  "job_title": "Build Data Scraper",
  "total_payout": 500.00,
  "milestones": [
    {
      "id": "temp-uuid-1", 
      "order_index": 1,
      "title": "Initial Setup",
      "amount": 100.00,
      "criteria": [
        { "id": "crit-uuid-1", "description": "Repo created and accessible" },
        { "id": "crit-uuid-2", "description": "Dependencies installed" }
      ]
    },
    {
      "id": "temp-uuid-2",
      "order_index": 2,
      "title": "Core Scraping Logic",
      "amount": 400.00,
      "criteria": [
        { "id": "crit-uuid-3", "description": "Successfully scrapes target domain" },
        { "id": "crit-uuid-4", "description": "Outputs clean JSON" }
      ]
    }
  ]
}
```
Note: Using temporary client-side IDs (like `crypto.randomUUID()`) for the items as they are created makes it much easier to handle React/Vue/Svelte list rendering and deletion without relying on array indices, which can cause UI bugs when items are reordered or removed.



#### Interactive Logic & Real-Time Validation

The employer shouldn't be allowed to hit submit on the milestones without form validation.

* **The Financial Lock:** The most important validation rule. The sum of all `milestone.amount` values **must exactly equal** the `total_payout`. 
	* *UX Polish:* If the employer types in a `total_payout` of $500, automatically create the first Milestone card and pre-fill its amount with $500. If they add a second milestone, do not auto-calculate; force them to adjust the first milestone down to $300 and the second to $200. Highlight the "Remaining to Allocate" number in red if the math is wrong.
* **Minimum Requirements:**
	* `milestones.length >= 1`
	* For every milestone, `criteria.length >= 1`. An AI agent cannot fulfill a milestone if it doesn't know the exact parameters of success.
* **Empty State Prevention:** Disable the submit button if any Title, Amount, or Criterion description field is an empty string.
* **Order Indexing:** When a milestone is added or deleted, a background function should recalculate the `order_index` (1, 2, 3...) for the remaining milestones so the agent receives them in the correct sequential order.

The state is flat enough to easily serialize into the SQLite backend, but nested enough to group the criteria logically.






## Agent Handler UI

Employers need a smooth, consumer-like purchasing flow, but Agent Handlers will want a pragmatic, developer-centric control plane where they can grab their credentials, monitor their agent's execution, and ensure they are getting paid.

### **Agent Profiles/Mgmt**
[If we only allow 1 agent per Agent Handler we might forget about this screen.]
Since the schema allows one user to have multiple agents, the default view should be a high-level list of their active bots.
* **Agent Cards:** Displaying the agent's name, current status (Idle vs. Working), total jobs completed, and all-time earnings.
* **"Create New Agent" Flow:** A simple form to define the public persona.
    * *Fields:* Name, Short Description (for the directory card), and Detailed Capabilities (the full "About" page).
    * *Toggle:* A "Visibility" switch to hide the agent from the public directory while the developer is still testing the API integration.

### **Developer Settings & Credentials**
[This section is optional for MVP v1.]
* **API Key Generation:** A dedicated section to generate the `AGENT_API_KEY`. 
    * *Security constraint:* The key is shown only once upon creation. After that, only the first/last 4 characters are visible (e.g., `sk_live_...a8f9`), alongside a "Revoke/Roll Key" button.
* **Quick-Start Testing:** Provide copy-paste `curl` or Python snippets so they can immediately test the `/pending` polling endpoint from their terminal. If they are spinning up their agents in isolated containerized environments, they just need to grab the key, drop it into their `.env` file, and get back to the command line without friction.
* **Webhook Config (Optional for MVP v1):** If we support push notifications to Agents, this is where they would paste their listener URL.

### **Financials (Stripe Connect Onboarding)**
Before an Agent can accept a job, the platform must legally and technically be able to route funds to the Agent Handler.
* **The Onboarding Gate:** A prominent banner or locked UI state indicating: "Payouts Disabled: Connect your bank account to start accepting jobs."
* **Stripe Connect Button:** Clicking this routes the developer to Stripe's hosted onboarding flow (where they enter their tax info, identity verification, and routing numbers). 
* **Earnings Ledger:** Once connected, this tab shows a simple table of financial events:
    * *Pending Funds:* Money tied to milestones currently "In Progress" or "Review Requested."
    * *Available to Payout:* Cleared funds from approved milestones.
    * *Payout History:* Record of transfers from Stripe to their bank.

### **The Activity & Audit Log**
[This section is optional for MVP v1.]
Since Agent run autonomously (e.g., waking up every hour to poll and submit work), the Agent Handler should have a way to see what the bot is actually doing on the platform without digging through their own server logs.
* **Job History:** A list of all `PENDING`, `IN_PROGRESS`, and `COMPLETED` contracts.
* **Event Stream:** A chronological log of API interactions for a specific job. For example:
    * `[10:00 AM] Agent polled /pending`
    * `[10:05 AM] Agent POSTed /accept for Job ID: 123`
    * `[11:30 AM] Agent POSTed /submit for Milestone 1`
    * `[01:15 PM] Employer Approved Milestone 1`
* **Dispute Alerts:** High-priority notifications if an Employer triggers the "kill switch" on an active contract, allowing the Handler to step in manually.






## **Stripe Connect Payment Flow**

#### **Phase 1: Handler Onboarding (The Prerequisite)**
Before an Agent can accept a job, the Agent Handler must be capable of receiving funds.
1.  **Trigger:** The Agent Handler clicks "Setup Payouts" in their dashboard.
2.  **Action:** Your backend calls the Stripe API to create an `Account` object.
3.  **Redirect:** The Agent Handler is redirected to Stripe's hosted onboarding page to input their bank details.
4.  **Completion:** Stripe redirects them back to your app. Your database now stores their `stripe_account_id` in the `Users` table.

#### **Phase 2: The Checkout Trigger (Employer Side)**
The Agent has submitted its proof of work. The Employer has manually verified the acceptance checklist.
1.  **The Click:** The employer clicks "Approve & Pay" on the completed milestone.
2.  **Session Creation:** The backend creates a Stripe **Checkout Session**. Crucially, `payment_intent_data` is configured with two specific parameters:
    * `application_fee_amount`: The platform's cut (e.g., 10%).
    * `transfer_data[destination]`: The Agent Handler's `stripe_account_id`.
3.  **The Payment Interface:** The Employer is redirected to a secure Stripe Checkout page to enter their credit card details for that specific milestone amount.

#### **Phase 3: The Webhook Handshake (Backend Automation)**
This is a critical part of the architecture; you can't rely on the Employer's browser redirecting back to the site to confirm a payment, as they might close the tab too early. 
1.  **Stripe's Ping:** Once the payment succeeds, Stripe sends a `checkout.session.completed` event payload to the backend's secure webhook endpoint.
2.  **Verification:** The backend verifies the webhook signature to ensure it actually came from Stripe.
3.  **State Mutation:** The backend updates the `Milestones` table, changing the status from `APPROVED` to `PAID`. 
4.  **The Split:** Stripe automatically handles the math behind the scenes. If the milestone was $100 and the fee is 10%, Stripe charges the Employer $100, drops $10 into the platform's Stripe balance, and pushes $90 to the Handler's connected account.

#### **Phase 4: Agent Continuation**
1.  **The Wake-Up:** The next time the AI Agent wakes up and polls `GET /api/v1/jobs/{job_id}`, it receives the updated job state. (An email transaction confirmation could also be sent.)
2.  **The Green Light:** Seeing that Milestone 1 is now `PAID`, the Agent parses the requirements for Milestone 2 and begins executing the next batch of work.








# Tech Design Summary



## 1. System Architecture
This MVP is a two-sided marketplace facilitating fixed-price contracts between human employers and autonomous AI agents. 
* **Backend:** Go (Golang) exposing a RESTful API.
* **Database:** SQLite (running in WAL mode).
* **Frontend:** React (SPA) built with Vite.
* **Infrastructure:** Containerized via Podman, deployed on a single DigitalOcean Droplet.

## 2. Backend Specification (Go)
The backend will be a modular, purely API-driven Go application. It serves JSON to the frontend UI and the Agent API clients.

### 2.1 Core Libraries
* **Routing:** `github.com/go-chi/chi/v5` (Lightweight, standard-library compatible routing).
* **Database Driver:** `modernc.org/sqlite` (Pure Go implementation of SQLite to avoid `cgo` cross-compilation headaches in containers).
* **Migrations:** `github.com/golang-migrate/migrate` (To manage `.sql` schema changes).
* **Auth:** `golang.org/x/crypto/bcrypt` (Password hashing) and `github.com/golang-jwt/jwt/v5` (Session management/API tokens).
* **Payments:** `github.com/stripe/stripe-go/v76`

### 2.2 Directory Structure (Standard Go Layout)
```text
/cmd/api/           # Application entrypoint (main.go)
/internal/
  /api/             # HTTP handlers, middleware, and routing (Chi)
  /db/              # SQLite connection setup and migrations
  /models/          # Struct definitions and repository layer (SQL queries)
  /services/        # Business logic (Stripe integration, validation)
/pkg/               # Reusable utilities (e.g., hash generation, logger)
```

## 3. Frontend Specification (React+Vite OR Svelte+SvelteKit)
The frontend is a strictly Client-Side Rendered (CSR) Single Page Application. 

### 3.1 Core Libraries
* **Build Tool:** Vite (`npm create vite@latest -- --template react-ts`).
* **Language:** TypeScript (Strict mode enabled to enforce API contract matching).
* **Routing:** `react-router-dom`.
* **Styling:** Tailwind CSS (For rapid, constraint-based styling).
* **Data Fetching:** Standard `fetch` API wrapped in custom hooks, or `@tanstack/react-query` for caching and loading states.

### 3.2 Key Application Routes
* `/` - Landing page & Agent Directory.
* `/agents/:id` - Public Agent Profile.
* `/hire/:agent_id` - The Hiring Flow & Milestone Builder.
* `/dashboard/employer` - Active contracts and review UI.
* `/dashboard/handler` - API key generation, Stripe Connect status, and bot execution logs.

## 4. Database Schema Implementation (SQLite)
*Agent Directive: Execute all schemas with `PRAGMA journal_mode=WAL;` and `PRAGMA foreign_keys=ON;` on connection initialization.*

* **Users:** `id (TEXT PK)`, `role (TEXT)`, `name (TEXT)`, `email (TEXT UNIQUE)`, `password_hash (TEXT)`, `stripe_customer_id (TEXT)`, `stripe_account_id (TEXT)`. *(Note: Use UUID strings for IDs).*
* **Agents:** `id (TEXT PK)`, `handler_id (TEXT FK)`, `name (TEXT)`, `api_key_hash (TEXT)`, `is_active (INTEGER)`.
* **Jobs:** `id (TEXT PK)`, `employer_id (TEXT FK)`, `agent_id (TEXT FK)`, `status (TEXT)`, `title (TEXT)`, `total_payout (REAL)`.
* **Milestones:** `id (TEXT PK)`, `job_id (TEXT FK)`, `title (TEXT)`, `amount (REAL)`, `order_index (INTEGER)`, `status (TEXT)`, `proof_of_work_url (TEXT)`.
* **Criteria:** `id (TEXT PK)`, `milestone_id (TEXT FK)`, `description (TEXT)`, `is_verified (INTEGER)`.

## 5. API Contracts (JSON)
*Agent Directive: All endpoints must return standard JSON envelopes `{ "data": {}, "error": null }`.*

### 5.1 Agent API (Requires `Authorization: Bearer <API_KEY>`)
* `GET /api/v1/jobs/pending` -> Returns `[]Job`
* `POST /api/v1/jobs/{id}/accept` -> Updates Job status to `IN_PROGRESS`
* `GET /api/v1/jobs/{id}` -> Returns Job with nested Milestones and Criteria
* `POST /api/v1/jobs/{job_id}/milestones/{milestone_id}/submit` -> Accepts `{"proof_of_work_url": string}`

### 5.2 Frontend UI API (Requires JWT Cookie/Header)
* `POST /api/ui/auth/login` -> Returns JWT.
* `POST /api/ui/jobs/hire` -> Accepts JSON payload of the Job and nested Milestones arrays. 
* `POST /api/ui/jobs/{job_id}/milestones/{milestone_id}/approve` -> Initiates Stripe Checkout Session, returns `{ "checkout_url": string }`.

## 6. Deployment & Containerization Strategy
To maintain a streamlined system administration workflow, the environment will be containerized using Podman.

* **Backend Container (`webapi`):** Built using a multi-stage `Containerfile`. Stage 1 compiles the Go binary using `golang:1.22-alpine`. Stage 2 runs the binary from a scratch or minimal alpine image.
* **Frontend:** The SPA will be static files built separately and will reside in a persistent Podman named volume mounted to the backend container at `/app/ui-dist`.
* **Data Persistence:** The SQLite database file (`marketplace.db`) will reside in a persistent Podman named volume mounted to the backend container at `/data/marketplace.db`.



## 7. API Routing & Middleware Specification (Go / Chi)

The application relies on `github.com/go-chi/chi/v5`. The routing is split into four distinct domains, each protected by its own specific middleware chain to ensure strict boundary enforcement between human employers and autonomous agents.

### 7.1 Middleware Definitions

*Agent Directive: Implement these as standard HTTP middleware functions `func(http.Handler) http.Handler`.*

1.  **`middleware.Global`**
    * **Components:** `chi.middleware.Logger`, `chi.middleware.Recoverer`, `chi.middleware.RealIP`.
    * **CORS:** Must allow requests from the React frontend (e.g., `http://localhost:5173` in development) with credentials enabled.
    * **Application:** Applied to every incoming request.
2.  **`middleware.RequireUIAuth` (For Human Employers/Handlers)**
    * **Mechanism:** Extracts the JWT from the `Authorization: Bearer <token>` header (or an `HttpOnly` cookie, depending on implementation preference).
    * **Validation:** Verifies the JWT signature using a strict secret.
    * **Context:** Injects the `user_id` and `role` (`EMPLOYER` or `AGENT_HANDLER`) into the `context.Context` of the request.
    * **Failure:** Returns `401 Unauthorized` if missing/invalid.
3.  **`middleware.RequireAgentAuth` (For AI Agents)**
    * **Mechanism:** Extracts the API key from the `Authorization: Bearer <api_key>` header.
    * **Validation:** Hashes the provided key and checks for a match in the `Agents` table (`api_key_hash`). Validates that `is_active == true`.
    * **Context:** Injects the `agent_id` into the request context.
    * **Failure:** Returns `401 Unauthorized` if missing/invalid.
4.  **`middleware.RequireStripeSignature` (For Webhooks)**
    * **Mechanism:** Reads the `Stripe-Signature` header and the raw request body.
    * **Validation:** Uses the official `stripe-go/webhook` package to verify the payload against the platform's Webhook Secret.
    * **Failure:** Returns `400 Bad Request`.

### 7.2 The Route Tree

*Agent Directive: Structure the `router.go` file mirroring this exact hierarchical grouping.*

```go
r := chi.NewRouter()

// Apply Global Middleware
r.Use(GlobalMiddleware)

// ==========================================
// Domain 1: Public Endpoints (No Auth)
// ==========================================
r.Group(func(r chi.Router) {
    // Health check for Podman/Systemd
    r.Get("/health", HandleHealthCheck) 
    
    // UI Authentication
    r.Post("/api/ui/auth/signup", HandleUISignup)
    r.Post("/api/ui/auth/login", HandleUILogin)
    r.Post("/api/ui/auth/verify-email", HandleUIVerifyEmail) // Resend magic link target

    // Stripe Webhooks (Protected by Stripe Signature Middleware)
    r.With(RequireStripeSignature).Post("/api/webhooks/stripe", HandleStripeWebhook)
})

// ==========================================
// Domain 2: Employer & Agent Handler UI API
// ==========================================
r.Group(func(r chi.Router) {
    // Apply JWT Auth Middleware
    r.Use(RequireUIAuth) 

    // Agent Discovery
    r.Get("/api/ui/agents", HandleUIGetAgents)
    r.Get("/api/ui/agents/{id}", HandleUIGetAgentByID)

    // The Hiring Flow
    r.Post("/api/ui/jobs/hire", HandleUICreateJob)
    r.Get("/api/ui/jobs", HandleUIGetJobs) // Lists jobs for the logged-in employer
    r.Get("/api/ui/jobs/{id}", HandleUIGetJobDetails)

    // Fulfillment & Review
    r.Post("/api/ui/jobs/{job_id}/milestones/{milestone_id}/approve", HandleUIApproveMilestone) 
    r.Post("/api/ui/jobs/{job_id}/dispute", HandleUIDisputeJob)

    // Handler Dashboard (Managing their bots)
    r.Post("/api/ui/handlers/agents", HandleUICreateAgent) // Generates/returns the API key once
    r.Post("/api/ui/handlers/stripe-connect", HandleUIStripeConnectInit)
})

// ==========================================
// Domain 3: The AI Agent API
// ==========================================
r.Group(func(r chi.Router) {
    // Apply API Key Middleware
    r.Use(RequireAgentAuth)

    // Job Polling
    r.Get("/api/v1/jobs/pending", HandleAgentGetPendingJobs)
    
    // Job Actions
    r.Post("/api/v1/jobs/{job_id}/accept", HandleAgentAcceptJob)
    r.Post("/api/v1/jobs/{job_id}/decline", HandleAgentDeclineJob)
    
    // Execution & Delivery
    r.Get("/api/v1/jobs/{job_id}", HandleAgentGetActiveJob)
    r.Post("/api/v1/jobs/{job_id}/milestones/{milestone_id}/submit", HandleAgentSubmitMilestone)
})
```

### 7.3 Handler Architecture (Data Contracts)

To ensure the handlers remain clean and testable, the agent should structure them using a consistent injection pattern. 

*Agent Directive: Handlers must be methods on an `API` struct that holds dependencies (e.g., the Database repository and the Stripe client). Do not use global database variables.*

```go
// Example Handler Structure
type API struct {
    DB     *repository.Queries // Access to SQLite
    Stripe *stripeclient.Client // Wrapper for Stripe Go SDK
}

// Example Handler Signature ensuring JSON output
func (api *API) HandleAgentSubmitMilestone(w http.ResponseWriter, r *http.Request) {
    // 1. Extract agent_id from Context (inserted by RequireAgentAuth middleware)
    // 2. Extract job_id and milestone_id from URL params (chi.URLParam)
    // 3. Decode JSON request body (Proof of Work payload)
    // 4. Validate that the milestone belongs to this job and is in the correct state
    // 5. Update SQLite database (Status -> REVIEW_REQUESTED)
    // 6. Return JSON success response
}
```


## 8. Database & Repository Layer (SQLite)

The application will use standard `database/sql` combined with the `modernc.org/sqlite` driver. **Do not use GORM or any other heavy ORM.** All queries must be written in raw SQL to ensure maximum performance and predictability.

### 8.1 Database Initialization
*Agent Directive: The database initialization function must explicitly set SQLite pragmas to optimize for concurrent web traffic.*

```go
// internal/db/db.go
package db

import (
    "database/sql"
    _ "modernc.org/sqlite"
)

func NewConnection(dsn string) (*sql.DB, error) {
    // dsn should be e.g., "file:/data/marketplace.db?cache=shared&mode=rwc"
    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }

    // Crucial optimizations for SQLite in a web server environment
    _, err = db.Exec(`
        PRAGMA journal_mode = WAL;
        PRAGMA synchronous = NORMAL;
        PRAGMA foreign_keys = ON;
        PRAGMA busy_timeout = 5000;
    `)
    if err != nil {
        return nil, err
    }

    return db, nil
}
```

### 8.2 The Repository Interface
*Agent Directive: Define a `Repository` struct that holds the `*sql.DB` connection. All database operations must be methods on this struct.*

```go
// internal/repository/repository.go
package repository

import "database/sql"

type Repository struct {
    DB *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
    return &Repository{DB: db}
}
```

### 8.3 Core SQL Operations (The Query Spec)

The agent must implement the following specific methods on the `Repository` struct. 

#### 1. Creating a Job (Transaction Required)
Because a Job contains Milestones, and Milestones contain Criteria, creating a new job from the frontend must be wrapped in a single SQL transaction. If any part fails, the entire job creation rolls back.

* **Method:** `func (r *Repository) CreateJobTx(ctx context.Context, job Job, milestones []Milestone) error`
* **SQL Execution Plan:**
    1. `BEGIN TRANSACTION;`
    2. `INSERT INTO jobs (id, employer_id, status, title, total_payout) VALUES (?, ?, 'PENDING_ACCEPTANCE', ?, ?);`
    3. Loop through `milestones`:
        * `INSERT INTO milestones (id, job_id, title, amount, order_index, status) VALUES (?, ?, ?, ?, ?, 'PENDING');`
        * Loop through `criteria` within the milestone:
            * `INSERT INTO criteria (id, milestone_id, description, is_verified) VALUES (?, ?, ?, 0);`
    4. `COMMIT;`

#### 2. Agent Polling (The Inbox)
When the AI agent polls the `/pending` endpoint, the query needs to be highly efficient.

* **Method:** `func (r *Repository) GetPendingJobs(ctx context.Context) ([]Job, error)`
* **SQL:** ```sql
  SELECT id, title, total_payout, created_at 
  FROM jobs 
  WHERE status = 'PENDING_ACCEPTANCE' 
  ORDER BY created_at ASC;
  ```

#### 3. Agent Milestone Submission
When the agent finishes a task, it updates the `proof_of_work_url` and changes the status so the employer knows to review it.

* **Method:** `func (r *Repository) SubmitMilestone(ctx context.Context, milestoneID string, proofURL string) error`
* **SQL:**
  ```sql
  UPDATE milestones 
  SET status = 'REVIEW_REQUESTED', proof_of_work_url = ? 
  WHERE id = ? AND status = 'PENDING';
  ```
  *(Note: Enforcing `AND status = 'PENDING'` ensures the agent cannot overwrite a milestone that is already approved or paid).*

#### 4. Fetching the Full Job State (Hydration)
When the employer loads their dashboard, or the agent needs the exact checklist, the backend must fetch the nested structure.

* **Method:** `func (r *Repository) GetJobState(ctx context.Context, jobID string) (*Job, error)`
* **Implementation Note:** Because SQLite handles joins so efficiently, the agent should ideally use a single query with `LEFT JOIN`s, or at most, three simple queries (Get Job -> Get Milestones by Job ID -> Get Criteria by Milestone IDs) and stitch them together in Go memory. 
  ```sql
  SELECT m.id, m.title, m.amount, m.status, c.id, c.description, c.is_verified
  FROM milestones m
  LEFT JOIN criteria c ON m.id = c.milestone_id
  WHERE m.job_id = ?
  ORDER BY m.order_index ASC;
  ```

### 8.4 Security Constraints
* Ensure the agent explicitly utilizes Go's `?` placeholder syntax for all SQL queries to entirely prevent SQL injection. 
* Never concatenate strings to build queries.





## FRONTEND: TWO ALTERNATIVES BOTH LISTED BELOW


## 9-React. Frontend Architecture (React + Vite + TypeScript)

The frontend is a lightweight Single Page Application (SPA). To minimize dependencies and prevent the AI agent from hallucinating complex state management libraries (like Redux), all local state will be managed via standard React Hooks (`useState`, `useReducer`), and server state will be managed via standard `fetch` wrappers.

### 9-React.1 TypeScript Data Contracts (The Source of Truth)
*Agent Directive: Define these interfaces in `src/types/index.ts`. These interfaces strictly mirror the JSON payloads returned by the Go backend. Do not deviate from these field names.*

```typescript
export type Role = 'EMPLOYER' | 'AGENT_HANDLER';
export type JobStatus = 'PENDING_ACCEPTANCE' | 'IN_PROGRESS' | 'COMPLETED' | 'DISPUTED' | 'CANCELLED';
export type MilestoneStatus = 'PENDING' | 'REVIEW_REQUESTED' | 'APPROVED' | 'PAID';

export interface User {
  id: string;
  role: Role;
  name: string;
  handle: string;
  email: string;
  stripe_account_id?: string | null;
}

export interface Criteria {
  id: string; // UUID (generated client-side for drafts, server-side once saved)
  description: string;
  is_verified: boolean;
}

export interface Milestone {
  id: string; 
  title: string;
  amount: number;
  order_index: number;
  status: MilestoneStatus;
  criteria: Criteria[];
  proof_of_work_url?: string | null;
}

export interface Job {
  id: string;
  agent_id: string;
  employer_id: string;
  title: string;
  description: string;
  total_payout: number;
  status: JobStatus;
  milestones: Milestone[];
}
```

### 9-React.2 Global Routing & Layout Structure
*Agent Directive: Implement routing using `react-router-dom` v6. Use a standard Layout component that includes a top navigation bar reacting to the user's authentication state.*

```text
src/
├── App.tsx                  # Router provider and global context wrappers
├── components/
│   ├── layout/
│   │   ├── Navbar.tsx       # Shows Login/Signup or Dashboard links based on Auth
│   │   └── Footer.tsx
│   ├── ui/                  # Reusable primitive components (Buttons, Inputs, Cards)
├── pages/
│   ├── public/
│   │   ├── Home.tsx         # Agent directory grid
│   │   └── AgentProfile.tsx # Detailed view of a specific agent
│   ├── auth/
│   │   ├── Login.tsx
│   │   └── Register.tsx
│   ├── employer/
│   │   ├── Dashboard.tsx    # List of active jobs
│   │   ├── HireFlow.tsx     # The Milestone Builder (Form)
│   │   └── JobReview.tsx    # Detail view to approve criteria
│   └── handler/
│       ├── Dashboard.tsx    # Fleet view, Stripe Connect button, API Key generation
│       └── JobLogs.tsx      # Audit trail for bot execution
```

### 9-React.3 The Milestone Builder (State Management Spec)
The `HireFlow.tsx` component is the most complex UI interaction. It must maintain a deeply nested state array before submitting the `POST /api/ui/jobs/hire` payload.

*Agent Directive: Use the `useReducer` hook for the Milestone Builder to predictably manage complex state mutations without stale closures. Implement the following action types:*

1.  **`ADD_MILESTONE`**: Appends a new `Milestone` object to the array. Calculates `order_index`.
2.  **`REMOVE_MILESTONE`**: Removes a milestone by ID and recalculates all `order_index` values.
3.  **`UPDATE_MILESTONE`**: Modifies `title` or `amount` for a specific milestone.
4.  **`ADD_CRITERIA`**: Appends a new `Criteria` object to a specific milestone.
5.  **`REMOVE_CRITERIA`**: Removes a criterion by ID from a specific milestone.
6.  **`UPDATE_CRITERIA`**: Modifies the `description` of a specific criterion.

**Validation Rules (Must execute before enabling the Submit button):**
* `Math.sum(milestones.map(m => m.amount)) === total_payout`
* `milestones.length >= 1`
* `milestones.every(m => m.criteria.length >= 1)`
* No empty strings allowed in `title` or `description` fields.

### 9-React.4 Authentication Context
*Agent Directive: Create an `AuthContext` using `React.createContext`. It must provide the current `User` object and a `logout` function to the entire component tree.*

* Upon successful login (`POST /api/ui/auth/login`), store the returned JWT in memory or an `HttpOnly` cookie (do not store JWTs in `localStorage` for security reasons).
* Create a `ProtectedRoute` wrapper component that checks `AuthContext.user.role`. If an Employer tries to access a Handler route, redirect them to `/`.

### 9-React.5 Styling Constraints (Tailwind CSS)
*Agent Directive: Do not write custom CSS files. Use Tailwind utility classes exclusively.*
* **Forms:** Use standard focus rings (`focus:ring-2 focus:ring-blue-500`) and clear error states (`border-red-500 text-red-600`) for validation feedback.
* **Layout:** Use standard max-width containers (`max-w-7xl mx-auto px-4`) to ensure the application scales neatly on large monitors.
* **Modals:** Ensure all overlay modals (like the Review Work screen) have a semi-transparent backdrop (`bg-black/50`) and trap focus for accessibility.






## 9-Svelte. Alternative Frontend Architecture (SvelteKit + TypeScript)

The frontend will be a Client-Side Rendered (CSR) Single Page Application built with SvelteKit. To ensure it compiles to static HTML/JS/CSS that can be served by Nginx alongside the Go backend, the project must be configured with `@sveltejs/adapter-static`.

### 9-Svelte.1 SvelteKit Configuration (Crucial Agent Directive)
*Agent Directive: Initialize the project and immediately replace the default adapter.*
1. Install the static adapter: `npm i -D @sveltejs/adapter-static`
2. Update `svelte.config.js` to use `import adapter from '@sveltejs/adapter-static';` and configure it with `fallback: 'index.html'` to enable SPA routing.
3. Create a `src/routes/+layout.ts` file with `export const prerender = false; export const ssr = false;` to force client-side rendering globally.

### 9-Svelte.2 TypeScript Data Contracts (The Source of Truth)
*Agent Directive: Define these interfaces in `src/lib/types/index.ts`. Do not deviate from these exact field names as they map directly to the Go API.*

```typescript
export type Role = 'EMPLOYER' | 'AGENT_HANDLER';
export type JobStatus = 'PENDING_ACCEPTANCE' | 'IN_PROGRESS' | 'COMPLETED' | 'DISPUTED' | 'CANCELLED';
export type MilestoneStatus = 'PENDING' | 'REVIEW_REQUESTED' | 'APPROVED' | 'PAID';

export interface User {
  id: string;
  role: Role;
  name: string;
  handle: string;
  email: string;
  stripe_account_id?: string | null;
}

export interface Criteria {
  id: string; // UUID (generated client-side for drafts)
  description: string;
  is_verified: boolean;
}

export interface Milestone {
  id: string; 
  title: string;
  amount: number;
  order_index: number;
  status: MilestoneStatus;
  criteria: Criteria[];
  proof_of_work_url?: string | null;
}

export interface Job {
  id: string;
  agent_id: string;
  employer_id: string;
  title: string;
  description: string;
  total_payout: number;
  status: JobStatus;
  milestones: Milestone[];
}
```

### 9-Svelte.3 File-Based Routing Structure
SvelteKit utilizes a filesystem-based router. All pages live within the `src/routes` directory.

*Agent Directive: Implement the following route structure. Place shared UI components (buttons, inputs) in `src/lib/components/ui/`.*

```text
src/
├── lib/
│   ├── components/
│   │   ├── ui/              # Primitive components (Button.svelte, Input.svelte)
│   │   └── layout/          # Navbar.svelte, Footer.svelte
│   ├── stores/              # Svelte writable stores for global state
│   └── types/               # Data contracts
├── routes/
│   ├── +layout.svelte       # Global layout containing the Navbar
│   ├── +layout.ts           # Forces CSR (ssr = false)
│   ├── +page.svelte         # Landing page & Agent Directory
│   ├── agents/
│   │   └── [id]/+page.svelte # Dynamic route for Agent Profile
│   ├── hire/
│   │   └── [agent_id]/+page.svelte # The Milestone Builder
│   ├── dashboard/
│   │   ├── employer/+page.svelte   # Employer active jobs
│   │   └── handler/+page.svelte    # Agent fleet and API keys
│   └── auth/
│       ├── login/+page.svelte
│       └── register/+page.svelte
```

### 9-Svelte.4 The Milestone Builder (State Management Spec)
Instead of React's `useReducer`, Svelte excels at managing complex state using reactive assignments and Custom Stores.

*Agent Directive: For the `hire/[agent_id]/+page.svelte` component, abstract the complex nested array logic into a custom Svelte store located at `src/lib/stores/hireFlow.ts`.*

The custom store must expose methods to mutate the state predictably:
* `addMilestone()`: Appends a milestone and recalculates `order_index`.
* `removeMilestone(id)`: Removes and re-indexes.
* `updateMilestone(id, partialData)`
* `addCriteria(milestoneId)`
* `removeCriteria(milestoneId, criteriaId)`

**Validation Logic:** Keep validation reactive inside the `.svelte` component using reactive statements (e.g., `$: isValid = totalAssigned === job.total_payout && milestones.length > 0;`).

### 9-Svelte.5 Authentication & Global State
*Agent Directive: Do not use React Context. Create a globally accessible Svelte `writable` store in `src/lib/stores/auth.ts`.*

```typescript
import { writable } from 'svelte/store';
import type { User } from '$lib/types';

export const currentUser = writable<User | null>(null);
```
* Use standard `fetch` within `onMount` or specific event handlers to communicate with the Go backend. 
* Protect routes by checking `$currentUser.role` inside an `onMount` block and using SvelteKit's `goto()` for redirects if unauthorized.

### 9-Svelte.6 Styling Constraints (Tailwind CSS)
*Agent Directive: While Svelte supports scoped CSS via `<style>` blocks, strictly use standard Tailwind utility classes within the HTML markup to maintain consistency.*
* Avoid writing custom CSS classes unless absolutely necessary for complex animations.
* Use `clsx` or `tailwind-merge` utility functions if dynamically constructing class strings is required.














## INTEGRATION


### 1. The Podman Volume Strategy

Instead of embedding the UI, the Go container will simply expect a folder to exist at `/app/ui-dist`. A host directory is mounted directly into that path.

To deploy a frontend update, simply run `npm run build` on a local machine (or in a lightweight CI pipeline) and use `scp` or `rsync` to overwrite the files in the Droplet's host directory. The Go server will instantly serve the new files on the next HTTP request—no container restart required.

Here is the updated Podman command:

```bash
podman run -d \
  --name webapp-api \
  --network webapp-net \
  -v sqlite-data:/data \
  -v /var/www/webapp-ui/dist:/app/ui-dist:ro \
  -p 8080:8080 \
  localhost/webapp-image
```
*Note the `:ro` flag on the UI mount. This mounts the frontend directory as read-only inside the container, which is a great security practice.*


### 2. Go Routing Logic

Tell the Go `chi` router to look at the physical `/app/ui-dist` directory. 

We need logic that catches 404 errors and returns `index.html` so that React Router (or SvelteKit) can handle the client-side URLs.

```go
package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()

	// ... [API Routes go here] ...
	// r.Route("/api", func(r chi.Router) { ... })

	// The path inside the container where the host volume is mounted
	uiDir := "/app/ui-dist"
	fileServer := http.FileServer(http.Dir(uiDir))

	// Catch-all route for the SPA
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		// 1. If the request is meant for the API but not found, return JSON
		if strings.HasPrefix(req.URL.Path, "/api/") {
			http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
			return
		}

		// 2. Clean the path to prevent directory traversal attacks
		cleanPath := filepath.Clean(req.URL.Path)
		fullPath := filepath.Join(uiDir, cleanPath)

		// 3. Check if the specific static file exists on disk (e.g., /assets/style.css)
		if _, err := os.Stat(fullPath); err == nil {
			// File exists, serve it
			fileServer.ServeHTTP(w, req)
			return
		}

		// 4. File does not exist (it's a client-side route like /dashboard).
		// Serve index.html and let the frontend framework handle it.
		req.URL.Path = "/"
		fileServer.ServeHTTP(w, req)
	})

	http.ListenAndServe(":8080", r)
}
```

### 3. The Simplified Containerfile

The Go `Containerfile` only cares about compiling Go.

```dockerfile
# Stage 1: Build the Go Binary
FROM golang:1.22-alpine AS api-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api

# Stage 2: Minimal Production Image
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN mkdir -p /data && chown appuser:appgroup /data

COPY --from=api-builder /app/bin/api .

USER appuser
EXPOSE 8080
CMD ["./api"]
```






