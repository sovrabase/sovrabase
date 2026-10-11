import { Field, Input, SaveBar, SecretInput, Section, Segmented, Switch } from '../../../ui';
import { bool, isSet, str, useConfigSection, type Config, type SectionProps } from './shared';

type Form = {
  s3_enabled: boolean;
  s3_endpoint: string;
  s3_bucket_prefix: string;
  s3_access_key: string;
  s3_secret_key: string;
  s3_use_ssl: boolean;
};

const toForm = (c: Config): Form => ({
  s3_enabled: bool(c, 's3_enabled'),
  s3_endpoint: str(c, 's3_endpoint'),
  s3_bucket_prefix: str(c, 's3_bucket_prefix'),
  s3_access_key: str(c, 's3_access_key'),
  s3_secret_key: '',
  s3_use_ssl: bool(c, 's3_use_ssl'),
});

const toBody = (d: Form) => ({
  s3_enabled: d.s3_enabled,
  s3_endpoint: d.s3_endpoint.trim(),
  s3_bucket_prefix: d.s3_bucket_prefix.trim(),
  s3_access_key: d.s3_access_key.trim(),
  s3_secret_key: d.s3_secret_key,
  s3_use_ssl: d.s3_use_ssl,
});

export function StorageSection({ config, onSaved, stats }: SectionProps & { stats: Record<string, unknown> | null }) {
  const { draft, patch, dirty, reset, saving, save } = useConfigSection(config, toForm, toBody, onSaved);
  if (!draft) return null;
  const dataDir = str(config, 'data_dir');
  const driver = typeof stats?.storage_driver === 'string' ? stats.storage_driver : null;

  return (
    <>
      <Section
        title="Where files live"
        description={
          <>
            Uploads from every project's Storage.
            {driver && (
              <>
                {' '}
                Running now: <b className="font-medium text-ink-2">{driver}</b>.
              </>
            )}
          </>
        }
        last={!draft.s3_enabled}
      >
        <Field label="Location">
          <Segmented
            aria-label="Location"
            value={draft.s3_enabled ? 's3' : 'disk'}
            onChange={(v) => patch({ s3_enabled: v === 's3' })}
            options={[
              { value: 'disk', label: "This server's disk" },
              { value: 's3', label: 'S3-compatible bucket' },
            ]}
          />
        </Field>
        <p className="m-0 text-[13px] text-ink-3">
          {draft.s3_enabled ? (
            'For larger volumes or several nodes. Works with any S3 API, including EU-hosted providers.'
          ) : (
            <>
              Simplest. Files sit next to the database{dataDir && <> in <code className="font-mono text-[12.5px]">{dataDir}</code></>} and are included in
              backups.
            </>
          )}
        </p>
      </Section>

      {draft.s3_enabled && (
        <Section title="S3 bucket" description="Credentials need permission to create buckets and read and write objects." last>
          <Field label="Endpoint">
            <Input mono value={draft.s3_endpoint} onChange={(e) => patch({ s3_endpoint: e.target.value })} placeholder="https://s3.fr-par.scw.cloud" />
          </Field>
          <Field label="Bucket prefix" hint="Each project's bucket name starts with this.">
            <Input mono value={draft.s3_bucket_prefix} onChange={(e) => patch({ s3_bucket_prefix: e.target.value })} placeholder="sovrabase-" />
          </Field>
          <Field label="Access key">
            <Input mono value={draft.s3_access_key} onChange={(e) => patch({ s3_access_key: e.target.value })} autoComplete="off" />
          </Field>
          <Field label="Secret key">
            <SecretInput
              key={`s${isSet(config, 's3_secret_key')}`}
              isSet={isSet(config, 's3_secret_key')}
              value={draft.s3_secret_key}
              onChange={(v) => patch({ s3_secret_key: v })}
            />
          </Field>
          <Switch checked={draft.s3_use_ssl} onChange={(v) => patch({ s3_use_ssl: v })} label="Use HTTPS" description="Turn off only for a local test endpoint." />
        </Section>
      )}

      <SaveBar dirty={dirty} saving={saving} onSave={save} onDiscard={reset} />
    </>
  );
}
