import { useEffect, useState } from 'react';
import { Download, FileText, Trash2 } from 'lucide-react';
import { formatBytes, formatDate } from '../../../api';
import { Button, CopyButton, Sheet, Spinner } from '../../../ui';
import { fetchBlob, fileName, kindOf, type StorageFile } from './files';

/** One file: preview, facts, link, download and delete. */
export function FileSheet({
  projectId,
  bucket,
  file,
  onClose,
  onDelete,
}: {
  projectId: string;
  bucket: string;
  file: StorageFile | null;
  onClose: () => void;
  onDelete: (f: StorageFile) => void;
}) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [text, setText] = useState<string | null>(null);
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle');
  const kind = kindOf(file?.content_type);

  useEffect(() => {
    setBlobUrl(null);
    setText(null);
    if (!file || kind === 'other') return;
    let url: string | null = null;
    let live = true;
    setState('loading');
    fetchBlob(projectId, bucket, file.path)
      .then(async (b) => {
        if (!live) return;
        if (kind === 'text') {
          const t = await b.slice(0, 100_000).text();
          if (live) setText(t + (b.size > 100_000 ? '\n…' : ''));
        } else {
          url = URL.createObjectURL(b);
          setBlobUrl(url);
        }
        if (live) setState('idle');
      })
      .catch(() => live && setState('error'));
    return () => {
      live = false;
      if (url) URL.revokeObjectURL(url);
    };
  }, [projectId, bucket, file, kind]);

  const download = async () => {
    if (!file) return;
    const b = await fetchBlob(projectId, bucket, file.path);
    const u = URL.createObjectURL(b);
    const a = document.createElement('a');
    a.href = u;
    a.download = fileName(file.path);
    a.click();
    setTimeout(() => URL.revokeObjectURL(u), 1000);
  };

  return (
    <Sheet
      open={!!file}
      onClose={onClose}
      title={<span className="break-all">{file ? fileName(file.path) : ''}</span>}
      description={file ? <span className="font-mono text-[12px] break-all">{bucket}/{file.path}</span> : undefined}
      footer={
        file && (
          <>
            <Button variant="ghost" className="mr-auto hover:!text-danger hover:!bg-danger-soft" icon={<Trash2 className="w-4 h-4" />} onClick={() => onDelete(file)}>
              Delete…
            </Button>
            <Button icon={<Download className="w-4 h-4" />} onClick={download}>
              Download
            </Button>
          </>
        )
      }
    >
      {file && (
        <>
          <div className="rounded-lg border border-line bg-sunken min-h-[180px] flex items-center justify-center overflow-hidden">
            {state === 'loading' ? (
              <Spinner label="Loading preview" />
            ) : state === 'error' ? (
              <p className="m-0 p-6 text-[13px] text-ink-3">The preview could not be loaded.</p>
            ) : kind === 'image' && blobUrl ? (
              <img src={blobUrl} alt={fileName(file.path)} className="max-w-full max-h-[50vh] object-contain" />
            ) : kind === 'video' && blobUrl ? (
              <video src={blobUrl} controls className="max-w-full max-h-[50vh]" />
            ) : kind === 'audio' && blobUrl ? (
              <audio src={blobUrl} controls className="w-full m-4" />
            ) : kind === 'pdf' && blobUrl ? (
              <iframe src={blobUrl} title={fileName(file.path)} className="w-full h-[50vh] bg-surface" />
            ) : kind === 'text' && text != null ? (
              <pre className="m-0 p-4 w-full self-stretch font-mono text-[12px] leading-5 text-ink-2 whitespace-pre-wrap break-all max-h-[50vh] overflow-auto">{text}</pre>
            ) : (
              <div className="flex flex-col items-center gap-2 p-6 text-ink-3">
                <FileText className="w-8 h-8" />
                <p className="m-0 text-[13px]">No preview for this type of file.</p>
              </div>
            )}
          </div>

          <dl className="m-0 grid grid-cols-[110px_1fr] gap-x-4 gap-y-2 text-[13px]">
            <dt className="text-ink-3">Size</dt>
            <dd className="m-0 text-ink">{file.size != null ? formatBytes(file.size) : '—'}</dd>
            <dt className="text-ink-3">Type</dt>
            <dd className="m-0 text-ink font-mono text-[12.5px]">{file.content_type || '—'}</dd>
            <dt className="text-ink-3">Uploaded</dt>
            <dd className="m-0 text-ink">{formatDate(file.created_at || file.updated_at)}</dd>
            {file.updated_at && file.updated_at !== file.created_at && (
              <>
                <dt className="text-ink-3">Updated</dt>
                <dd className="m-0 text-ink">{formatDate(file.updated_at)}</dd>
              </>
            )}
          </dl>

          {file.url && (
            <div className="flex flex-col gap-1.5">
              <span className="text-[13px] font-semibold text-ink">Link</span>
              <div className="flex items-center gap-2 min-w-0">
                <code className="flex-1 min-w-0 truncate font-mono text-[12.5px] text-ink-2 px-3 h-9 leading-9 rounded-md bg-sunken border border-line">{file.url}</code>
                <CopyButton text={file.url} />
              </div>
            </div>
          )}
        </>
      )}
    </Sheet>
  );
}
