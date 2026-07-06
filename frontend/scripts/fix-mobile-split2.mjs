import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const mobileDir = path.join(__dirname, '..', 'src', 'project', 'mobile');

function walk(dir, cb) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(p, cb);
    else if (ent.name.endsWith('.ts') || ent.name.endsWith('.tsx')) cb(p);
  }
}

walk(mobileDir, (p) => {
  let c = fs.readFileSync(p, 'utf8');
  c = c.replaceAll("from '../../api'", "from '../../../api'");
  c = c.replaceAll("from '../../types'", "from '../../../types'");
  c = c.replaceAll("from '../../store'", "from '../../../store'");
  c = c.replaceAll("from '../../components/", "from '../../../components/");
  fs.writeFileSync(p, c);
});

// context + hook: add projectName
const contextPath = path.join(mobileDir, 'context.tsx');
let ctx = fs.readFileSync(contextPath, 'utf8');
ctx = ctx.replace(
  'export function MobileDashboardProvider({ projectId, children }: { projectId: string; children: ReactNode }) {',
  'export function MobileDashboardProvider({ projectId, projectName, children }: { projectId: string; projectName: string; children: ReactNode }) {'
);
ctx = ctx.replace(
  'const value = useMobileDashboardData(projectId);',
  'const value = useMobileDashboardData(projectId, projectName);'
);
fs.writeFileSync(contextPath, ctx);

const hookPath = path.join(mobileDir, 'hooks', 'useMobileDashboardData.ts');
let hook = fs.readFileSync(hookPath, 'utf8');
hook = hook.replace(
  'export function useMobileDashboardData(projectId: string) {',
  'export function useMobileDashboardData(projectId: string, projectName: string) {'
);
hook = hook.replace(
  'renderFormattedJson, filteredUsers, filteredDocs, filteredLogs, projectId,',
  'renderFormattedJson, filteredUsers, filteredDocs, filteredLogs, projectId, projectName,'
);
fs.writeFileSync(hookPath, hook);

const dashPath = path.join(mobileDir, 'MobileDashboard.tsx');
let dash = fs.readFileSync(dashPath, 'utf8');
dash = dash.replace(
  '<MobileDashboardProvider projectId={props.projectId}>',
  '<MobileDashboardProvider projectId={props.projectId} projectName={props.projectName}>'
);
fs.writeFileSync(dashPath, dash);

const moreSection = path.join(mobileDir, 'more', 'MobileMoreSection.tsx');
let ms = fs.readFileSync(moreSection, 'utf8');
ms = ms.replace("from '../FunctionsTab'", "from '../../FunctionsTab'");
fs.writeFileSync(moreSection, ms);

// Fix MobileConfigPage broken structure
const configPath = path.join(mobileDir, 'more', 'MobileConfigPage.tsx');
let cfg = fs.readFileSync(configPath, 'utf8');
cfg = cfg.replace(/\n                  <\/div>\n                \)\}\n\n                \{\/\* CONFIG KEY-VALUE ENTRIES SUB-PAGE \*\/\}\n                \{activeSubPage === 'config' && \(/,
  '\n                  </div>\n\n                {/* CONFIG KEY-VALUE ENTRIES */}');
cfg = cfg.replace(/\n                \)\}\n  \);/, '\n  );');
cfg = cfg.replace('activeSubPage, ', '');
fs.writeFileSync(configPath, cfg);

// Remove unused activeSubPage from more pages destructure
for (const file of fs.readdirSync(path.join(mobileDir, 'more'))) {
  if (!file.endsWith('.tsx')) continue;
  const p = path.join(mobileDir, 'more', file);
  let c = fs.readFileSync(p, 'utf8');
  c = c.replace(/activeSubPage, /g, '');
  c = c.replace(/, activeSubPage/g, '');
  fs.writeFileSync(p, c);
}

// Remove unused api imports where not used - simpler: prefix with void for files that import api but script added it
// Fix MobileMoreGrid - remove api import if unused
for (const [file, needsApi] of [
  ['MobileMoreGrid.tsx', false],
  ['MobileQueuesPage.tsx', false],
  ['MobileApiPage.tsx', false],
  ['MobileAnalyticsPage.tsx', false],
]) {
  const p = path.join(mobileDir, 'more', file);
  if (!fs.existsSync(p)) continue;
  let c = fs.readFileSync(p, 'utf8');
  if (!needsApi) c = c.replace("import { api } from '../../../api';\n", '');
  fs.writeFileSync(p, c);
}

// Fix unused vars in tabs - remove docs from overview if unused, users from auth if unused, logs from logs tab
const overviewPath = path.join(mobileDir, 'tabs', 'MobileOverviewTab.tsx');
let ov = fs.readFileSync(overviewPath, 'utf8');
ov = ov.replace('collections, docs, users,', 'collections, users,');
fs.writeFileSync(overviewPath, ov);

const authPath = path.join(mobileDir, 'tabs', 'MobileAuthTab.tsx');
let auth = fs.readFileSync(authPath, 'utf8');
auth = auth.replace('authSubTab, setAuthSubTab, users, swipedUserId,', 'authSubTab, setAuthSubTab, swipedUserId,');
fs.writeFileSync(authPath, auth);

const logsPath = path.join(mobileDir, 'tabs', 'MobileLogsTab.tsx');
let logs = fs.readFileSync(logsPath, 'utf8');
logs = logs.replace('logs, setLogs, isStreaming,', 'setLogs, isStreaming,');
fs.writeFileSync(logsPath, logs);

// Modals - trim unused destructure
const modalsPath = path.join(mobileDir, 'modals', 'MobileModals.tsx');
let mod = fs.readFileSync(modalsPath, 'utf8');
mod = mod.replace(
  /const \{[\s\S]*?\} = useMobileDashboard\(\);/,
  `const {
    activeModal, modalData, setModalData, setActiveModal, selectedCol, selectedBucket,
    showToast, loadBuckets, loadFiles, loadMembers, loadWebhooks, loadCronJobs,
    loadDocuments, loadCollections, loadConfigEntries, projectId,
  } = useMobileDashboard();`
);
fs.writeFileSync(modalsPath, mod);

console.log('Import and context fixes applied');
