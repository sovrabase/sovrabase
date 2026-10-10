import { useEffect, useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../../api';
import { useAuth } from '../../store';
import { Mark } from '../../shell/Mark';
import { Banner, Button, Field, Input } from '../../ui';

type Health = 'unknown' | 'ok' | 'down';

export default function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [health, setHealth] = useState<Health>('unknown');

  useEffect(() => {
    let alive = true;
    api<{ status?: string }>('/health')
      .then((h) => alive && setHealth(h.status === 'ok' ? 'ok' : 'down'))
      .catch(() => alive && setHealth('down'));
    return () => {
      alive = false;
    };
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(email.trim(), password);
      navigate('/projects');
    } catch (err) {
      setError((err as Error).message || 'Sign-in failed. Check your email and password.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="min-h-screen bg-bg flex flex-col items-center justify-center px-4 py-10">
      <div className="w-full max-w-[400px] flex flex-col gap-6">
        <div className="flex items-center gap-2.5 justify-center">
          <Mark size={30} />
          <span className="text-[17px] font-semibold text-ink tracking-[-0.01em]">Sovrabase</span>
        </div>

        <div className="bg-surface border border-line rounded-lg shadow-pop p-6 sm:p-7">
          <h1 className="m-0 text-[22px] leading-7 font-semibold text-ink tracking-[-0.01em]">Sign in</h1>
          <p className="m-0 mt-1 mb-6 text-sm text-ink-3">Sign in to the console of this server.</p>

          <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
            <Field label="Email">
              <Input
                type="email"
                autoComplete="username"
                autoFocus
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
              />
            </Field>
            <Field label="Password">
              <Input
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </Field>

            {error && <Banner tone="danger">{error}</Banner>}

            <Button type="submit" variant="primary" loading={loading} disabled={!email.trim() || !password} className="w-full mt-1">
              Sign in
            </Button>
          </form>
        </div>

        <div className="flex flex-col items-center gap-1.5 text-[12.5px] text-ink-3">
          <span className="inline-flex items-center gap-1.5" role="status">
            <span
              className={
                'w-[7px] h-[7px] rounded-full ' + (health === 'ok' ? 'bg-ok' : health === 'down' ? 'bg-danger' : 'bg-line-2')
              }
              aria-hidden
            />
            {health === 'ok' ? 'All services running' : health === 'down' ? 'Server not reachable' : 'Checking server'}
          </span>
          <span>Your data, your server, your jurisdiction.</span>
        </div>
      </div>
    </main>
  );
}
