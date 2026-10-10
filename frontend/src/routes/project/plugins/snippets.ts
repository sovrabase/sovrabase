import type { CatalogEntry } from '../../instance/plugins/manifest';

/** How an app uses a plugin once it is installed (carried over from the old Integrations tab). */
export function snippetFor(def: CatalogEntry): string {
  const origin = window.location.origin;
  if (def.id === 'paypal') {
    return `// 1. Read public config from Sovrabase
const res = await fetch('${origin}/api/v1/integrations', {
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token }
});
const { integrations } = await res.json();
const paypal = integrations.find(i => i.id === 'paypal');

// 2. Create an order (server-side, uses your secret)
const order = await fetch('${origin}/api/v1/integrations/paypal/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: 'create_order', data: { amount: '10.00', currency: 'USD' } })
}).then(r => r.json());

// 3. Capture after approval
await fetch('${origin}/api/v1/integrations/paypal/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: 'capture_order', data: { order_id: order.id } })
});

// Webhook URL for PayPal dashboard:
//   ${origin}/api/v1/integrations/paypal/webhook?project_key=PROJECT_KEY`;
  } else if (def.category === 'notifications') {
    return `// AUTOMATIC: Notification webhooks fire on DB/API events.
// You do NOT need to call the action API manually. As soon as this
// integration is enabled, the server sends a message on:
//   - record:create (new document inserted)
//   - record:update (document modified)
//   - record:delete (document removed)
//   - auth:signup (new user registered)
//   - auth:signin (user logged in)
//
// EVENT FILTERING: Use the config modal to select which events trigger
// this integration, and optionally restrict it to specific collections
// (e.g. only "orders" document creations). Leave events unchecked to
// fire on everything; leave collections empty for all collections.
//
// Manual send (optional — for custom messages):
await fetch('${origin}/api/v1/integrations/${def.id}/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: 'send_message', data: { content: 'Custom message!' } })
});
`;
  } else if (def.id === 'sendgrid' || def.id === 'twilio') {
    return `// Server-side send
await fetch('${origin}/api/v1/integrations/${def.id}/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: '${def.id === 'sendgrid' ? 'send_email' : 'send_sms'}', data: { ... } })
});
`;
  } else if (def.id === 'algolia') {
    return `// Index a document
await fetch('${origin}/api/v1/integrations/algolia/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: 'save_object', data: { index: 'products', object: { objectID: '1', name: 'Widget', price: 9.99 } } })
});

// Search
const results = await fetch('${origin}/api/v1/integrations/algolia/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: 'search', data: { index: 'products', query: 'widget' } })
}).then(r => r.json());

// Available actions: save_object, save_objects, search,
//   delete_object, set_settings, list_indexes`;
  }
  return `// Server-side action
await fetch('${origin}/api/v1/integrations/${def.id}/action', {
  method: 'POST',
  headers: { 'X-Project-Key': PROJECT_KEY, 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ action: '...', data: { ... } })
});

// Webhook URL for ${def.name} dashboard:
//   ${origin}/api/v1/integrations/${def.id}/webhook?project_key=PROJECT_KEY`;
}
