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
      className="min-h-screen flex items-center justify-center p-4 bg-background"
      style={{
        background:
          'radial-gradient(ellipse at 50% 0%, rgba(78,222,163,0.15) 0%, transparent 60%), radial-gradient(ellipse at 80% 80%, rgba(87,27,193,0.08) 0%, transparent 50%), #12131a',
      }}
    >
      <div className="glass border border-outline-variant rounded-2xl shadow-2xl p-8 w-full max-w-md animate-fade-slide relative overflow-hidden bento-glow">
        {/* Header */}
        <div className="flex flex-col items-center mb-8">
          <div className="mb-4 animate-bounce p-3 bg-primary/10 rounded-2xl border border-primary/20 shadow-[0_0_15px_rgba(78,222,163,0.15)]">
            <Flag className="w-10 h-10 text-primary" />
          </div>
          <h1 className="text-gradient text-3xl font-extrabold mb-1 font-headline-lg tracking-tight">Sovrabase</h1>
          <p className="text-text-secondary text-xs font-label-caps uppercase tracking-widest mt-1">Sovereign Developer Control Plane</p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div>
            <label htmlFor="email" className="block text-xs font-bold text-text-secondary uppercase tracking-widest mb-1.5 font-label-caps">
              Email
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
              placeholder="you@suprabase.io"
              className="w-full px-4 py-2.5 rounded-lg bg-bg-input border border-outline-variant/60 text-text-primary placeholder:text-text-muted/40 text-sm focus:outline-none focus:ring-1 focus:ring-primary/40 focus:border-primary transition-all font-sans"
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-xs font-bold text-text-secondary uppercase tracking-widest mb-1.5 font-label-caps">
              Password
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              placeholder="••••••••"
              className="w-full px-4 py-2.5 rounded-lg bg-bg-input border border-outline-variant/60 text-text-primary placeholder:text-text-muted/40 text-sm focus:outline-none focus:ring-1 focus:ring-primary/40 focus:border-primary transition-all font-sans"
            />
          </div>

          {error && (
            <div className="bg-error-container/20 border border-error/30 rounded-lg px-4 py-2.5 text-xs text-error font-semibold font-mono">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="mt-2 w-full py-3 rounded-lg bg-primary hover:brightness-110 text-on-primary font-bold text-xs uppercase tracking-widest transition-all active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed font-label-caps"
          >
            {loading ? 'Signing in…' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  );
}
