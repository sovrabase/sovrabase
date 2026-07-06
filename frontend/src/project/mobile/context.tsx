import { createContext, useContext, type ReactNode } from 'react';
import { useMobileDashboardData, type MobileDashboardContextValue } from './hooks/useMobileDashboardData';
import type { MobileDashboardProps } from './types';

const MobileDashboardContext = createContext<MobileDashboardContextValue | null>(null);

export function MobileDashboardProvider({ children, ...props }: MobileDashboardProps & { children: ReactNode }) {
  const value = useMobileDashboardData(props);
  return <MobileDashboardContext.Provider value={value}>{children}</MobileDashboardContext.Provider>;
}

export function useMobileDashboard(): MobileDashboardContextValue {
  const ctx = useContext(MobileDashboardContext);
  if (!ctx) throw new Error('useMobileDashboard must be used within MobileDashboardProvider');
  return ctx;
}
