/** The Sovrabase mark: twelve gold dots around a white square, on an ink tile. */
export function Mark({ size = 26 }: { size?: number }) {
  return (
    <span
      className="inline-flex items-center justify-center shrink-0 bg-[#0D1017] shadow-[inset_0_0_0_1px_rgba(255,255,255,.08)]"
      style={{ width: size, height: size, borderRadius: Math.round(size * 0.27) }}
      aria-hidden
    >
      <svg width={size * 0.7} height={size * 0.7} viewBox="0 0 24 24">
        <g fill="#F2B705">
          {Array.from({ length: 12 }, (_, i) => {
            const a = (i * Math.PI) / 6;
            return <circle key={i} cx={12 + 9 * Math.cos(a)} cy={12 + 9 * Math.sin(a)} r="1.6" />;
          })}
        </g>
        <rect x="9" y="9" width="6" height="6" rx="1.5" fill="#FFFFFF" />
      </svg>
    </span>
  );
}
