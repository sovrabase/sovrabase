import { useState } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { CodeBlock, CopyButton, IconButton, Segmented } from '../../../ui';
import { apiBase, mask } from './data';

type Lang = 'javascript' | 'dart' | 'curl';

function snippet(lang: Lang, key: string, projectId: string, origin: string) {
  switch (lang) {
    case 'javascript':
      return [`const sovrabase = new SovrabaseClient({`, `  baseUrl: '${origin}',`, `  projectKey: '${key}',`, `});`].join('\n');
    case 'dart':
      return [`final client = SovrabaseClient(`, `  apiKey: '${key}',`, `  projectId: '${projectId}',`, `);`].join('\n');
    case 'curl':
      return [`curl -H "X-Project-Key: ${key}" \\`, `  -H "Authorization: Bearer USER_TOKEN" \\`, `  ${origin}/api/v1`].join('\n');
  }
}

/** Code to paste into an app, in three flavours. The key is masked on screen but copied in full. */
export function ConnectSnippet({ apiKey, projectId }: { apiKey?: string; projectId: string }) {
  const [lang, setLang] = useState<Lang>('javascript');
  const [reveal, setReveal] = useState(false);
  const origin = apiBase();
  const key = apiKey || 'YOUR_API_KEY';
  const shown = snippet(lang, apiKey ? (reveal ? apiKey : mask(apiKey)) : key, projectId, origin);
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Segmented
          aria-label="Language"
          size="sm"
          value={lang}
          onChange={setLang}
          options={[
            { value: 'javascript', label: 'JavaScript' },
            { value: 'dart', label: 'Flutter' },
            { value: 'curl', label: 'cURL' },
          ]}
        />
        <div className="flex items-center gap-1">
          {apiKey && (
            <IconButton size="sm" label={reveal ? 'Hide key' : 'Show key'} onClick={() => setReveal((r) => !r)}>
              {reveal ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </IconButton>
          )}
          <CopyButton text={snippet(lang, key, projectId, origin)} />
        </div>
      </div>
      <CodeBlock>{shown}</CodeBlock>
    </div>
  );
}
