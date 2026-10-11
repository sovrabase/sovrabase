import { useCallback, useEffect, useState } from 'react';
import { Archive, Download, Plus, Trash2 } from 'lucide-react';
import { api, formatBytes } from '../../../api';
import type { Backup } from '../../../types';
import { Button, Card, ConfirmDialog, EmptyState, Field, IconButton, Input, Row, SaveBar, Section, Skeleton, useToast } from '../../../ui';
import { shortDateTime, str, useConfigSection, type Config, type SectionProps } from './shared';

type Form = { backup_interval: string };
const toForm = (c: Config): Form => ({ backup_interval: str(c, 'backup_interval') });
const toBody = (d: Form) => ({ backup_interval: d.backup_interval.trim() });
const DURATION = /^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+$/;
const validate = (d: Form) => (d.backup_interval.trim() && !DURATION.test(d.backup_interval.trim()) ? 'Backup interval must look like 1h or 24h.' : null);

/** Fetch with the admin token and save the zip, since a plain link can't send the Authorization header. */
async function downloadBackup(name: string) {
  const token = localStorage.getItem('sovrabase_admin_token');
  const res = await fetch(`/admin/backups/${encodeURIComponent(name)}/download`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw new Error(`Download failed (${res.status})`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${name}.zip`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function BackupsSection({ config, onSaved, canWrite }: SectionProps & { canWrite: boolean }) {
  const { showToast } = useToast();
  const { draft, patch, dirty, reset, saving, save } = useConfigSection(config, toForm, toBody, onSaved, validate);
  const [backups, setBackups] = useState<Backup[] | null>(null);
  const [creating, setCreating] = useState(false);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const d = await api<{ backups: Backup[] }>('/admin/backups');
      const list = d.backups || [];
      list.sort((a, b) => (b.modified ?? '').localeCompare(a.modified ?? ''));
      setBackups(list);
    } catch {
      setBackups([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const create = async () => {
    setCreating(true);
    try {
      await api('/admin/backups', { method: 'POST' });
      showToast('Backup created', 'success');
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setCreating(false);
    }
  };

  const download = async (name: string) => {
    setDownloading(name);
    try {
      await downloadBackup(name);
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setDownloading(null);
    }
  };

  const remove = async () => {
    if (!toDelete) return;
    try {
      await api(`/admin/backups/${encodeURIComponent(toDelete)}`, { method: 'DELETE' });
      showToast('Backup deleted', 'success');
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const durErr = draft && validate(draft) ? 'Use a Go duration such as 1h, 6h or 24h. 0s turns automatic backups off.' : undefined;

  return (
    <>
      <Section title="Automatic backups" description="A full copy of every project's data. Changes apply after a restart.">
        <Field label="Every" hint="For example 6h or 24h. Use 0s to turn automatic backups off." error={durErr}>
          <Input
            mono
            value={draft?.backup_interval ?? ''}
            onChange={(e) => patch({ backup_interval: e.target.value })}
            placeholder="24h"
            className="max-w-[200px]"
          />
        </Field>
      </Section>

      <Section title="Saved backups" description="Download one to keep it off this server, or to restore elsewhere." last>
        {canWrite && (
          <div>
            <Button icon={<Plus className="w-4 h-4" />} loading={creating} onClick={create}>
              Back up now
            </Button>
          </div>
        )}
        <Card padded={false}>
          {backups === null ? (
            <div className="flex flex-col gap-3 p-4">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          ) : backups.length === 0 ? (
            <EmptyState icon={<Archive className="w-5 h-5" />} title="No backups yet" description="Back up now, or set an interval above." className="py-10" />
          ) : (
            backups.map((b) => (
              <Row key={b.name}>
                <div className="flex flex-col min-w-0 flex-1">
                  <span className="font-mono text-[13px] text-ink truncate">{b.name}</span>
                  <span className="text-[12.5px] text-ink-3">
                    {shortDateTime(b.modified)}
                    {b.size != null ? ` · ${formatBytes(b.size)}` : ''}
                  </span>
                </div>
                <div className="flex items-center gap-1">
                  <Button size="sm" icon={<Download className="w-3.5 h-3.5" />} loading={downloading === b.name} onClick={() => download(b.name)}>
                    Download
                  </Button>
                  {canWrite && (
                    <IconButton label={`Delete ${b.name}`} size="sm" onClick={() => setToDelete(b.name)} className="hover:!text-danger">
                      <Trash2 className="w-4 h-4" />
                    </IconButton>
                  )}
                </div>
              </Row>
            ))
          )}
        </Card>
      </Section>

      <SaveBar dirty={dirty} saving={saving} onSave={save} onDiscard={reset} />

      <ConfirmDialog open={!!toDelete} onClose={() => setToDelete(null)} onConfirm={remove} title="Delete backup" confirmLabel="Delete backup" confirmText={toDelete ?? undefined}>
        <p className="m-0">This backup is removed from the server. It can't be undone.</p>
      </ConfirmDialog>
    </>
  );
}
