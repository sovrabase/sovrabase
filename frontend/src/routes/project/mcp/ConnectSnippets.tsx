import { useState } from 'react';
import { CodeBlock, CopyButton, Tabs } from '../../../ui';
import { CLIENTS, snippet, type ClientKey } from './model';

/** Ready-to-paste client config, filled with a token when one was just created. */
export function ConnectSnippets({ url, token }: { url: string; token?: string | null }) {
  const [client, setClient] = useState<ClientKey>('claude-code');
  const text = snippet(client, url, token || 'YOUR_TOKEN');
  const where = CLIENTS.find((c) => c.value === client)?.where;
  return (
    <div className="flex flex-col gap-3 min-w-0">
      <Tabs<ClientKey> tabs={CLIENTS.map(({ value, label }) => ({ value, label }))} value={client} onChange={setClient} />
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="m-0 text-[12.5px] text-ink-3 flex-1 min-w-[200px]">{where}</p>
        <CopyButton text={text} />
      </div>
      <CodeBlock>{text}</CodeBlock>
      <p className="m-0 text-[12.5px] text-ink-3">
        {token
          ? 'This snippet holds the token you just created. Copy it now: it will not be shown again.'
          : 'Replace YOUR_TOKEN with a token from Access tokens. Creating a token fills it in for you.'}
      </p>
    </div>
  );
}
