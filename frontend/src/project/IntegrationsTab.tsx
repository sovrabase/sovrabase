import { useState, useEffect, useCallback } from 'react';
import { Loader2, Plus, Trash2, Check, Settings2, Code2 } from 'lucide-react';
import { api } from '../api';
import Modal from '../components/Modal';
import { useToast } from '../components/Toast';

interface ConfigField { key: string; label: string; type: string; required: boolean; placeholder?: string; help_text?: string; }
interface IntegrationDef { id: string; name: string; description: string; category: string; icon: string; color: string; config_fields: ConfigField[]; }
interface ProjectIntegration { id: string; config: Record<string, unknown>; }

interface Props { projectId: string; }

const CATEGORY_LABELS: Record<string, string> = {
  payments: 'Payments', email: 'Email', sms: 'SMS', notifications: 'Notifications', search: 'Search', analytics: 'Analytics',
};

export default function IntegrationsTab({ projectId }: Props) {
  const { showToast } = useToast();
  const [catalog, setCatalog] = useState<IntegrationDef[]>([]);
  const [enabled, setEnabled] = useState<ProjectIntegration[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<IntegrationDef | null>(null);
  const [editConfig, setEditConfig] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [showUsage, setShowUsage] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [cat, proj] = await Promise.all([
        api<{ integrations: IntegrationDef[] }>('/admin/integrations/catalog'),
        api<{ integrations: ProjectIntegration[] }>(`/admin/projects/${encodeURIComponent(projectId)}/integrations`),
      ]);
      setCatalog(cat.integrations || []);
      setEnabled(proj.integrations || []);
    } catch { /* */ }
    setLoading(false);
  }, [projectId]);

  useEffect(() => { load(); }, [load]);

  const isEnabled = (id: string) => enabled.some((e) => e.id === id);
  const getEnabled = (id: string) => enabled.find((e) => e.id === id);

  const openConfig = (def: IntegrationDef) => {
    setEditing(def);
    const existing = getEnabled(def.id);
    const cfg: Record<string, string> = {};
    if (existing) {
      for (const [k, v] of Object.entries(existing.config)) {
        cfg[k] = typeof v === 'boolean' ? String(v) : String(v ?? '');
      }
    }
    setEditConfig(cfg);
  };

  const saveIntegration = async () => {
    if (!editing) return;
    setSaving(true);
    try {
      // Build config object with proper types
      const config: Record<string, unknown> = {};
      for (const field of editing.config_fields) {
        const val = editConfig[field.key] ?? '';
        if (field.type === 'boolean') {
          config[field.key] = val === 'true' || val === 'true';
        } else if (field.type === 'number') {
          config[field.key] = val === '' ? 0 : Number(val);
        } else {
          config[field.key] = val;
        }
      }

      // Build new enabled list: replace or add
      let newList: ProjectIntegration[];
      const idx = enabled.findIndex((e) => e.id === editing.id);
      if (idx >= 0) {
        newList = [...enabled];
        newList[idx] = { id: editing.id, config };
      } else {
        newList = [...enabled, { id: editing.id, config }];
      }

      await api(`/admin/projects/${encodeURIComponent(projectId)}/integrations`, {
        method: 'PUT', body: JSON.stringify({ integrations: newList }),
      });
      showToast(`${editing.name} configured`, 'success');
      setEditing(null);
      await load();
    } catch (e) { showToast((e as Error).message || 'Failed', 'error'); }
    setSaving(false);
  };

  const removeIntegration = async (id: string) => {
    const def = catalog.find((c) => c.id === id);
    if (!confirm(`Disable ${def?.name || id}? Configuration will be lost.`)) return;
    try {
      const newList = enabled.filter((e) => e.id !== id);
      await api(`/admin/projects/${encodeURIComponent(projectId)}/integrations`, {
        method: 'PUT', body: JSON.stringify({ integrations: newList }),
      });
      showToast(`${def?.name || id} disabled`, 'success');
      await load();
    } catch (e) { showToast((e as Error).message || 'Failed', 'error'); }
  };

  // Group catalog by category
  const categories = [...new Set(catalog.map((c) => c.category))].sort();

  // Build code snippet for a configured integration
  const snippetFor = (def: IntegrationDef): string => {
    if (def.id === 'paypal') {
      return `// 1. Read public config from Sovrabase
const res = await fetch('${window.location.origin}/api/v1/integrations', {
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token }
});
const { integrations } = await res.json();
const paypal = integrations.find(i => i.id === 'paypal');

// 2. Create an order (server-side, uses your secret)
const order = await fetch('${window.location.origin}/api/v1/integrations/paypal/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: 'create_order', data: { amount: '10.00', currency: 'USD' } })
}).then(r => r.json());

// 3. Capture after approval
await fetch('${window.location.origin}/api/v1/integrations/paypal/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: 'capture_order', data: { order_id: order.id } })
});

// Webhook URL for PayPal dashboard:
//   ${window.location.origin}/api/v1/integrations/paypal/webhook?project_key=PROJECT_KEY`;
    }
    if (def.category === 'notifications') {
      return `// AUTOMATIC: Discord/Slack webhooks fire on every DB/API event.
// You do NOT need to call the action API manually. As soon as this
// integration is enabled, the server sends a message on:
//   - record:create (new document inserted)
//   - record:update (document modified)
//   - record:delete (document removed)
//   - auth:signup (new user registered)
//   - auth:signin (user logged in)
//
// Manual send (optional — for custom messages):
await fetch('${window.location.origin}/api/v1/integrations/${def.id}/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: 'send_message', data: { content: 'Custom message!' } })
});
`;
    }
    if (def.id === 'sendgrid' || def.id === 'twilio') {
      return `// Server-side send
await fetch('${window.location.origin}/api/v1/integrations/${def.id}/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: '${def.id === 'sendgrid' ? 'send_email' : 'send_sms'}', data: { ... } })
});
`;
    }
    return `// Server-side action
await fetch('${window.location.origin}/api/v1/integrations/${def.id}/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: '...', data: { ... } })
});

// Webhook URL for ${def.name} dashboard:
//   ${window.location.origin}/api/v1/integrations/${def.id}/webhook?project_key=PROJECT_KEY`;
  };

  if (loading) return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading integrations...</div>;

  return (
    <div className="space-y-6">
      {/* Enabled summary */}
      {enabled.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {enabled.map((e) => {
            const def = catalog.find((c) => c.id === e.id);
            if (!def) return null;
            return (
              <span key={e.id} className="inline-flex items-center gap-1.5 px-2.5 py-1 bg-success/10 text-success rounded-full text-xs font-medium">
                <span className="w-4 h-4 rounded-full flex items-center justify-center text-[8px] font-bold text-white" style={{ backgroundColor: def.color }}>{def.icon}</span>
                {def.name}
                <Check className="w-3 h-3" />
              </span>
            );
          })}
        </div>
      )}

      {/* Catalog by category */}
      {categories.map((cat) => (
        <div key={cat}>
          <h3 className="text-text-muted text-xs uppercase tracking-wider mb-3 font-medium">{CATEGORY_LABELS[cat] || cat}</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {catalog.filter((c) => c.category === cat).map((def) => {
              const on = isEnabled(def.id);
              return (
                <div key={def.id} className={`bg-bg-card border rounded-lg p-4 flex flex-col gap-3 ${on ? 'border-accent/40' : 'border-border'}`}>
                  <div className="flex items-start gap-3">
                    <span className="w-10 h-10 rounded-lg flex items-center justify-center text-base font-bold text-white shrink-0" style={{ backgroundColor: def.color }}>{def.icon}</span>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <h4 className="text-text-primary text-sm font-semibold truncate">{def.name}</h4>
                        {on && <span className="w-2 h-2 rounded-full bg-success shrink-0" />}
                      </div>
                      <p className="text-text-muted text-xs mt-0.5 line-clamp-2">{def.description}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 mt-auto">
                    <button onClick={() => openConfig(def)} className={`flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${on ? 'border border-border text-text-secondary hover:text-text-primary' : 'bg-accent text-white hover:opacity-90'}`}>
                      {on ? <><Settings2 className="w-3.5 h-3.5" /> Configure</> : <><Plus className="w-3.5 h-3.5" /> Enable</>}
                    </button>
                    {on && (
                      <>
                        <button onClick={() => setShowUsage((s) => s === def.id ? null : def.id)} className={`flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors border border-border ${showUsage === def.id ? 'text-accent bg-accent/10' : 'text-text-secondary hover:text-text-primary'}`}>
                          <Code2 className="w-3.5 h-3.5" /> Usage
                        </button>
                        <button onClick={() => removeIntegration(def.id)} className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs text-text-muted hover:text-danger hover:bg-danger/10 transition-colors">
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </>
                    )}
                  </div>
                  {on && showUsage === def.id && (
                    <pre className="bg-bg-input border border-border rounded-lg p-3 text-[11px] font-mono text-text-secondary overflow-auto max-h-64 whitespace-pre-wrap">{snippetFor(def)}</pre>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      ))}

      {/* Config Modal */}
      <Modal isOpen={!!editing} onClose={() => setEditing(null)} title={editing ? `Configure ${editing.name}` : ''} size="md">
        {editing && (
          <div className="space-y-4">
            <p className="text-text-muted text-sm">{editing.description}</p>
            {editing.config_fields.map((field) => (
              <div key={field.key}>
                <label className="block text-text-secondary text-sm mb-1">
                  {field.label}{field.required && <span className="text-danger ml-1">*</span>}
                </label>
                {field.type === 'boolean' ? (
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" checked={editConfig[field.key] === 'true'} onChange={(e) => setEditConfig((c) => ({ ...c, [field.key]: String(e.target.checked) }))} className="w-4 h-4 rounded border-border bg-bg-input accent-accent" />
                    <span className="text-text-muted text-xs">Enabled</span>
                  </label>
                ) : (
                  <input
                    type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'}
                    value={editConfig[field.key] ?? ''}
                    onChange={(e) => setEditConfig((c) => ({ ...c, [field.key]: e.target.value }))}
                    placeholder={field.placeholder}
                    className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono"
                  />
                )}
                {field.help_text && <p className="text-text-muted text-xs mt-1">{field.help_text}</p>}
              </div>
            ))}
            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setEditing(null)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button>
              <button onClick={saveIntegration} disabled={saving} className="flex items-center gap-1 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">
                {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />} Save
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
