import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Eye, EyeOff, Trash2 } from 'lucide-react';
import { api, adminProjectPath } from '../../api';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import { useProjects } from '../../store';
import { Banner, Button, Card, CardHeader, ConfirmDialog, CopyButton, IconButton, Page, PageHeader, Row, Spinner, Tabs, useToast } from '../../ui';
import { ConnectSnippet } from './home/ConnectSnippet';
import { apiBase, mask, useProjectDetail } from './home/data';
import { GeneralSection } from './settings/GeneralSection';
import { TeamSection } from './settings/TeamSection';
import { VariablesSection } from './settings/VariablesSection';

type SectionId = 'general' | 'api-keys' | 'team' | 'variables' | 'danger';

const SECTIONS: Array<{ value: SectionId; label: string }> = [
  { value: 'general', label: 'General' },
  { value: 'api-keys', label: 'API keys' },
  { value: 'team', label: 'Team' },
  { value: 'variables', label: 'Variables' },
  { value: 'danger', label: 'Danger zone' },
];

export default function ProjectSettings() {
  const { project, projectId, reload } = useProject();
  const { section } = useParams<{ section?: string }>();
  const navigate = useNavigate();
  const current: SectionId = SECTIONS.some((s) => s.value === section) ? (section as SectionId) : 'general';
  const { detail, error, reload: reloadDetail } = useProjectDetail(projectId);

  return (
    <Page>
      <PageHeader title="Settings" description={`Name, keys, team, variables and deletion for ${project.name}.`} />
      <Tabs<SectionId>
        className="mb-6"
        tabs={SECTIONS}
        value={current}
        onChange={(v) => navigate(projectPath(projectId, v === 'general' ? 'settings' : `settings/${v}`), { replace: true })}
      />

      {error && current !== 'team' && current !== 'variables' && (
        <div className="mb-4">
          <Banner tone="danger" title="Project details could not be loaded">
            {error}
          </Banner>
        </div>
      )}

      {current === 'general' && (
        <GeneralSection
          projectId={projectId}
          detail={detail}
          onSaved={async () => {
            await Promise.all([reloadDetail(), reload()]);
          }}
        />
      )}
      {current === 'api-keys' && (detail ? <ApiKeys projectId={projectId} apiKey={detail.api_key ?? project.api_key} /> : !error && <Spinner label="Loading keys" />)}
      {current === 'team' && <TeamSection projectId={projectId} />}
      {current === 'variables' && <VariablesSection projectId={projectId} />}
      {current === 'danger' && <DangerZone projectId={projectId} name={project.name} />}
    </Page>
  );
}

/* ---------- API keys ---------- */

function ApiKeys({ projectId, apiKey }: { projectId: string; apiKey?: string }) {
  const [reveal, setReveal] = useState(false);
  const url = `${apiBase()}/api/v1`;
  return (
    <div className="flex flex-col gap-5">
      <Card padded={false}>
        <CardHeader title="Project key" description="Your apps send it in the X-Project-Key header. Treat it like a password: keep it out of public repositories." />
        <Row>
          <div className="flex-1 min-w-[200px]">
            <p className="m-0 text-[12.5px] text-ink-3 mb-0.5">Key</p>
            {apiKey ? (
              <code className="font-mono text-[13px] text-ink break-all">{reveal ? apiKey : mask(apiKey)}</code>
            ) : (
              <span className="text-sm text-ink-3">This project has no key.</span>
            )}
          </div>
          {apiKey && (
            <div className="flex items-center gap-1">
              <IconButton size="sm" label={reveal ? 'Hide key' : 'Show key'} onClick={() => setReveal((r) => !r)}>
                {reveal ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </IconButton>
              <CopyButton text={apiKey} />
            </div>
          )}
        </Row>
        <Row>
          <div className="flex-1 min-w-[200px]">
            <p className="m-0 text-[12.5px] text-ink-3 mb-0.5">API URL</p>
            <code className="font-mono text-[13px] text-ink break-all">{url}</code>
          </div>
          <CopyButton text={url} />
        </Row>
        <Row>
          <div className="flex-1 min-w-[200px]">
            <p className="m-0 text-[12.5px] text-ink-3 mb-0.5">Project id</p>
            <code className="font-mono text-[13px] text-ink break-all">{projectId}</code>
          </div>
          <CopyButton text={projectId} />
        </Row>
      </Card>

      <Card>
        <h3 className="m-0 mb-1 text-[15px] font-semibold text-ink">Use it in your app</h3>
        <p className="m-0 mb-4 text-[12.5px] text-ink-3">Signed-in requests also send the user's token as a Bearer header.</p>
        <ConnectSnippet apiKey={apiKey} projectId={projectId} />
      </Card>
    </div>
  );
}

/* ---------- Danger zone ---------- */

function DangerZone({ projectId, name }: { projectId: string; name: string }) {
  const navigate = useNavigate();
  const { showToast } = useToast();
  const loadProjects = useProjects((s) => s.loadProjects);
  const [open, setOpen] = useState(false);

  const del = async () => {
    try {
      await api(adminProjectPath(projectId, ''), { method: 'DELETE' });
      showToast(`${name} deleted`, 'success');
      navigate('/projects');
      loadProjects();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  return (
    <Card padded={false} className="border-danger/40">
      <CardHeader title="Danger zone" description="Actions here can't be undone." />
      <Row>
        <div className="flex-1 min-w-[220px]">
          <p className="m-0 text-sm font-medium text-ink">Delete {name}</p>
          <p className="m-0 text-[12.5px] text-ink-3">Removes every collection, user, file, function and setting of this project.</p>
        </div>
        <Button variant="danger" size="sm" icon={<Trash2 className="w-3.5 h-3.5" />} onClick={() => setOpen(true)}>
          Delete project…
        </Button>
      </Row>
      <ConfirmDialog open={open} onClose={() => setOpen(false)} onConfirm={del} title={`Delete ${name}?`} confirmLabel="Delete project" confirmText={name}>
        All data of this project is deleted for good: records, users, files, functions, automations and keys. Apps using it stop working at once.
      </ConfirmDialog>
    </Card>
  );
}
