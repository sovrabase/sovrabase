import type { ReactNode } from 'react';

interface StatCardProps {
  icon: ReactNode;
  label: string;
  value: string | number;
  subtitle?: string;
}

export function StatCard({ icon, label, value, subtitle }: StatCardProps) {
  return (
    <div className="bg-bg-card border border-border rounded-xl p-5 flex flex-col gap-2">
      <div className="flex items-center gap-3">
        <div className="text-accent">{icon}</div>
        <div>
          <p className="text-2xl font-semibold text-text-primary">{value}</p>
          <p className="text-sm text-text-muted">{label}</p>
        </div>
      </div>
      {subtitle && (
        <p className="text-xs text-text-secondary mt-1">{subtitle}</p>
      )}
    </div>
  );
}
