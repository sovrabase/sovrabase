import { BrowserRouter, Navigate, Outlet, Route, Routes } from 'react-router-dom';
import { hasToken } from './api';
import { AppShell } from './shell/AppShell';
import { ProjectProvider } from './shell/ProjectContext';

import Login from './routes/instance/Login';
import Projects from './routes/instance/Projects';
import PluginLibrary from './routes/instance/PluginLibrary';
import PluginDetail from './routes/instance/PluginDetail';
import Members from './routes/instance/Members';
import InstanceSettings from './routes/instance/InstanceSettings';
import Account from './routes/instance/Account';

import ProjectHome from './routes/project/Home';
import ProjectDatabase from './routes/project/Database';
import ProjectStorage from './routes/project/Storage';
import ProjectAuth from './routes/project/Auth';
import ProjectFunctions from './routes/project/Functions';
import ProjectAutomations from './routes/project/Automations';
import ProjectPlugins from './routes/project/Plugins';
import ProjectMcp from './routes/project/Mcp';
import ProjectLogs from './routes/project/Logs';
import ProjectAnalytics from './routes/project/Analytics';
import ProjectSettings from './routes/project/Settings';

function AuthGuard() {
  if (!hasToken()) return <Navigate to="/" replace />;
  return <Outlet />;
}

function GuestGuard() {
  if (hasToken()) return <Navigate to="/projects" replace />;
  return <Login />;
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<GuestGuard />} />

      <Route element={<AuthGuard />}>
        <Route element={<AppShell />}>
          <Route path="/projects" element={<Projects />} />
          <Route path="/plugins" element={<PluginLibrary />} />
          <Route path="/plugins/:pluginId" element={<PluginDetail />} />
          <Route path="/members" element={<Members />} />
          <Route path="/settings" element={<InstanceSettings />} />
          <Route path="/settings/:section" element={<InstanceSettings />} />
          <Route path="/account" element={<Account />} />

          <Route path="/projects/:id" element={<ProjectProvider />}>
            <Route index element={<ProjectHome />} />
            <Route path="database" element={<ProjectDatabase />} />
            <Route path="database/:collection" element={<ProjectDatabase />} />
            <Route path="storage" element={<ProjectStorage />} />
            <Route path="storage/:bucket" element={<ProjectStorage />} />
            <Route path="auth" element={<ProjectAuth />} />
            <Route path="auth/:section" element={<ProjectAuth />} />
            <Route path="functions" element={<ProjectFunctions />} />
            <Route path="functions/:fn" element={<ProjectFunctions />} />
            <Route path="automations" element={<ProjectAutomations />} />
            <Route path="automations/:kind/:itemId" element={<ProjectAutomations />} />
            <Route path="plugins" element={<ProjectPlugins />} />
            <Route path="plugins/:provider" element={<ProjectPlugins />} />
            <Route path="mcp" element={<ProjectMcp />} />
            <Route path="logs" element={<ProjectLogs />} />
            <Route path="analytics" element={<ProjectAnalytics />} />
            <Route path="settings" element={<ProjectSettings />} />
            <Route path="settings/:section" element={<ProjectSettings />} />
          </Route>

          {/* Old addresses */}
          <Route path="/dashboard" element={<Navigate to="/projects" replace />} />
          <Route path="/profile" element={<Navigate to="/account" replace />} />
        </Route>
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  );
}
