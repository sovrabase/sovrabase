// Fonction déclenchée à la création d'un document (trigger: record:create).
// La variable globale `event` contient le contexte.

console.log('Nouveau document:', event.collection, event.id);
console.log('Données:', JSON.stringify(event.data));
