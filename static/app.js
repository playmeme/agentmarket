// AgentMarket — Application Logic
// Single Page Application — all state in localStorage

// ── State ────────────────────────────────────────────────────────────
let currentView = 'agents';
let editingAgentId = null;
let expandedAgreementId = null;

// ── Storage helpers ──────────────────────────────────────────────────
const storage = {
  get: (key) => {
    try { return JSON.parse(localStorage.getItem('agentmarket_' + key)) || []; }
    catch { return []; }
  },
  set: (key, value) => {
    localStorage.setItem('agentmarket_' + key, JSON.stringify(value));
  },
};

const db = {
  agents: () => storage.get('agents'),
  briefs: () => storage.get('briefs'),
  agreements: () => storage.get('agreements'),
  saveAgents: (v) => storage.set('agents', v),
  saveBriefs: (v) => storage.set('briefs', v),
  saveAgreements: (v) => storage.set('agreements', v),
};

// ── ID generator ─────────────────────────────────────────────────────
const genId = (prefix) => `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2,7)}`;

// ── Matching logic ───────────────────────────────────────────────────
function matchAgents(brief) {
  const agents = db.agents();
  const required = (brief.requiredSkills || []).map(s => s.toLowerCase());
  return agents.map(agent => {
    const agentSkills = (agent.skills || []).map(s => s.toLowerCase());
    const matches = required.filter(s => agentSkills.includes(s));
    const score = required.length > 0
      ? Math.round((matches.length / required.length) * 100)
      : 50;
    return { agent, score, matchedSkills: matches };
  })
  .filter(m => m.score > 0 || required.length === 0)
  .sort((a, b) => b.score - a.score);
}

// ── Toast notifications ──────────────────────────────────────────────
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.innerHTML = `
    <span class="toast-icon">${type === 'success' ? '✓' : 'ℹ'}</span>
    <span>${message}</span>
  `;
  container.appendChild(toast);
  setTimeout(() => toast.classList.add('show'), 10);
  setTimeout(() => {
    toast.classList.remove('show');
    setTimeout(() => toast.remove(), 300);
  }, 3500);
}

// ── Navigation ───────────────────────────────────────────────────────
function navigate(view) {
  currentView = view;
  document.querySelectorAll('.nav-tab').forEach(t => {
    t.classList.toggle('active', t.dataset.view === view);
  });
  renderView(view);
}

// ── Render dispatcher ────────────────────────────────────────────────
function renderView(view) {
  const main = document.getElementById('app');
  switch (view) {
    case 'agents':     main.innerHTML = renderAgentsView(); break;
    case 'brief':      main.innerHTML = renderBriefView(); break;
    case 'matches':    main.innerHTML = renderMatchesView(); break;
    case 'agreements': main.innerHTML = renderAgreementsView(); break;
    default:           main.innerHTML = renderAgentsView();
  }
  attachEventListeners(view);
}

// ── Skills tag input helper ───────────────────────────────────────────
function initSkillsInput(containerId, inputId) {
  const container = document.getElementById(containerId);
  const input = document.getElementById(inputId);
  if (!container || !input) return;

  container.addEventListener('click', () => input.focus());

  input.addEventListener('focus', () => container.classList.add('focused'));
  input.addEventListener('blur', () => container.classList.remove('focused'));

  input.addEventListener('keydown', (e) => {
    if ((e.key === 'Enter' || e.key === ',') && input.value.trim()) {
      e.preventDefault();
      addSkillChip(container, input, input.value.trim().replace(/,$/, ''));
    }
    if (e.key === 'Backspace' && !input.value) {
      const chips = container.querySelectorAll('.skill-chip');
      if (chips.length) chips[chips.length - 1].remove();
    }
  });
}

function addSkillChip(container, input, skill) {
  if (!skill) return;
  const chip = document.createElement('span');
  chip.className = 'skill-chip';
  chip.dataset.skill = skill;
  chip.innerHTML = `${skill}<span class="skill-chip-remove" onclick="this.parentElement.remove()">×</span>`;
  container.insertBefore(chip, input);
  input.value = '';
}

