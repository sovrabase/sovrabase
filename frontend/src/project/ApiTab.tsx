import { useState } from 'react';
import { Copy, Terminal, Code } from 'lucide-react';
import { useToast } from '../components/Toast';
import { TabBar } from '../components/TabBar';

interface Props {
  projectId: string;
  apiKey?: string;
}

type SnippetLang = 'curl' | 'dart' | 'javascript';

const TAB_ICONS: Record<SnippetLang, React.ReactNode> = {
  curl: <Terminal className="w-4 h-4" />,
  dart: <Code className="w-4 h-4" />,
  javascript: <Code className="w-4 h-4" />,
};

const TAB_LABELS: Record<SnippetLang, string> = {
  curl: 'cURL',
  dart: 'Dart / Flutter',
  javascript: 'JavaScript',
};

function buildSnippets(apiKey: string, projectId: string, origin: string): Record<SnippetLang, string> {
  return {
    curl: [
      `curl -H "X-Project-Key: ${apiKey}" \\`,
      `  -H "Authorization: Bearer USER_TOKEN" \\`,
      `  ${origin}/api/v1`,
    ].join('\n'),
    dart: [
      `final client = SovrabaseClient(`,
      `  apiKey: '${apiKey}',`,
      `  projectId: '${projectId}',`,
      `);`,
    ].join('\n'),
    javascript: [
      `const sovrabase = new SovrabaseClient({`,
      `  apiKey: '${apiKey}',`,
      `  projectId: '${projectId}',`,
      `});`,
    ].join('\n'),
  };
}

const TABS = [
  { id: 'curl', label: 'cURL', icon: <Terminal className="w-4 h-4" /> },
  { id: 'dart', label: 'Dart / Flutter', icon: <Code className="w-4 h-4" /> },
  { id: 'javascript', label: 'JavaScript', icon: <Code className="w-4 h-4" /> },
];

export default function ApiTab({ projectId, apiKey }: Props) {
  const { showToast } = useToast();
  const [activeTab, setActiveTab] = useState<SnippetLang>('curl');
  const [copied, setCopied] = useState(false);

  const origin = typeof window !== 'undefined' ? window.location.origin : '';
  const snippets = buildSnippets(apiKey || 'YOUR_API_KEY', projectId, origin);
  const currentSnippet = snippets[activeTab];

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(currentSnippet);
      setCopied(true);
      showToast('Code snippet copied', 'success');
      setTimeout(() => setCopied(false), 2000);
    } catch {
      showToast('Failed to copy', 'error');
    }
  };

  if (!apiKey) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3 text-text-muted">
        <Terminal className="w-10 h-10 text-text-muted/40" />
        <p className="text-text-secondary">No API key generated for this project.</p>
        <p className="text-text-muted text-sm">Generate an API key from the Overview tab first.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Tab bar */}
      <TabBar tabs={TABS} activeTab={activeTab} onClick={(id) => setActiveTab(id as SnippetLang)} />

      {/* Code block */}
      <div className="bg-bg-card border border-border rounded-xl overflow-hidden">
        <div className="flex items-center justify-between px-4 py-2.5 bg-bg-input border-b border-border">
          <span className="flex items-center gap-2 text-text-secondary text-sm font-medium">
            {TAB_ICONS[activeTab]}
            {TAB_LABELS[activeTab]}
          </span>
          <button
            onClick={handleCopy}
            className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
              copied
                ? 'bg-success/10 text-success'
                : 'text-text-muted hover:text-text-primary hover:bg-bg-card'
            }`}
          >
            <Copy className="w-3.5 h-3.5" />
            {copied ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre className="p-5 overflow-x-auto text-sm font-mono text-text-primary leading-relaxed whitespace-pre-wrap bg-bg-input/30">
          {currentSnippet}
        </pre>
      </div>

      {/* Quick info */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <p className="text-text-muted text-xs uppercase tracking-wider mb-1">Endpoint</p>
          <code className="text-sm font-mono text-text-primary break-all">{origin}/api/v1</code>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <p className="text-text-muted text-xs uppercase tracking-wider mb-1">Auth Header</p>
          <code className="text-sm font-mono text-text-primary break-all">X-Project-Key: {apiKey}</code>
        </div>
      </div>
    </div>
  );
}
