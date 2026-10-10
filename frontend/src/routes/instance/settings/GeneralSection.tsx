import { useState } from 'react';
import { RotateCw } from 'lucide-react';
import { api, formatBytes } from '../../../api';
import { Button, ConfirmDialog, Field, SaveBar, Section, Segmented, useToast } from '../../../ui';
import { DefList, str, useConfigSection, type Config, type SectionProps } from './shared';

type Form = { env: string };
const toForm = (c: Config): Form => ({ env: str(c, 'env') || 'development' });
const toBody = (d: Form) => ({ env: d.env });

const INFO_LABELS: Record<string, string> = {
  version: 'Version',
  go_version: 'Go version',
  region: 'Region',
  uptime: 'Uptime',
  os: 'OS',
  arch: 'Architecture',
  storage_driver: 'File storage',
  memory_bytes: 'Memory',
  storage_bytes: 'Disk used',
  max_storage_bytes: 'Disk limit',
  projects: 'Projects',
};

function infoValue(v: string) {
  return <span className={v.includes('/') ? 'font-mono text-[12.5px]' : undefined}>{v}</span>;
}

export function GeneralSection({ config, onSaved, stats }: SectionProps & { stats: Record<string, unknown> | null }) {
  const { showToast } = useToast();
  const { draft, patch, dirty, reset, saving, save } = useConfigSection(config, toForm, toBody, onSaved);
  const [restartOpen, setRestartOpen] = useState(false);

  const restart = async () => {
    try {
      await api('/admin/restart', { method: 'POST' });
      showToast('Server restarting. The console reconnects in a few seconds.', 'info');
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const fmt = (k: string, v: unknown) => (k.endsWith('_bytes') && typeof v === 'number' ? formatBytes(v) : String(v));
  const info: Array<[string, string]> = [];
  for (const k of Object.keys(INFO_LABELS)) {
    const v = stats?.[k];
    if (v !== undefined && v !== null && v !== '' && (typeof v !== 'number' || k !== 'max_storage_bytes' || v > 0)) info.push([INFO_LABELS[k], fmt(k, v)]);
  }
  for (const k of ['listen_addr', 'data_dir', 'config_file', 'plugins_dir']) {
    const v = str(config, k);
    if (v) info.push([{ listen_addr: 'Listen address', data_dir: 'Data folder', config_file: 'Config file', plugins_dir: 'Plugins folder' }[k]!, v]);
  }

  return (
    <>
      <Section title="Environment" description="Production turns on stricter defaults for every project.">
        <Field label="Mode">
          <Segmented
            aria-label="Mode"
            value={draft?.env ?? 'development'}
            onChange={(v) => patch({ env: v })}
            options={[
              { value: 'development', label: 'Development' },
              { value: 'production', label: 'Production' },
            ]}
          />
        </Field>
      </Section>

      {info.length > 0 && (
        <Section title="This server" description="Read from the running process and config.yaml.">
          <DefList rows={info.map(([k, v]) => [k, infoValue(v)])} />
        </Section>
      )}

      <Section title="Restart" description="Some changes (TLS files, backup interval, replication) apply after a restart." last>
        <div>
          <Button icon={<RotateCw className="w-4 h-4" />} onClick={() => setRestartOpen(true)}>
            Restart server
          </Button>
        </div>
      </Section>

      <SaveBar dirty={dirty} saving={saving} onSave={save} onDiscard={reset} />

      <ConfirmDialog
        open={restartOpen}
        onClose={() => setRestartOpen(false)}
        onConfirm={restart}
        title="Restart the server?"
        confirmLabel="Restart"
      >
        <p className="m-0">Every project is unavailable for a few seconds while the server restarts. Unsaved changes on this page are lost.</p>
      </ConfirmDialog>
    </>
  );
}
