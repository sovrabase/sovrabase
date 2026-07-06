// Fonction HTTP — doit exporter une fonction `handler(req)`.
// req: { method, path, query, headers, body }
// Retour: { status, headers?, body }

function handler(req) {
  const name = (req.query && req.query.name) || 'monde';
  return {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    body: {
      message: 'Bonjour ' + name + ' !',
      method: req.method,
    },
  };
}
