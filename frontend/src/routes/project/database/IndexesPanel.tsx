import { useCallback, useEffect, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { api } from '../../../api';
import { Badge, Banner, Button, Card, CardHeader, ConfirmDialog, DataTable, EmptyState, Field, IconButton, Input, Segmented, Sheet, Spinner, useToast } from '../../../ui';
import { colPath } from './shared';

interface IndexConfig {
  field: string;
  type: 'simple' | 'unique';
}

/** Indexes speed up queries on a field; unique ones also reject duplicates. */
export function IndexesPanel({ projectId, collection, fields }: { projectId: string; collection: string; fields: string[] }) {
  const { showToast } = useToast();
  const [items, setItems] = useState<IndexConfig[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [field, setField] = useState('');
  const [type, setType] = useState<'simple' | 'unique'>('simple');
  const [busy, setBusy] = useState(false);
  const [dropTarget, setDropTarget] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const d = await api<{ indexes: IndexConfig[] }>(colPath(projectId, collection, '/indexes'));
      setItems(d.indexes || []);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [projectId, collection]);

  useEffect(() => {
    setItems(null);
    load();
  }, [load]);

  const create = async () => {
    if (!field.trim()) return;
    setBusy(true);
    try {
      await api(colPath(projectId, collection, '/indexes'), { method: 'POST', body: JSON.stringify({ field: field.trim(), type }) });
      showToast(`Index on ${field.trim()} created`, 'success');
      setAdding(false);
      setField('');
      setType('simple');
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setBusy(false);
    }
  };

  const drop = async () => {
    if (!dropTarget) return;
    try {
      await api(colPath(projectId, collection, `/indexes/${encodeURIComponent(dropTarget)}`), { method: 'DELETE' });
      showToast('Index removed', 'success');
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  if (error) return <Banner tone="danger" title="Indexes could not be loaded">{error}</Banner>;
  if (!items) return <Spinner label="Loading indexes" />;

  const suggestions = fields.filter((f) => !items.some((i) => i.field === f) && !f.startsWith('_id'));

  return (
    <Card padded={false}>
      <CardHeader
        title="Indexes"
        description="Index the fields you filter or sort on. A unique index also refuses duplicate values."
        actions={
          <Button size="sm" icon={<Plus className="w-3.5 h-3.5" />} onClick={() => setAdding(true)}>
            New index
          </Button>
        }
      />
      <div className="border-t border-line">
        <DataTable
          rows={items}
          rowKey={(i) => i.field}
          empty={
            <EmptyState
              title="No indexes"
              description="Queries still work, they just scan every record."
              action={
                <Button variant="primary" size="sm" onClick={() => setAdding(true)}>
                  New index
                </Button>
              }
            />
          }
          columns={[
            { key: 'field', header: 'Field', cell: (i) => <span className="font-mono text-[13px] text-ink">{i.field}</span> },
            { key: 'type', header: 'Type', width: '120px', cell: (i) => <Badge tone={i.type === 'unique' ? 'accent' : 'neutral'}>{i.type === 'unique' ? 'Unique' : 'Simple'}</Badge> },
            {
              key: 'x',
              header: <span className="sr-only">Actions</span>,
              width: '60px',
              align: 'right',
              cell: (i) => (
                <IconButton size="sm" label={`Remove index on ${i.field}`} onClick={() => setDropTarget(i.field)}>
                  <Trash2 className="w-4 h-4" />
                </IconButton>
              ),
            },
          ]}
        />
      </div>

      <Sheet
        open={adding}
        onClose={() => setAdding(false)}
        title="New index"
        description={`On ${collection}`}
        footer={
          <>
            <Button variant="ghost" onClick={() => setAdding(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={busy} disabled={!field.trim()} onClick={create}>
              Create index
            </Button>
          </>
        }
      >
        <Field label="Field" hint="Use dots for nested fields, e.g. address.city.">
          <Input mono value={field} onChange={(e) => setField(e.target.value)} placeholder="email" list="idx-fields" onKeyDown={(e) => e.key === 'Enter' && create()} />
        </Field>
        <datalist id="idx-fields">
          {suggestions.map((f) => (
            <option key={f} value={f} />
          ))}
        </datalist>
        <Field label="Type">
          <Segmented
            aria-label="Index type"
            value={type}
            onChange={setType}
            options={[
              { value: 'simple', label: 'Simple' },
              { value: 'unique', label: 'Unique' },
            ]}
          />
        </Field>
      </Sheet>

      <ConfirmDialog open={!!dropTarget} onClose={() => setDropTarget(null)} onConfirm={drop} title={`Remove the index on ${dropTarget}?`} confirmLabel="Remove index">
        Records stay as they are. Queries on this field get slower{dropTarget && items.find((i) => i.field === dropTarget)?.type === 'unique' ? ' and duplicates are no longer refused' : ''}.
      </ConfirmDialog>
    </Card>
  );
}