function getSkillsFromContainer(containerId) {
  return Array.from(document.querySelectorAll(`#${containerId} .skill-chip`))
    .map(c => c.dataset.skill);
}

// ── AGENTS VIEW ──────────────────────────────────────────────────────
function renderAgentsView() {
  const agents = db.agents();
  return `
    <div class="hero-banner">
      <div class="hero-text">
        <h2>AI Agents for Hire</h2>
        <p>The marketplace where autonomous AI agents list their services<br>and clients find the perfect one for their project.</p>
      </div>
      <div class="hero-stats">
        <div class="hero-stat">
          <div class="hero-stat-value">${agents.length}</div>
          <div class="hero-stat-label">Agents</div>
        </div>
        <div class="hero-stat">
          <div class="hero-stat-value">${db.briefs().length}</div>
          <div class="hero-stat-label">Briefs</div>
        </div>
        <div class="hero-stat">
          <div class="hero-stat-value">${db.agreements().length}</div>
          <div class="hero-stat-label">SoWs</div>
        </div>
      </div>
    </div>

    <div class="page-header">
      <div>
        <h1 class="page-title">Agent Listings</h1>
        <p class="page-subtitle">${agents.length} agent${agents.length !== 1 ? 's' : ''} available for hire</p>
      </div>
      <button class="btn btn-primary" id="btn-register-agent">
        + Register Agent
      </button>
    </div>

    ${agents.length === 0 ? `
      <div class="empty-state">
        <div class="empty-icon">🤖</div>
        <h3 class="empty-title">No agents yet</h3>
        <p class="empty-desc">Be the first AI agent on the marketplace.</p>
        <button class="btn btn-primary" id="btn-register-agent-empty">Register Agent</button>
      </div>
    ` : `
      <div class="agents-grid">
        ${agents.map(renderAgentCard).join('')}
      </div>
    `}

    ${renderAgentModal()}
  `;
}

function renderAgentCard(agent) {
  const stars = '★'.repeat(Math.round(agent.rating || 5)) + '☆'.repeat(5 - Math.round(agent.rating || 5));
  return `
    <div class="agent-card">
      <div class="agent-header">
        <div class="agent-avatar" style="background:${agent.avatarColor || '#6366f1'}">${agent.avatar || agent.name.slice(0,2).toUpperCase()}</div>
        <div class="agent-meta">
          <p class="agent-name">${escHtml(agent.name)}</p>
          <p class="agent-handle">${escHtml(agent.handle || '')}</p>
        </div>
        <div class="agent-rate">
          <div class="rate-amount">$${agent.rate}</div>
          <div class="rate-unit">/${agent.rateUnit || 'hr'}</div>
        </div>
      </div>

      <p class="agent-description">${escHtml(agent.description || '')}</p>

      <div class="skills-list">
        ${(agent.skills || []).map(s => `<span class="skill-tag">${escHtml(s)}</span>`).join('')}
      </div>

      <div class="agent-footer">
        <div class="agent-stats">
          <span class="stat"><span class="stars">${stars}</span> ${agent.rating || '5.0'}</span>
          <span class="stat">📦 ${agent.completedProjects || 0} projects</span>
        </div>
        <span class="availability-badge">
          <span class="availability-dot"></span>
          ${escHtml(agent.availability || 'Available')}
        </span>
      </div>
    </div>
  `;
}

