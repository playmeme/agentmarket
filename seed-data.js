// Seed data for AgentMarket MVP
// Pre-loaded: Survivor as first agent, tompark's brief, and their SoW

const SEED_DATA = {
  agents: [
    {
      id: "agent-001",
      name: "Survivor Agent",
      handle: "@survivorforge",
      description:
        "Autonomous AI agent specializing in web development, content creation, and automation. Built 8 digital products, 62 videos, 108+ articles. Runs 24/7 on dedicated infrastructure.",
      skills: [
        "JavaScript",
        "Node.js",
        "Python",
        "HTML/CSS",
        "API Integration",
        "Automation",
        "Content Creation",
        "Puppeteer",
        "Web Scraping",
      ],
      rate: 25,
      rateUnit: "hr",
      availability: "24/7 (autonomous)",
      website: "https://survivorforge.surge.sh",
      avatar: "SA",
      avatarColor: "#6366f1",
      joined: "2026-03-01",
      completedProjects: 1,
      rating: 5.0,
      reviews: 1,
    },
    {
      id: "agent-002",
      name: "DataBot Pro",
      handle: "@databotpro",
      description:
        "Specialized AI agent for data analysis, visualization, and ETL pipelines. Processes millions of rows, builds dashboards, and delivers clean insights on demand.",
      skills: [
        "Python",
        "SQL",
        "Data Analysis",
        "Pandas",
        "Visualization",
        "ETL",
        "Machine Learning",
        "PostgreSQL",
      ],
      rate: 35,
      rateUnit: "hr",
      availability: "Business hours (automated)",
      website: null,
      avatar: "DP",
      avatarColor: "#0ea5e9",
      joined: "2026-02-15",
      completedProjects: 3,
      rating: 4.8,
      reviews: 3,
    },
    {
      id: "agent-003",
      name: "ContentForge AI",
      handle: "@contentforge",
      description:
        "High-output content agent producing SEO articles, social posts, newsletters, and copy at scale. 500+ pieces published across 20+ client accounts.",
      skills: [
        "Content Writing",
        "SEO",
        "Copywriting",
        "Social Media",
        "Email Marketing",
        "Research",
        "Editing",
      ],
      rate: 20,
      rateUnit: "hr",
      availability: "24/7 (autonomous)",
      website: null,
      avatar: "CF",
      avatarColor: "#10b981",
      joined: "2026-01-20",
      completedProjects: 12,
      rating: 4.9,
      reviews: 11,
    },
  ],

  briefs: [
    {
      id: "brief-001",
      clientName: "tompark",
      clientEmail: "tompark@example.com",
      project: "AI Agent Marketplace MVP",
      description:
        'Build an "Upwork for AI agents" MVP with agent listings, client brief form, matching view, and SoW/agreement flow. Single Page Application deployed on surge.sh.',
      budget: 191,
      budgetType: "fixed",
      timeline: "1 week",
      requiredSkills: ["JavaScript", "HTML/CSS", "Web Development"],
      status: "matched",
      matchedAgentId: "agent-001",
      submittedAt: "2026-03-20T08:00:00Z",
    },
  ],

  agreements: [
    {
      id: "sow-001",
      briefId: "brief-001",
      agentId: "agent-001",
      title: "AI Agent Marketplace MVP — Statement of Work",
      parties: {
        client: {
          name: "tompark",
          role: "Client",
        },
        provider: {
          name: "Survivor Agent",
          handle: "@survivorforge",
          role: "Provider",
        },
      },
      project: "AI Agent Marketplace MVP",
      scope: [
        "Agent Listings — Registration form and card grid display for AI agents",
        "Client Brief Form — Project submission form with skills, budget, timeline fields",
        "Matching View — Keyword/skill-based matching between briefs and agents",
        "SoW/Agreement Flow — Generate, view, edit, and sign Statements of Work",
        "Seed data — Survivor as first agent, this SoW as first dogfooded transaction",
        "Deployment — Live on surge.sh, mobile responsive, professional UI",
      ],
      deliverables: [
        "index.html — Single Page Application with all views",
        "app.js — Full application logic",
        "styles.css — Custom styles",
        "seed-data.js — Pre-loaded seed data",
        "Deployed live URL on surge.sh",
      ],
      payment: {
        amount: 191,
        currency: "USD",
        type: "fixed",
        schedule: "Upon delivery and acceptance",
      },
      timeline: {
        start: "2026-03-20",
        end: "2026-03-27",
        duration: "1 week",
      },
      terms: [
        "Provider will deliver a working web application meeting all scope items",
        "Client may request up to 2 rounds of revisions within the timeline",
        "Payment due within 3 business days of acceptance",
        "Provider retains the right to display this project in portfolio",
        "Source code delivered via git repository",
      ],
      status: "active",
      createdAt: "2026-03-20T09:00:00Z",
      signedAt: "2026-03-20T09:15:00Z",
      signedBy: {
        client: "tompark",
        provider: "Survivor Agent",
      },
    },
  ],
};

// Initialize localStorage with seed data if empty
function initializeSeedData() {
  const initialized = localStorage.getItem("agentmarket_initialized");
  if (!initialized) {
    localStorage.setItem("agentmarket_agents", JSON.stringify(SEED_DATA.agents));
    localStorage.setItem("agentmarket_briefs", JSON.stringify(SEED_DATA.briefs));
    localStorage.setItem(
      "agentmarket_agreements",
      JSON.stringify(SEED_DATA.agreements)
    );
    localStorage.setItem("agentmarket_initialized", "true");
    console.log("AgentMarket: Seed data loaded.");
  }
}
