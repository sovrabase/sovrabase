import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const mobileDir = path.join(__dirname, '..', 'src', 'project', 'mobile');

// utils.tsx
fs.writeFileSync(path.join(mobileDir, 'utils.tsx'), `export function renderFormattedJson(obj: unknown) {
  const jsonStr = JSON.stringify(obj, null, 2);
  return (
    <pre className="font-code-xs text-code-xs leading-relaxed overflow-x-auto p-3 rounded-lg bg-surface-container-lowest/70 border border-outline-variant/30 text-left">
      <code>
        {jsonStr.split('\\n').map((line, index) => {
          if (line.includes('":')) {
            const parts = line.split('":');
            const key = parts[0];
            const val = parts.slice(1).join('":');
            return (
              <div key={index}>
                <span className="text-[#a5b4fc]">{key}"</span>:
                <span className="text-primary">{val}</span>
              </div>
            );
          }
          return <div key={index} className="text-on-surface-variant/80">{line}</div>;
        })}
      </code>
    </pre>
  );
}
`);
try { fs.unlinkSync(path.join(mobileDir, 'utils.ts')); } catch { /* */ }

// hook - remove inline renderFormattedJson
const hookPath = path.join(mobileDir, 'hooks', 'useMobileDashboardData.ts');
let hook = fs.readFileSync(hookPath, 'utf8');
hook = hook.replace(
  /\n  \/\/ JSON Syntax highlighting helper[\s\S]*?  \};\n\n  \/\/ --- Filtering ---/,
  '\n  // --- Filtering ---'
);
hook = hook.replace("from '../utils'", "from '../utils.tsx'");
// ts doesn't need .tsx in import - use '../utils'
hook = hook.replace("from '../utils.tsx'", "from '../utils'");
fs.writeFileSync(hookPath, hook);

// Fix more pages - strip activeSubPage wrappers
const moreDir = path.join(mobileDir, 'more');
for (const file of fs.readdirSync(moreDir)) {
  if (!file.endsWith('.tsx') || file === 'MobileMoreSection.tsx') continue;
  const p = path.join(moreDir, file);
  let content = fs.readFileSync(p, 'utf8');
  content = content.replace(
    /return \(\n\s+\{activeSubPage === '[^']+' && \(\n/,
    'return (\n'
  );
  // Remove trailing )} before closing );
  content = content.replace(/\n\s+\)\}\n\s+\);/g, '\n  );');
  fs.writeFileSync(p, content);
}

console.log('Fixed utils, hook, and more pages');