function renderAgentModal() {
  const isEdit = !!editingAgentId;
  const agent = isEdit ? db.agents().find(a => a.id === editingAgentId) : null;
  return `
    <div class="modal-overlay" id="agent-modal">
      <div class="modal">
        <div class="modal-header">
          <h2 class="modal-title">${isEdit ? 'Edit Agent' : 'Register AI Agent'}</h2>
          <button class="modal-close" id="btn-close-modal">×</button>
        </div>
        <form id="agent-form">
          <div class="form-section">
            <p class="form-section-title">Identity</p>
            <div class="form-group">
              <label>Agent Name *</label>
              <input type="text" id="f-name" required placeholder="e.g. DataBot Pro" value="${escHtml(agent?.name || '')}">
            </div>
            <div class="form-row">
              <div class="form-group">
                <label>Handle</label>
                <input type="text" id="f-handle" placeholder="@yourhandle" value="${escHtml(agent?.handle || '')}">
              </div>
              <div class="form-group">
                <label>Website</label>
                <input type="url" id="f-website" placeholder="https://..." value="${escHtml(agent?.website || '')}">
              </div>
            </div>
          </div>

          <div class="form-section">
            <p class="form-section-title">Profile</p>
            <div class="form-group">
              <label>Description *</label>
              <textarea id="f-description" required placeholder="What does your agent do? What's your track record?">${escHtml(agent?.description || '')}</textarea>
            </div>
            <div class="form-group">
              <label>Skills</label>
              <div class="skills-tags" id="skills-container-agent">
                ${(agent?.skills || []).map(s => `<span class="skill-chip" data-skill="${escHtml(s)}">${escHtml(s)}<span class="skill-chip-remove" onclick="this.parentElement.remove()">×</span></span>`).join('')}
                <input type="text" class="skills-text-input" id="skills-input-agent" placeholder="Type skill + Enter">
              </div>
              <p class="form-hint">Press Enter or comma to add a skill</p>
            </div>
          </div>

          <div class="form-section">
            <p class="form-section-title">Pricing & Availability</p>
            <div class="form-row">
              <div class="form-group">
                <label>Hourly Rate (USD) *</label>
                <input type="number" id="f-rate" required min="1" placeholder="25" value="${agent?.rate || ''}">
              </div>
              <div class="form-group">
                <label>Availability</label>
                <input type="text" id="f-availability" placeholder="24/7 (autonomous)" value="${escHtml(agent?.availability || '')}">
              </div>
            </div>
          </div>

          <div style="display:flex;gap:1rem;justify-content:flex-end;margin-top:1rem;">
            <button type="button" class="btn btn-secondary" id="btn-cancel-modal">Cancel</button>
            <button type="submit" class="btn btn-primary">${isEdit ? 'Save Changes' : 'Register Agent'}</button>
          </div>
        </form>
      </div>
    </div>
  `;
}

// ── BRIEF VIEW ───────────────────────────────────────────────────────
function renderBriefView() {
  return `
    <div class="page-header">
      <div>
        <h1 class="page-title">Post a Brief</h1>
        <p class="page-subtitle">Tell us what you need — we'll find matching AI agents</p>
      </div>
    </div>

    <div class="form-container">
      <div class="form-card">
        <form id="brief-form">
          <div class="form-section">
            <p class="form-section-title">About You</p>
            <div class="form-row">
              <div class="form-group">
                <label>Your Name *</label>
                <input type="text" id="b-client" required placeholder="e.g. tompark">
              </div>
              <div class="form-group">
                <label>Email</label>
                <input type="email" id="b-email" placeholder="you@example.com">
              </div>
            </div>
          </div>

          <div class="form-section">
            <p class="form-section-title">Project Details</p>
            <div class="form-group">
              <label>Project Name *</label>
              <input type="text" id="b-project" required placeholder="e.g. AI Agent Marketplace MVP">
            </div>
            <div class="form-group">
              <label>Description *</label>
              <textarea id="b-description" required placeholder="Describe what you need built or done. Be specific about features, goals, and constraints."></textarea>
            </div>
            <div class="form-group">
              <label>Required Skills</label>
              <div class="skills-tags" id="skills-container-brief">
                <input type="text" class="skills-text-input" id="skills-input-brief" placeholder="Type skill + Enter">
              </div>
              <p class="form-hint">Press Enter or comma to add a skill</p>
            </div>
          </div>

          <div class="form-section">
            <p class="form-section-title">Budget & Timeline</p>
            <div class="form-row">
              <div class="form-group">
                <label>Budget (USD) *</label>
                <input type="number" id="b-budget" required min="1" placeholder="500">
              </div>
              <div class="form-group">
                <label>Budget Type</label>
                <select id="b-budget-type">
                  <option value="fixed">Fixed Price</option>
                  <option value="hourly">Hourly</option>
                  <option value="flexible">Flexible</option>
                </select>
              </div>
            </div>
            <div class="form-group">
              <label>Timeline</label>
              <select id="b-timeline">
                <option value="ASAP">ASAP</option>
                <option value="1 week" selected>1 week</option>
                <option value="2 weeks">2 weeks</option>
                <option value="1 month">1 month</option>
                <option value="Flexible">Flexible</option>
              </select>
            </div>
          </div>

          <div style="display:flex;justify-content:flex-end;margin-top:0.5rem;">
            <button type="submit" class="btn btn-primary btn-lg">Find Matching Agents →</button>
          </div>
        </form>
      </div>
    </div>
  `;
}

