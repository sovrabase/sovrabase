import { Field, Input, SaveBar, SecretInput, Section, Select, Switch, TokenInput } from '../../../ui';
import { bool, isSet, str, toNum, useConfigSection, type Config, type SectionProps } from './shared';

type Form = {
  session_duration: string;
  jwt_secret: string;
  allow_origins: string[];
  rate_limit_per_minute: string;
  rate_limit_burst: string;
  captcha_enabled: boolean;
  captcha_provider: string;
  captcha_site_key: string;
  captcha_secret: string;
  cert_file: string;
  key_file: string;
};

const toForm = (c: Config): Form => ({
  session_duration: str(c, 'session_duration'),
  jwt_secret: '',
  allow_origins: str(c, 'allow_origins')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean),
  rate_limit_per_minute: str(c, 'rate_limit_per_minute'),
  rate_limit_burst: str(c, 'rate_limit_burst'),
  captcha_enabled: bool(c, 'captcha_enabled'),
  captcha_provider: str(c, 'captcha_provider'),
  captcha_site_key: str(c, 'captcha_site_key'),
  captcha_secret: '',
  cert_file: str(c, 'cert_file'),
  key_file: str(c, 'key_file'),
});

const toBody = (d: Form) => ({
  session_duration: d.session_duration.trim(),
  jwt_secret: d.jwt_secret,
  allow_origins: d.allow_origins.join(','),
  rate_limit_per_minute: toNum(d.rate_limit_per_minute),
  rate_limit_burst: toNum(d.rate_limit_burst),
  captcha_enabled: d.captcha_enabled,
  captcha_provider: d.captcha_provider,
  captcha_site_key: d.captcha_site_key.trim(),
  captcha_secret: d.captcha_secret,
  cert_file: d.cert_file.trim(),
  key_file: d.key_file.trim(),
});

const DURATION = /^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+$/;

const validate = (d: Form) => {
  if (d.session_duration.trim() && !DURATION.test(d.session_duration.trim())) return 'Session length must look like 24h or 168h.';
  if (d.captcha_enabled && !d.captcha_provider) return 'Pick a captcha provider or turn captcha off.';
  return null;
};

export function SecuritySection({ config, onSaved }: SectionProps) {
  const { draft, patch, dirty, reset, saving, save, saved } = useConfigSection(config, toForm, toBody, onSaved, validate);
  if (!draft) return null;
  const durErr = draft.session_duration.trim() && !DURATION.test(draft.session_duration.trim()) ? 'Use a Go duration such as 24h, 168h or 30m.' : undefined;
  const tlsChanged = draft.cert_file !== saved.cert_file || draft.key_file !== saved.key_file;

  return (
    <>
      <Section title="Sessions" description="Applies to console admins and to every project's users.">
        <Field label="Session length" hint="How long a sign-in lasts, e.g. 24h or 168h (7 days)." error={durErr}>
          <Input mono value={draft.session_duration} onChange={(e) => patch({ session_duration: e.target.value })} placeholder="24h" className="max-w-[200px]" />
        </Field>
        <Field label="JWT signing secret" hint="Replacing it signs everyone out.">
          <SecretInput key={`j${isSet(config, 'jwt_secret')}`} isSet={isSet(config, 'jwt_secret')} value={draft.jwt_secret} onChange={(v) => patch({ jwt_secret: v })} />
        </Field>
      </Section>

      <Section title="Who may call the API" description="Browser origins and request limits.">
        <Field label="Allowed origins" hint="Press Enter after each origin. Use * to allow any origin.">
          <TokenInput mono value={draft.allow_origins} onChange={(v) => patch({ allow_origins: v })} placeholder="https://app.example.com" />
        </Field>
        <div className="flex flex-wrap gap-4">
          <Field label="Rate limit" className="w-[200px]">
            <Input
              type="number"
              min={1}
              value={draft.rate_limit_per_minute}
              onChange={(e) => patch({ rate_limit_per_minute: e.target.value })}
              placeholder="100"
              suffix={<span className="text-[12.5px] text-ink-3 pr-1.5 whitespace-nowrap">req / min</span>}
            />
          </Field>
          <Field label="Burst" className="w-[200px]">
            <Input
              type="number"
              min={1}
              value={draft.rate_limit_burst}
              onChange={(e) => patch({ rate_limit_burst: e.target.value })}
              placeholder="20"
              suffix={<span className="text-[12.5px] text-ink-3 pr-1.5">requests</span>}
            />
          </Field>
        </div>
      </Section>

      <Section title="Captcha" description="Protect sign-up and sign-in with a challenge.">
        <Switch
          checked={draft.captcha_enabled}
          onChange={(v) => patch({ captcha_enabled: v })}
          label="Require a captcha"
          description="Applies to sign-up and sign-in on every project."
        />
        {draft.captcha_enabled && (
          <>
            <Field label="Provider">
              <Select
                value={draft.captcha_provider}
                onChange={(e) => patch({ captcha_provider: e.target.value })}
                placeholder="Choose a provider"
                options={[
                  { value: 'hcaptcha', label: 'hCaptcha' },
                  { value: 'turnstile', label: 'Cloudflare Turnstile' },
                ]}
              />
            </Field>
            <Field label="Site key">
              <Input mono value={draft.captcha_site_key} onChange={(e) => patch({ captcha_site_key: e.target.value })} />
            </Field>
            <Field label="Secret key">
              <SecretInput key={`c${isSet(config, 'captcha_secret')}`} isSet={isSet(config, 'captcha_secret')} value={draft.captcha_secret} onChange={(v) => patch({ captcha_secret: v })} />
            </Field>
          </>
        )}
      </Section>

      <Section title="TLS" description="Certificates for serving HTTPS directly. Leave empty when a proxy handles HTTPS." last>
        <Field label="Certificate file" optional hint={tlsChanged ? 'Restart the server to apply.' : undefined}>
          <Input mono value={draft.cert_file} onChange={(e) => patch({ cert_file: e.target.value })} placeholder="/etc/ssl/certs/server.pem" />
        </Field>
        <Field label="Key file" optional>
          <Input mono value={draft.key_file} onChange={(e) => patch({ key_file: e.target.value })} placeholder="/etc/ssl/private/server.key" />
        </Field>
      </Section>

      <SaveBar dirty={dirty} saving={saving} onSave={save} onDiscard={reset} message={tlsChanged ? 'Unsaved changes · TLS needs a restart' : undefined} />
    </>
  );
}
