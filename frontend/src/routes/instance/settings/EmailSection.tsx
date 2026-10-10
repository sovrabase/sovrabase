import { useState } from 'react';
import { Send } from 'lucide-react';
import { api } from '../../../api';
import { Button, Field, Input, SaveBar, SecretInput, Section, Select, Switch, useToast } from '../../../ui';
import { bool, isSet, str, toNum, useConfigSection, type Config, type SectionProps } from './shared';

type Form = {
  email_provider: string;
  email_api_key: string;
  email_api_secret: string;
  email_verification: boolean;
  smtp_sender: string;
  smtp_host: string;
  smtp_port: string;
  smtp_user: string;
  smtp_password: string;
};

const toForm = (c: Config): Form => ({
  email_provider: str(c, 'email_provider') || 'smtp',
  email_api_key: '',
  email_api_secret: '',
  email_verification: bool(c, 'email_verification'),
  smtp_sender: str(c, 'smtp_sender'),
  smtp_host: str(c, 'smtp_host'),
  smtp_port: str(c, 'smtp_port'),
  smtp_user: str(c, 'smtp_user'),
  smtp_password: '',
});

const toBody = (d: Form) => ({
  email_provider: d.email_provider,
  email_api_key: d.email_api_key,
  email_api_secret: d.email_api_secret,
  email_verification: d.email_verification,
  smtp_sender: d.smtp_sender.trim(),
  smtp_host: d.smtp_host.trim(),
  smtp_port: toNum(d.smtp_port),
  smtp_user: d.smtp_user.trim(),
  smtp_password: d.smtp_password,
});

const PROVIDERS = [
  { value: 'smtp', label: 'SMTP server', help: 'Any SMTP server: your own, Gmail, Mailgun and so on.' },
  { value: 'resend', label: 'Resend', help: 'Needs an API key from resend.com. The sender domain must be verified there.' },
  { value: 'mailtrap', label: 'Mailtrap', help: 'Needs an API key from mailtrap.io. Good for testing in sandbox mode.' },
  { value: 'brevo', label: 'Brevo', help: 'Needs an API key from brevo.com (formerly Sendinblue).' },
  { value: 'mailjet', label: 'Mailjet', help: 'Needs an API key and a secret key from mailjet.com.' },
  { value: 'maildev', label: 'Log only', help: 'Nothing is sent. Every email is written to the email log. No setup needed.' },
];

/** Fields that matter to the test for the chosen provider, compared with what is saved. */
function testOverrides(d: Form, saved: Form): Record<string, unknown> | null {
  const secretTyped = !!(d.email_api_key || d.email_api_secret || d.smtp_password);
  const changed =
    secretTyped ||
    (['email_provider', 'smtp_sender', 'smtp_host', 'smtp_port', 'smtp_user'] as const).some((k) => d[k] !== saved[k]);
  if (!changed) return null;
  const body: Record<string, unknown> = { email_provider: d.email_provider, smtp_sender: d.smtp_sender.trim() };
  if (d.email_provider === 'smtp') {
    body.smtp_host = d.smtp_host.trim();
    const port = toNum(d.smtp_port);
    if (port !== undefined) body.smtp_port = port;
    body.smtp_user = d.smtp_user.trim();
    if (d.smtp_password) body.smtp_password = d.smtp_password;
  } else {
    if (d.email_api_key) body.email_api_key = d.email_api_key;
    if (d.email_provider === 'mailjet' && d.email_api_secret) body.email_api_secret = d.email_api_secret;
  }
  return body;
}