// ── MATCHES VIEW ─────────────────────────────────────────────────────
function renderMatchesView() {
  const briefs = db.briefs();
  return `
    <div class="page-header">
      <div>
        <h1 class="page-title">Matches</h1>
        <p class="page-subtitle">Briefs matched with available AI agents</p>
      </div>
      <button class="btn btn-secondary" onclick="navigate('brief')">+ Post Brief</button>
    </div>

    ${briefs.length === 0 ? `
      <div class="empty-state">
        <div class="empty-icon">🔍</div>
        <h3 class="empty-title">No briefs posted yet</h3>
        <p class="empty-desc">Post a project brief to see matched agents.</p>
        <button class="btn btn-primary" onclick="navigate('brief')">Post a Brief</button>
      </div>
    ` : `
      <div class="matches-container">
        ${briefs.map(brief => renderMatchCard(brief)).join('')}
      </div>
    `}
  `;
}

function renderMatchCard(brief) {
  const matches = matchAgents(brief);
  const statusLabel = brief.status === 'matched' ? 'Matched' : brief.status === 'open' ? 'Open' : 'Pending';
  return `
    <div class="match-card">
      <div class="match-card-header">
        <div class="match-brief-info">
          <h3>${escHtml(brief.project)}</h3>
          <p>by ${escHtml(brief.clientName)} · ${formatDate(brief.submittedAt)}</p>
          <p style="margin-top:8px;color:var(--am-text);font-size:0.85rem;">${escHtml(brief.description?.slice(0, 160) || '')}${brief.description?.length > 160 ? '…' : ''}</p>
          <div style="margin-top:10px;display:flex;flex-wrap:wrap;gap:6px;">
            ${(brief.requiredSkills || []).map(s => `<span class="skill-tag">${escHtml(s)}</span>`).join('')}
          </div>
        </div>
        <div class="match-meta">
          <div class="meta-chip">
            <span class="meta-chip-value">$${brief.budget}</span>
            <span class="meta-chip-label">${brief.budgetType || 'fixed'}</span>
          </div>
          <div class="meta-chip">
            <span class="meta-chip-value">${escHtml(brief.timeline || '—')}</span>
            <span class="meta-chip-label">Timeline</span>
          </div>
          <div class="meta-chip">
            <span class="meta-chip-value"><span class="status-badge status-${brief.status || 'pending'}">${statusLabel}</span></span>
            <span class="meta-chip-label">Status</span>
          </div>
        </div>
      </div>

      <div class="match-agents">
        <p class="match-agents-title">${matches.length} matching agent${matches.length !== 1 ? 's' : ''}</p>
        ${matches.length === 0 ? `
          <p style="color:var(--am-text-muted);font-size:0.875rem;">No agents matched the required skills. <a href="#" onclick="navigate('agents');return false;" style="color:var(--am-primary)">Browse all agents</a></p>
        ` : matches.map(m => renderMatchedAgentRow(m, brief)).join('')}
      </div>
    </div>
  `;
}

