import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Flag } from 'lucide-react';
import { useAuth } from '../store';

export function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(email, password);
      navigate('/dashboard');
    } catch (err) {
      setError((err as Error).message || 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="min-h-screen flex items-center justify-center p-4"
      style={{
        background:
          'radial-gradient(ellipse at 50% 0%, rgba(91,91,255,0.12) 0%, transparent 60%), radial-gradient(ellipse at 80% 80%, rgba(34,197,94,0.06) 0%, transparent 50%), var(--color-bg-app)',
      }}
    >
      <div className="glass border border-border rounded-2xl shadow-2xl p-8 w-full max-w-md animate-fade-slide">
        {/* Header */}
        <div className="flex flex-col items-center mb-8">
          <div className="mb-4 animate-[pulse_3s_ease-in-out_infinite]">
            <Flag className="w-12 h-12" style={{ color: '#003399' }} />
          </div>
          <h1 className="text-gradient text-3xl font-bold mb-1">Sovrabase</h1>
          <p className="text-text-muted text-sm">Sign in to sovereign control plane</p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div>
            <label htmlFor="email" className="block text-sm font-medium text-text-secondary mb-1.5">
              Email
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
              placeholder="admin@sovrabase.local"
              className="w-full px-4 py-2.5 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted text-sm focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent transition-colors"
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-sm font-medium text-text-secondary mb-1.5">
              Password
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              placeholder="••••••••"
              className="w-full px-4 py-2.5 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted text-sm focus:outline-none focus:ring-2 focus:ring-accent/40 focus:border-accent transition-colors"
            />
          </div>

          {error && (
            <div className="bg-danger/10 border border-danger/30 rounded-lg px-4 py-2.5 text-sm text-danger">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="mt-2 w-full py-2.5 rounded-lg bg-accent hover:bg-accent-hover text-white font-semibold text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Signing in…' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  );
}
