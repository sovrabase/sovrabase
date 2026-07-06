export function renderFormattedJson(obj: unknown) {
  const jsonStr = JSON.stringify(obj, null, 2);
  return (
    <pre className="font-code-xs text-code-xs leading-relaxed overflow-x-auto p-3 rounded-lg bg-surface-container-lowest/70 border border-outline-variant/30 text-left">
      <code>
        {jsonStr.split('\n').map((line, index) => {
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