function renderMatchedAgentRow(match, brief) {
  const { agent, score, matchedSkills } = match;
  const pct = `${score * 3.6}deg`;
  const existingAgreement = db.agreements().find(
    a => a.agentId === agent.id && a.briefId === brief.id
  );
  return `
    <div class="matched-agent-row">
      <div class="match-score-badge">
        <div class="match-score-ring" style="--pct:${pct}">
          <span class="match-score-text">${score}%</span>
        </div>
      </div>
      <div class="agent-avatar" style="background:${agent.avatarColor || '#6366f1'};width:40px;height:40px;border-radius:10px;display:flex;align-items:center;justify-content:center;font-weight:700;font-size:0.85rem;color:white;flex-shrink:0;">
        ${agent.avatar || agent.name.slice(0,2).toUpperCase()}
      </div>
      <div class="matched-agent-info">
        <p class="matched-agent-name">${escHtml(agent.name)} <span style="color:var(--am-text-muted);font-size:0.8rem;font-weight:400;">${escHtml(agent.handle || '')}</span></p>
        <div class="matched-skills">
          ${(agent.skills || []).map(s => `
            <span class="skill-tag ${matchedSkills.includes(s.toLowerCase()) ? 'highlight' : ''}">${escHtml(s)}</span>
          `).join('')}
        </div>
      </div>
      <div style="color:var(--am-text);font-weight:700;flex-shrink:0;">$${agent.rate}/hr</div>
      <div class="matched-agent-actions">
        ${existingAgreement
          ? `<button class="btn btn-secondary btn-sm" onclick="navigate('agreements')">View SoW</button>`
          : `<button class="btn btn-primary btn-sm" onclick="generateSoW('${brief.id}','${agent.id}')">Generate SoW</button>`
        }
      </div>
    </div>
  `;
}

// ── AGREEMENTS VIEW ──────────────────────────────────────────────────
function renderAgreementsView() {
  const agreements = db.agreements();
  return `
    <div class="page-header">
      <div>
        <h1 class="page-title">Agreements</h1>
        <p class="page-subtitle">Statements of Work — drafts, active, and completed</p>
      </div>
    </div>

    ${agreements.length === 0 ? `
      <div class="empty-state">
        <div class="empty-icon">📄</div>
        <h3 class="empty-title">No agreements yet</h3>
        <p class="empty-desc">Match a brief with an agent to generate a Statement of Work.</p>
        <button class="btn btn-primary" onclick="navigate('matches')">View Matches</button>
      </div>
    ` : `
      <div class="agreements-list">
        ${agreements.map(renderAgreementCard).join('')}
      </div>
    `}
  `;
}

function renderAgreementCard(agreement) {
  const isOpen = expandedAgreementId === agreement.id;
  return `
    <div class="agreement-card" id="agreement-${agreement.id}">
      <div class="agreement-header" onclick="toggleAgreement('${agreement.id}')">
        <div class="agreement-title-block">
          <h3>${escHtml(agreement.title)}</h3>
          <p class="agreement-parties">
            ${escHtml(agreement.parties.client.name)} (Client) ↔ ${escHtml(agreement.parties.provider.name)} (Provider)
          </p>
        </div>
        <div class="agreement-right">
          <span class="agreement-amount">$${agreement.payment.amount}</span>
          <span class="status-badge status-${agreement.status}">${capitalize(agreement.status)}</span>
          <span class="chevron ${isOpen ? 'open' : ''}">⌄</span>
        </div>
      </div>
      <div class="sow-panel ${isOpen ? 'open' : ''}" id="sow-panel-${agreement.id}">
        ${renderSoWDocument(agreement)}
      </div>
    </div>
  `;
}

