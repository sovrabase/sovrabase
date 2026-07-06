import { useMobileDashboard } from '../context';

export default function MobileApiPage() {
  const {
    apiLanguage, setApiLanguage, projectId
  } = useMobileDashboard();
  return (
                  <div className="space-y-stack-md animate-fade-slide">
                    <div className="flex border-b border-outline-variant/30 gap-4 mb-2">
                      {(['js', 'dart', 'curl'] as const).map(lang => (
                        <button
                          key={lang}
                          onClick={() => setApiLanguage(lang)}
                          className={`pb-1.5 text-xs font-bold uppercase tracking-wider font-label-caps ${apiLanguage === lang ? 'text-primary border-b border-primary' : 'text-on-surface-variant'}`}
                        >
                          {lang === 'js' ? 'Javascript' : lang === 'dart' ? 'Dart / Flutter' : 'cURL'}
                        </button>
                      ))}
                    </div>
    
                    <div className="bg-surface-container-low border border-outline-variant rounded-xl p-3 select-all overflow-x-auto">
                      <pre className="font-code-xs text-[10px] leading-relaxed text-secondary font-mono">
                        {apiLanguage === 'js' && `// Initialize JavaScript SDK\nimport { createClient } from '@sovrabase/sdk';\n\nconst sovrabase = createClient({\n  url: '${window.location.origin}/api/v1',\n  project: '${projectId}',\n  key: 'CLIENT_API_KEY'\n});\n\n// Read Documents\nconst data = await sovrabase.from('posts').select('*');`}
                        {apiLanguage === 'dart' && `// Initialize Dart client\nimport 'package:sovrabase/sovrabase.dart';\n\nfinal sovrabase = SovrabaseClient(\n  url: '${window.location.origin}/api/v1',\n  project: '${projectId}',\n  key: 'CLIENT_API_KEY'\n);\n\n// Query Stream\nfinal list = await sovrabase.from('posts').select();`}
                        {apiLanguage === 'curl' && `# Fetch all rows from posts collection\ncurl -X GET \\\n  -H "Authorization: Bearer CLIENT_API_KEY" \\\n  "${window.location.origin}/api/v1/collections/posts/documents"`}
                      </pre>
                    </div>
                  </div>
  );
}
