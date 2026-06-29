import { useState, useEffect } from 'react';
import { Mail, RotateCcw, Edit3, Loader2, FileText } from 'lucide-react';
import { api } from '../api';
import type { EmailTemplate } from '../types';
import Modal from '../components/Modal';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useToast } from '../components/Toast';

interface Props {
  projectId: string;
  apiKey?: string;
}

const TEMPLATE_INFO: Record<string, { label: string; variables: string[] }> = {
  email_verification: { label: 'Email Verification', variables: ['{{.URL}}', '{{.Email}}', '{{.Token}}'] },
  password_reset: { label: 'Password Reset', variables: ['{{.URL}}', '{{.Email}}', '{{.Token}}'] },
  magic_link: { label: 'Magic Link', variables: ['{{.URL}}', '{{.Email}}', '{{.Token}}'] },
  welcome: { label: 'Welcome', variables: ['{{.Email}}'] },
  invitation: { label: 'Invitation', variables: ['{{.URL}}', '{{.Email}}', '{{.ProjectName}}'] },
};

export default function EmailTemplatesTab({ projectId }: Props) {
  const { showToast } = useToast();
  const [templates, setTemplates] = useState<EmailTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [editTarget, setEditTarget] = useState<EmailTemplate | null>(null);
  const [resetTarget, setResetTarget] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const loadTemplates = () => {
    setLoading(true);
    api<{ data: EmailTemplate[] }>(`/admin/projects/${encodeURIComponent(projectId)}/email-templates`)
      .then((res) => setTemplates(res.data || []))
      .finally(() => setLoading(false));
  };

  useEffect(() => { loadTemplates(); }, [projectId]);

  const handleEdit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!editTarget) return;
    const form = e.currentTarget;
    const subject = (form.elements.namedItem('subject') as HTMLInputElement).value.trim();
    const body = (form.elements.namedItem('body') as HTMLTextAreaElement).value.trim();
    if (!subject || !body) return;
    setSubmitting(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/email-templates`, {
        method: 'POST',
        body: JSON.stringify({ type: editTarget.type, subject, body }),
      });
      showToast('Template saved', 'success');
      setEditTarget(null);
      loadTemplates();
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleReset = async () => {
    if (!resetTarget) return;
    setSubmitting(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/email-templates/${resetTarget}/reset`, { method: 'POST' });
      showToast('Template reset to default', 'success');
      setResetTarget(null);
      loadTemplates();
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const isCustom = (t: EmailTemplate) => !!t.updated_at;

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading templates...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Email Templates ({templates.length})</h2>
      </div>

      {templates.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Mail className="w-10 h-10" />
          <p>No email templates found</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {templates.map((t) => {
            const info = TEMPLATE_INFO[t.type] || { label: t.type, variables: [] };
            const custom = isCustom(t);
            return (
              <div key={t.type} className="border border-border rounded-xl p-5 bg-bg-card hover:border-accent/30 transition-colors">
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <h3 className="text-sm font-semibold text-text-primary">{info.label}</h3>
                      {custom ? (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-accent/10 text-accent text-xs font-medium">
                          <FileText className="w-3 h-3" /> Custom
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-bg-input text-text-muted text-xs font-medium">Default</span>
                      )}
                    </div>
                    <p className="text-sm text-text-primary font-mono mt-1.5 truncate" title={t.subject}>{t.subject}</p>
                    {custom && t.updated_at && (
                      <p className="text-xs text-text-muted mt-1">Modified {new Date(t.updated_at).toLocaleDateString()}</p>
                    )}
                    <div className="flex items-center gap-1.5 mt-2.5 flex-wrap">
                      {info.variables.map((v) => (
                        <code key={v} className="px-1.5 py-0.5 rounded bg-bg-input border border-border text-xs text-text-muted font-mono">{v}</code>
                      ))}
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <button onClick={() => setEditTarget(t)} className="p-2 rounded-lg hover:bg-accent/10 text-text-secondary hover:text-accent transition-colors" title="Edit template">
                      <Edit3 className="w-4 h-4" />
                    </button>
                    {custom && (
                      <button onClick={() => setResetTarget(t.type)} className="p-2 rounded-lg hover:bg-warning/10 text-text-secondary hover:text-warning transition-colors" title="Reset to default">
                        <RotateCcw className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Edit Modal */}
      <Modal isOpen={!!editTarget} onClose={() => setEditTarget(null)} title={editTarget ? `Edit ${TEMPLATE_INFO[editTarget.type]?.label || editTarget.type}` : ''} size="lg">
        {editTarget && (
          <form onSubmit={handleEdit} className="space-y-4">
            <div>
              <label className="block text-sm text-text-secondary mb-1">Subject</label>
              <input name="subject" defaultValue={editTarget.subject} required className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent" />
            </div>
            <div>
              <label className="block text-sm text-text-secondary mb-1">Body</label>
              <textarea name="body" defaultValue={editTarget.body} required rows={12} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm font-mono placeholder:text-text-muted focus:outline-none focus:border-accent resize-y" />
            </div>
            <div className="bg-bg-input border border-border rounded-lg p-3">
              <p className="text-xs text-text-muted font-medium mb-1.5">Available variables:</p>
              <div className="flex flex-wrap gap-1.5">
                {(TEMPLATE_INFO[editTarget.type]?.variables || []).map((v) => (
                  <code key={v} className="px-1.5 py-0.5 rounded bg-bg-card border border-border text-xs text-text-muted font-mono">{v}</code>
                ))}
              </div>
            </div>
            <div className="flex justify-end gap-3 pt-2">
              <button type="button" onClick={() => setEditTarget(null)} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary transition-colors">Cancel</button>
              <button type="submit" disabled={submitting} className="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:bg-accent-hover transition-colors disabled:opacity-50">
                {submitting ? 'Saving...' : 'Save'}
              </button>
            </div>
          </form>
        )}
      </Modal>

      <ConfirmDialog
        isOpen={!!resetTarget}
        onCancel={() => setResetTarget(null)}
        onConfirm={handleReset}
        title="Reset Email Template"
        message={`Reset this template to its default? Any customizations will be lost.`}
      />
    </div>
  );
}