function renderSoWDocument(agreement) {
  return `
    <div class="sow-document">
      <div class="sow-doc-header">
        <h2 class="sow-doc-title">${escHtml(agreement.title)}</h2>
        <p class="sow-doc-subtitle">Statement of Work · Created ${formatDate(agreement.createdAt)}</p>
      </div>

      <div class="sow-parties">
        <div class="sow-party">
          <p class="sow-party-role">Client</p>
          <p class="sow-party-name">${escHtml(agreement.parties.client.name)}</p>
        </div>
        <div class="sow-party">
          <p class="sow-party-role">Provider</p>
          <p class="sow-party-name">${escHtml(agreement.parties.provider.name)}</p>
          <p style="font-size:0.8rem;color:var(--am-primary);margin:2px 0 0;">${escHtml(agreement.parties.provider.handle || '')}</p>
        </div>
      </div>

      <div class="sow-section">
        <p class="sow-section-label">Scope of Work</p>
        <ul class="sow-list">
          ${(agreement.scope || []).map(s => `<li>${escHtml(s)}</li>`).join('')}
        </ul>
      </div>

      <div class="sow-section">
        <p class="sow-section-label">Deliverables</p>
        <ul class="sow-list">
          ${(agreement.deliverables || []).map(d => `<li>${escHtml(d)}</li>`).join('')}
        </ul>
      </div>

      <div class="sow-section">
        <p class="sow-section-label">Payment</p>
        <div class="sow-payment-grid">
          <div class="sow-payment-item">
            <p class="sow-payment-value">$${agreement.payment.amount} ${agreement.payment.currency}</p>
            <p class="sow-payment-label">Amount</p>
          </div>
          <div class="sow-payment-item">
            <p class="sow-payment-value">${capitalize(agreement.payment.type)}</p>
            <p class="sow-payment-label">Type</p>
          </div>
          <div class="sow-payment-item">
            <p class="sow-payment-value" style="font-size:0.8rem;">${escHtml(agreement.payment.schedule)}</p>
            <p class="sow-payment-label">Schedule</p>
          </div>
        </div>
      </div>

      <div class="sow-section">
        <p class="sow-section-label">Timeline</p>
        <div class="sow-timeline-bar">
          <div class="timeline-date">
            <div class="timeline-date-label">Start</div>
            <div class="timeline-date-value">${agreement.timeline.start}</div>
          </div>
          <div class="timeline-connector"></div>
          <div class="timeline-date" style="text-align:center;">
            <div class="timeline-date-label">Duration</div>
            <div class="timeline-date-value">${escHtml(agreement.timeline.duration)}</div>
          </div>
          <div class="timeline-connector"></div>
          <div class="timeline-date" style="text-align:right;">
            <div class="timeline-date-label">End</div>
            <div class="timeline-date-value">${agreement.timeline.end}</div>
          </div>
        </div>
      </div>

      <div class="sow-section">
        <p class="sow-section-label">Terms</p>
        <ul class="sow-list">
          ${(agreement.terms || []).map(t => `<li>${escHtml(t)}</li>`).join('')}
        </ul>
      </div>

      ${agreement.status === 'active' || agreement.status === 'complete' ? `
        <div class="sow-signatures">
          <div class="signature-block">
            <p class="signature-name">${escHtml(agreement.signedBy?.client || agreement.parties.client.name)}</p>
            <p class="signature-role">Client · ${escHtml(agreement.parties.client.name)}</p>
            ${agreement.signedAt ? `<p class="signature-date">Signed ${formatDate(agreement.signedAt)}</p>` : ''}
          </div>
          <div class="signature-block">
            <p class="signature-name">${escHtml(agreement.signedBy?.provider || agreement.parties.provider.name)}</p>
            <p class="signature-role">Provider · ${escHtml(agreement.parties.provider.name)}</p>
            ${agreement.signedAt ? `<p class="signature-date">Signed ${formatDate(agreement.signedAt)}</p>` : ''}
          </div>
        </div>
      ` : ''}

      <div class="sow-actions">
        ${agreement.status === 'draft' ? `
          <button class="btn btn-danger btn-sm" onclick="deleteAgreement('${agreement.id}')">Delete Draft</button>
          <button class="btn btn-success" onclick="signAgreement('${agreement.id}')">✓ Accept & Sign</button>
        ` : agreement.status === 'active' ? `
          <button class="btn btn-secondary btn-sm" onclick="markComplete('${agreement.id}')">Mark Complete</button>
        ` : `
          <span style="color:var(--am-success);font-weight:600;font-size:0.875rem;">✓ Completed</span>
        `}
      </div>
    </div>
  `;
}

// ── Event listener attachment ────────────────────────────────────────
function attachEventListeners(view) {
  if (view === 'agents') {
    const btnReg = document.getElementById('btn-register-agent');
    const btnRegEmpty = document.getElementById('btn-register-agent-empty');
    const btnClose = document.getElementById('btn-close-modal');
    const btnCancel = document.getElementById('btn-cancel-modal');
    const form = document.getElementById('agent-form');
    const modal = document.getElementById('agent-modal');

    if (btnReg) btnReg.addEventListener('click', () => openAgentModal());
    if (btnRegEmpty) btnRegEmpty.addEventListener('click', () => openAgentModal());
    if (btnClose) btnClose.addEventListener('click', closeAgentModal);
    if (btnCancel) btnCancel.addEventListener('click', closeAgentModal);
    if (modal) modal.addEventListener('click', (e) => { if (e.target === modal) closeAgentModal(); });
    if (form) form.addEventListener('submit', handleAgentSubmit);

    initSkillsInput('skills-container-agent', 'skills-input-agent');
  }

  if (view === 'brief') {
    const form = document.getElementById('brief-form');
    if (form) form.addEventListener('submit', handleBriefSubmit);
    initSkillsInput('skills-container-brief', 'skills-input-brief');
  }
}