export function EmailSection({ config, onSaved }: SectionProps) {
  const { showToast } = useToast();
  const { draft, patch, dirty, reset, saving, save, saved } = useConfigSection(config, toForm, toBody, onSaved);
  const [testTo, setTestTo] = useState(() => str(config, 'admin_email') || localStorage.getItem('sovrabase_admin_email') || '');
  const [testing, setTesting] = useState(false);
  if (!draft) return null;

  const provider = draft.email_provider;
  const isSmtp = provider === 'smtp';
  const usesKey = !isSmtp && provider !== 'maildev';
  const overrides = testOverrides(draft, saved);

  const sendTest = async () => {
    setTesting(true);
    try {
      const r = await api<{ to?: string }>('/admin/smtp/test', {
        method: 'POST',
        body: JSON.stringify({ to: testTo.trim(), ...(overrides ?? {}) }),
      });
      showToast(`Test email sent to ${r.to ?? testTo.trim()}`, 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setTesting(false);
    }
  };

  return (
    <>
      <Section title="Delivery" description="How the server sends verification, password reset and magic-link emails for every project.">
        <Field label="Provider" hint={PROVIDERS.find((p) => p.value === provider)?.help}>
          <Select value={provider} onChange={(e) => patch({ email_provider: e.target.value })} options={PROVIDERS} />
        </Field>
        <Field label="Sender address" hint="The From address. Its domain must be allowed by your provider.">
          <Input type="email" value={draft.smtp_sender} onChange={(e) => patch({ smtp_sender: e.target.value })} placeholder="no-reply@example.com" />
        </Field>

        {usesKey && (
          <Field label="API key">
            <SecretInput key={`k${isSet(config, 'email_api_key')}`} isSet={isSet(config, 'email_api_key')} value={draft.email_api_key} onChange={(v) => patch({ email_api_key: v })} />
          </Field>
        )}
        {provider === 'mailjet' && (
          <Field label="API secret">
            <SecretInput key={`s${isSet(config, 'email_api_secret')}`} isSet={isSet(config, 'email_api_secret')} value={draft.email_api_secret} onChange={(v) => patch({ email_api_secret: v })} />
          </Field>
        )}

        {isSmtp && (
          <>
            <div className="flex flex-wrap gap-4">
              <Field label="Host" className="flex-[3_1_220px]">
                <Input mono value={draft.smtp_host} onChange={(e) => patch({ smtp_host: e.target.value })} placeholder="smtp.example.com" />
              </Field>
              <Field label="Port" className="flex-[1_1_100px]">
                <Input type="number" min={1} max={65535} value={draft.smtp_port} onChange={(e) => patch({ smtp_port: e.target.value })} placeholder="587" />
              </Field>
            </div>
            <Field label="Username" optional>
              <Input value={draft.smtp_user} onChange={(e) => patch({ smtp_user: e.target.value })} autoComplete="off" />
            </Field>
            <Field label="Password" optional>
              <SecretInput key={`p${isSet(config, 'smtp_password')}`} isSet={isSet(config, 'smtp_password')} value={draft.smtp_password} onChange={(v) => patch({ smtp_password: v })} />
            </Field>
          </>
        )}
      </Section>

      <Section title="Verification" description="Ask new users to confirm their email address before they can sign in.">
        <Switch
          checked={draft.email_verification}
          onChange={(v) => patch({ email_verification: v })}
          label="Require email verification"
          description="Sends a confirmation link on sign-up. Templates live in each project's Auth > Emails."
        />
      </Section>

      <Section title="Send a test" description="Check the setup before your users depend on it." last>
        <div className="flex flex-wrap items-end gap-3">
          <Field label="Recipient" className="flex-[1_1_240px]">
            <Input type="email" value={testTo} onChange={(e) => setTestTo(e.target.value)} placeholder="you@example.com" />
          </Field>
          <Button icon={<Send className="w-4 h-4" />} loading={testing} disabled={!/\S+@\S+\.\S+/.test(testTo)} onClick={sendTest}>
            Send test email
          </Button>
        </div>
        <p className="m-0 text-[12.5px] text-ink-3">
          {overrides
            ? 'Uses the values above without saving them. Secrets you left as they are use the saved ones.'
            : 'Uses the saved settings. Edit a field above to try changes before saving.'}
        </p>
      </Section>

      <SaveBar dirty={dirty} saving={saving} onSave={save} onDiscard={reset} />
    </>
  );
}
