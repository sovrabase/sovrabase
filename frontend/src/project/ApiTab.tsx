import { useState } from 'react';
import { Copy, Code, Globe, Key, Terminal } from 'lucide-react';
import { useToast } from '../components/Toast';

interface Props {
  projectId: string;
  apiKey?: string;
}

const BASE_URL = 'https://api.sovrabase.io';

const SNIPPETS: { lang: string; icon: React.ReactNode; code: string }[] = [
  {
    lang: 'cURL',
    icon: <Terminal className="w-4 h-4" />,
    code: `curl -X GET "${BASE_URL}/v1/rest/records" \\
  -H "X-API-Key: YOUR_API_KEY"`,
  },
  {
    lang: 'JavaScript',
    icon: <Code className="w-4 h-4" />,
    code: `const sovra = new SovraClient({
  apiKey: 'YOUR_API_KEY',
  baseUrl: '${BASE_URL}',
});

const records = await sovra.get('my-collection');`,
  },
  {
    lang: 'Python',
    icon: <Code className="w-4 h-4" />,
    code: `from sovrabase import SovraClient

client = SovraClient(
    api_key='YOUR_API_KEY',
    base_url='${BASE_URL}'
)

records = client.get('my-collection')`,
  },
  {
    lang: 'Dart',
    icon: <Code className="w-4 h-4" />,
    code: `final sovra = SovraClient(
  apiKey: 'YOUR_API_KEY',
  baseUrl: '${BASE_URL}',
);

final records = await sovra.get('my-collection');`,
  },
  {
    lang: 'Go',
    icon: <Code className="w-4 h-4" />,
    code: `sovra := sovrabase.NewClient(
  sovrabase.WithAPIKey("YOUR_API_KEY"),
  sovrabase.WithBaseURL("${BASE_URL}"),
)

records, err := sovra.Get(context.Background(), "my-collection", nil)`,
  },
];

export default function ApiTab({ projectId, apiKey }: Props) {
  const { show } = useToast();
  const [copied, setCopied] = useState<string | null>(null);

  const copyText = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(label);
      show(`${label} copied`, 'success');
      setTimeout(() => setCopied(null), 2000);
    } catch {
      show('Failed to copy', 'error');
    }
  };

  const displayKey = apiKey || 'No API key generated. Generate one from Overview tab.';

  return (
    <div className="space-y-8">
      {/* Credentials */}
      <section>
        <h2 className="text-lg font-semibold text-text-primary mb-4 flex items-center gap-2">
          <Key className="w-5 h-5 text-accent" /> Project Credentials
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="bg-bg-card border border-border rounded-lg p-4">
            <p className="text-text-muted text-xs uppercase tracking-wider mb-1">API Key</p>
            <div className="flex items-center gap-2">
              <code className="text-sm font-mono text-text-primary break-all flex-1">{displayKey}</code>
              {apiKey && (
                <button onClick={() => copyText(apiKey, 'API Key')} className={`p-1.5 rounded transition-colors ${copied === 'API Key' ? 'text-success' : 'text-text-secondary hover:text-text-primary'}`} title="Copy">
                  <Copy className="w-4 h-4" />
                </button>
              )}
            </div>
          </div>
          <div className="bg-bg-card border border-border rounded-lg p-4">
            <p className="text-text-muted text-xs uppercase tracking-wider mb-1">Base URL</p>
            <div className="flex items-center gap-2">
              <div className="flex items-center gap-1 text-sm font-mono text-text-primary">
                <Globe className="w-4 h-4 text-text-muted" /> {BASE_URL}
              </div>
              <button onClick={() => copyText(BASE_URL, 'Base URL')} className={`p-1.5 rounded transition-colors ${copied === 'Base URL' ? 'text-success' : 'text-text-secondary hover:text-text-primary'}`} title="Copy">
                <Copy className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* SDK Install */}
      <section>
        <h2 className="text-lg font-semibold text-text-primary mb-3 flex items-center gap-2">
          <Terminal className="w-5 h-5 text-accent" /> SDK Installation
        </h2>
        <div className="bg-bg-card border border-border rounded-lg p-4 font-mono text-sm text-text-primary space-y-2">
          <p className="text-text-muted"># Install the SDK for your platform</p>
          <p><span className="text-accent">npm</span> install sovrabase-sdk</p>
          <p><span className="text-accent">pip</span> install sovrabase</p>
          <p><span className="text-accent">dart pub</span> add sovrabase</p>
          <p><span className="text-accent">go get</span> github.com/sovrabase/sovrabase-go</p>
        </div>
      </section>

      {/* Code Snippets */}
      <section>
        <h2 className="text-lg font-semibold text-text-primary mb-3">Quick Start</h2>
        <div className="space-y-3">
          {SNIPPETS.map((s) => (
            <div key={s.lang} className="bg-bg-card border border-border rounded-lg overflow-hidden">
              <div className="flex items-center justify-between px-4 py-2 bg-bg-input">
                <span className="flex items-center gap-2 text-text-secondary text-sm font-medium">
                  {s.icon} {s.lang}
                </span>
                <button onClick={() => copyText(s.code, s.lang)} className={`p-1 rounded transition-colors text-xs ${copied === s.lang ? 'text-success' : 'text-text-muted hover:text-text-primary'}`}>
                  {copied === s.lang ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <pre className="p-4 overflow-x-auto text-xs font-mono text-text-primary leading-relaxed">
                {s.code.replace('YOUR_API_KEY', apiKey || 'YOUR_API_KEY')}
              </pre>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