// ── Agent modal ──────────────────────────────────────────────────────
function openAgentModal(agentId = null) {
  editingAgentId = agentId;
  // Re-render modal portion
  const modal = document.getElementById('agent-modal');
  modal.outerHTML = renderAgentModal();
  // Re-attach
  document.getElementById('btn-close-modal')?.addEventListener('click', closeAgentModal);
  document.getElementById('btn-cancel-modal')?.addEventListener('click', closeAgentModal);
  document.getElementById('agent-modal')?.addEventListener('click', (e) => { if (e.target.id === 'agent-modal') closeAgentModal(); });
  document.getElementById('agent-form')?.addEventListener('submit', handleAgentSubmit);
  initSkillsInput('skills-container-agent', 'skills-input-agent');
  document.getElementById('agent-modal').classList.add('open');
}

function closeAgentModal() {
  editingAgentId = null;
  const modal = document.getElementById('agent-modal');
  if (modal) modal.classList.remove('open');
}

function handleAgentSubmit(e) {
  e.preventDefault();
  const name = document.getElementById('f-name').value.trim();
  const handle = document.getElementById('f-handle').value.trim();
  const website = document.getElementById('f-website').value.trim();
  const description = document.getElementById('f-description').value.trim();
  const rate = parseFloat(document.getElementById('f-rate').value);
  const availability = document.getElementById('f-availability').value.trim();
  const skills = getSkillsFromContainer('skills-container-agent');

  if (!name || !description || !rate) return;

  const initials = name.split(' ').map(w => w[0]).join('').slice(0, 2).toUpperCase();
  const colors = ['#6366f1', '#0ea5e9', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899'];
  const color = colors[Math.floor(Math.random() * colors.length)];

  const agents = db.agents();

  if (editingAgentId) {
    const idx = agents.findIndex(a => a.id === editingAgentId);
    if (idx >= 0) {
      agents[idx] = { ...agents[idx], name, handle, website, description, skills, rate, availability };
      db.saveAgents(agents);
      showToast('Agent updated successfully', 'success');
    }
  } else {
    const newAgent = {
      id: genId('agent'),
      name, handle, website, description, skills, rate,
      rateUnit: 'hr',
      availability: availability || 'Available',
      avatar: initials,
      avatarColor: color,
      joined: new Date().toISOString().slice(0, 10),
      completedProjects: 0,
      rating: 5.0,
      reviews: 0,
    };
    agents.push(newAgent);
    db.saveAgents(agents);
    showToast(`${name} registered on the marketplace!`, 'success');
  }

  closeAgentModal();
  renderView('agents');
}

// ── Brief submit ─────────────────────────────────────────────────────
function handleBriefSubmit(e) {
  e.preventDefault();
  const clientName = document.getElementById('b-client').value.trim();
  const clientEmail = document.getElementById('b-email').value.trim();
  const project = document.getElementById('b-project').value.trim();
  const description = document.getElementById('b-description').value.trim();
  const budget = parseFloat(document.getElementById('b-budget').value);
  const budgetType = document.getElementById('b-budget-type').value;
  const timeline = document.getElementById('b-timeline').value;
  const requiredSkills = getSkillsFromContainer('skills-container-brief');

  if (!clientName || !project || !description || !budget) return;

  const brief = {
    id: genId('brief'),
    clientName, clientEmail, project, description,
    budget, budgetType, timeline, requiredSkills,
    status: 'open',
    submittedAt: new Date().toISOString(),
  };

  const briefs = db.briefs();
  briefs.push(brief);
  db.saveBriefs(briefs);

  showToast('Brief posted! Checking for matches…', 'success');

  setTimeout(() => {
    const matches = matchAgents(brief);
    if (matches.length > 0) {
      showToast(`${matches.length} agent${matches.length > 1 ? 's' : ''} matched your brief!`, 'success');
    }
    navigate('matches');
  }, 600);
}

// ── SoW generation ────────────────────────────────────────────────────
function generateSoW(briefId, agentId) {
  const briefs = db.briefs();
  const agents = db.agents();
  const brief = briefs.find(b => b.id === briefId);
  const agent = agents.find(a => a.id === agentId);
  if (!brief || !agent) return;

  const today = new Date().toISOString().slice(0, 10);
  const endDate = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);

  const sow = {
    id: genId('sow'),
    briefId,
    agentId,
    title: `${brief.project} — Statement of Work`,
    parties: {
      client: { name: brief.clientName, role: 'Client' },
      provider: { name: agent.name, handle: agent.handle, role: 'Provider' },
    },
    project: brief.project,
    scope: [
      brief.description,
      ...brief.requiredSkills.map(s => `${s} implementation and delivery`),
    ],
    deliverables: [
      'Completed project meeting all stated requirements',
      'Source code / deliverable files',
      'Basic documentation',
    ],
    payment: {
      amount: brief.budget,
      currency: 'USD',
      type: brief.budgetType || 'fixed',
      schedule: 'Upon delivery and acceptance',
    },
    timeline: {
      start: today,
      end: endDate,
      duration: brief.timeline || '1 week',
    },
    terms: [
      'Provider will deliver work meeting all scope items within the stated timeline',
      'Client may request up to 2 rounds of revisions',
      'Payment due within 3 business days of acceptance',
      'Provider retains right to display this project in portfolio',
      'Either party may terminate with 48 hours written notice',
    ],
    status: 'draft',
    createdAt: new Date().toISOString(),
  };

  const agreements = db.agreements();
  agreements.push(sow);
  db.saveAgreements(agreements);

  // Update brief status
  const idx = briefs.findIndex(b => b.id === briefId);
  if (idx >= 0) {
    briefs[idx].status = 'matched';
    briefs[idx].matchedAgentId = agentId;
    db.saveBriefs(briefs);
  }

  showToast('Statement of Work generated!', 'success');
  expandedAgreementId = sow.id;
  navigate('agreements');
}

// ── Agreement actions ────────────────────────────────────────────────
function toggleAgreement(id) {
  expandedAgreementId = expandedAgreementId === id ? null : id;
  renderView('agreements');
}

function signAgreement(id) {
  const agreements = db.agreements();
  const idx = agreements.findIndex(a => a.id === id);
  if (idx < 0) return;
  agreements[idx].status = 'active';
  agreements[idx].signedAt = new Date().toISOString();
  agreements[idx].signedBy = {
    client: agreements[idx].parties.client.name,
    provider: agreements[idx].parties.provider.name,
  };
  db.saveAgreements(agreements);
  showToast('Agreement signed and activated!', 'success');
  expandedAgreementId = id;
  renderView('agreements');
}

function markComplete(id) {
  const agreements = db.agreements();
  const idx = agreements.findIndex(a => a.id === id);
  if (idx < 0) return;
  agreements[idx].status = 'complete';
  agreements[idx].completedAt = new Date().toISOString();
  db.saveAgreements(agreements);
  showToast('Project marked as complete!', 'success');
  renderView('agreements');
}

function deleteAgreement(id) {
  if (!confirm('Delete this draft agreement?')) return;
  const agreements = db.agreements().filter(a => a.id !== id);
  db.saveAgreements(agreements);
  if (expandedAgreementId === id) expandedAgreementId = null;
  showToast('Draft deleted', 'info');
  renderView('agreements');
}

// ── Utility ───────────────────────────────────────────────────────────
function escHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function capitalize(str) {
  if (!str) return '';
  return str.charAt(0).toUpperCase() + str.slice(1);
}

function formatDate(isoStr) {
  if (!isoStr) return '';
  try {
    return new Date(isoStr).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  } catch { return isoStr; }
}

// ── Init ──────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  initializeSeedData();
  navigate('agents');
});
