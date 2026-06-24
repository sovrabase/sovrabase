// === DATABASE EXPLORER ===

async function loadCollections() {
  try {
    const data = await api(`/admin/projects/${detailProjectId}/collections`);
    activeProjectEnv.collections = data.collections || [];
    renderCollectionsList();
  } catch (err) {
    showToast('Failed to load collections: ' + err.message, 'error');
  }
}

function renderCollectionsList() {
  const container = document.getElementById('fs-collections-list');
  if (!container) return;
  
  if (activeProjectEnv.collections.length === 0) {
    container.innerHTML = '<div style="padding:16px;text-align:center;color:var(--text-secondary);font-size:12px;">No collections</div>';
    selectCollection(null);
    return;
  }
  container.innerHTML = activeProjectEnv.collections.map(c => {
    const activeClass = activeProjectEnv.selectedCollection === c ? 'active' : '';
    return `<div class="fs-item ${activeClass}" onclick="selectCollection('${escapeHtml(c)}')">
      <span>${escapeHtml(c)}</span>
    </div>`;
  }).join('');
  
  if (activeProjectEnv.selectedCollection && !activeProjectEnv.collections.includes(activeProjectEnv.selectedCollection)) {
    selectCollection(null);
  }
}

function selectCollection(name) {
  activeProjectEnv.selectedCollection = name;
  document.querySelectorAll('#fs-collections-list .fs-item').forEach(el => {
    if (el.querySelector('span').textContent.trim() === name) el.classList.add('active');
    else el.classList.remove('active');
  });

  const dropBtn = document.getElementById('btn-drop-col-col1');
  const addDocBtn = document.getElementById('btn-add-doc-col2');
  const importDocBtn = document.getElementById('btn-import-doc-col2');
  const filterBar = document.getElementById('db-filter-bar');
  const filterInput = document.getElementById('db-filter-input');
  
  if (filterInput) filterInput.value = '';

  if (!name) {
    if (dropBtn) dropBtn.style.display = 'none';
    if (addDocBtn) addDocBtn.style.display = 'none';
    if (importDocBtn) importDocBtn.style.display = 'none';
    if (filterBar) filterBar.style.display = 'none';

    document.getElementById('fs-documents-list').innerHTML = `
      <div style="text-align:center; padding-top:40px; color:var(--text-secondary);" id="fs-no-collection-selected">
        <p style="font-size:11px;">Select a collection</p>
      </div>`;
    
    // Clear rules inputs
    const checkbox = document.getElementById('rls-enable-checkbox');
    if (checkbox) checkbox.checked = false;
    ['get', 'list', 'create', 'update', 'delete'].forEach(action => {
      const el = document.getElementById('rls-rule-' + action);
      if (el) el.value = '';
      const hl = document.getElementById('hl-' + action);
      if (hl) hl.innerHTML = '';
    });
    toggleRlsEnable(false);

    selectDocument(null);
    return;
  }
  
  if (dropBtn) dropBtn.style.display = 'block';
  if (addDocBtn) addDocBtn.style.display = 'block';
  if (importDocBtn) importDocBtn.style.display = 'block';
  if (filterBar) filterBar.style.display = 'flex';
  
  const noColEl = document.getElementById('fs-no-collection-selected');
  if (noColEl) noColEl.style.display = 'none';

  // If rules tab is currently active, load rules
  const rulesTab = document.getElementById('db-right-tab-rules');
  if (rulesTab && rulesTab.classList.contains('active')) {
    loadCollectionRules();
  }
  
  loadDocuments();
}

async function loadDocuments() {
  const col = activeProjectEnv.selectedCollection;
  if (!col) return;

  const filterInput = document.getElementById('db-filter-input');
  let url = `/admin/projects/${detailProjectId}/collections/${col}/documents`;

  if (filterInput && filterInput.value.trim() !== '') {
    const query = filterInput.value.trim();
    const separator = query.includes('=') ? '=' : (query.includes(':') ? ':' : null);
    if (separator) {
      const parts = query.split(separator);
      const key = parts[0].trim();
      const val = parts.slice(1).join(separator).trim();
      url += `?filter_key=${encodeURIComponent(key)}&filter_val=${encodeURIComponent(val)}`;
    }
  }

  try {
    const docs = await api(url);
    activeProjectEnv.documents = docs || [];
    applyDocumentFilter();
  } catch (err) {
    showToast('Failed to load documents: ' + err.message, 'error');
  }
}

function applyDocumentFilter() {
  const filterInput = document.getElementById('db-filter-input');
  if (!filterInput) return;
  const query = filterInput.value.trim().toLowerCase();

  let filtered = activeProjectEnv.documents;
  if (query !== '') {
    const separator = query.includes('=') ? '=' : (query.includes(':') ? ':' : null);
    if (separator) {
      const parts = query.split(separator);
      if (parts.length >= 2) {
        const key = parts[0].trim();
        const val = parts.slice(1).join(separator).trim();
        filtered = activeProjectEnv.documents.filter(doc => {
          return doc[key] !== undefined && String(doc[key]).toLowerCase().includes(val);
        });
      }
    } else {
      // General search
      filtered = activeProjectEnv.documents.filter(doc => {
        return JSON.stringify(doc).toLowerCase().includes(query);
      });
    }
  }

  renderFilteredDocuments(filtered);
}

function renderFilteredDocuments(filtered) {
  const container = document.getElementById('fs-documents-list');
  if (!container) return;
  
  if (filtered.length === 0) {
    container.innerHTML = '<div style="padding:16px;text-align:center;color:var(--text-secondary);font-size:12px;">No matching documents</div>';
    selectDocument(null);
    return;
  }
  container.innerHTML = filtered.map(d => {
    const id = d._id || '';
    const activeClass = activeProjectEnv.selectedDocumentId === id ? 'active' : '';
    return `<div class="fs-item ${activeClass}" onclick="selectDocument('${escapeHtml(id)}')">
      <span style="font-family:var(--font-mono);font-size:12px;">${escapeHtml(id)}</span>
    </div>`;
  }).join('');
  
  if (activeProjectEnv.selectedDocumentId) {
    const stillExists = filtered.some(d => d._id === activeProjectEnv.selectedDocumentId);
    if (stillExists) {
      selectDocument(activeProjectEnv.selectedDocumentId);
    } else {
      selectDocument(null);
    }
  }
}

function switchDbRightTab(tab) {
  const fieldsBtn = document.getElementById('db-right-tab-fields');
  const rulesBtn = document.getElementById('db-right-tab-rules');
  const fieldsContent = document.getElementById('fs-document-fields');
  const rulesContent = document.getElementById('fs-collection-rules');

  if (tab === 'fields') {
    fieldsBtn.classList.add('active');
    fieldsBtn.style.borderBottomColor = 'var(--accent)';
    fieldsBtn.style.color = 'var(--text-primary)';
    rulesBtn.classList.remove('active');
    rulesBtn.style.borderBottomColor = 'transparent';
    rulesBtn.style.color = 'var(--text-secondary)';

    fieldsContent.style.display = 'block';
    rulesContent.style.display = 'none';
  } else if (tab === 'rules') {
    rulesBtn.classList.add('active');
    rulesBtn.style.borderBottomColor = 'var(--accent)';
    rulesBtn.style.color = 'var(--text-primary)';
    fieldsBtn.classList.remove('active');
    fieldsBtn.style.borderBottomColor = 'transparent';
    fieldsBtn.style.color = 'var(--text-secondary)';

    fieldsContent.style.display = 'none';
    rulesContent.style.display = 'flex';

    loadCollectionRules();
  }
}

async function loadCollectionRules() {
  const col = activeProjectEnv.selectedCollection;
  if (!col) return;

  try {
    const data = await api(`/admin/projects/${detailProjectId}/collections/${col}/rules`);
    const enabled = data.enabled || false;
    const rules = data.rules || {};

    document.getElementById('rls-enable-checkbox').checked = enabled;
    
    // Backwards compatibility: if rules.read exists but not get/list, use it!
    const getVal = rules.get !== undefined ? rules.get : (rules.read || '');
    const listVal = rules.list !== undefined ? rules.list : (rules.read || '');

    document.getElementById('rls-rule-get').value = getVal;
    document.getElementById('rls-rule-list').value = listVal;
    document.getElementById('rls-rule-create').value = rules.create || '';
    document.getElementById('rls-rule-update').value = rules.update || '';
    document.getElementById('rls-rule-delete').value = rules.delete || '';

    // Trigger syntax highlights
    ['get', 'list', 'create', 'update', 'delete'].forEach(action => {
      updateRuleHighlight(action);
    });

    toggleRlsEnable(enabled);
  } catch (err) {
    showToast('Failed to load rules: ' + err.message, 'error');
  }
}

function toggleRlsEnable(enabled) {
  const inputsDiv = document.getElementById('rls-rules-inputs');
  if (!inputsDiv) return;
  if (enabled) {
    inputsDiv.style.opacity = '1';
    inputsDiv.style.pointerEvents = 'auto';
  } else {
    inputsDiv.style.opacity = '0.5';
    inputsDiv.style.pointerEvents = 'none';
  }
}

async function saveCollectionRules() {
  const col = activeProjectEnv.selectedCollection;
  if (!col) return;

  const enabled = document.getElementById('rls-enable-checkbox').checked;
  const getVal = document.getElementById('rls-rule-get').value.trim();
  const listVal = document.getElementById('rls-rule-list').value.trim();
  const create = document.getElementById('rls-rule-create').value.trim();
  const update = document.getElementById('rls-rule-update').value.trim();
  const deleteRule = document.getElementById('rls-rule-delete').value.trim();

  const body = {
    enabled: enabled,
    rules: {
      get: getVal,
      list: listVal,
      create: create,
      update: update,
      delete: deleteRule
    }
  };

  try {
    await api(`/admin/projects/${detailProjectId}/collections/${col}/rules`, {
      method: 'POST',
      body: JSON.stringify(body)
    });
    showToast('Rules saved successfully');
  } catch (err) {
    showToast('Failed to save rules: ' + err.message, 'error');
  }
}

function renderDocumentsList() {
  const container = document.getElementById('fs-documents-list');
  if (!container) return;
  
  if (activeProjectEnv.documents.length === 0) {
    container.innerHTML = '<div style="padding:16px;text-align:center;color:var(--text-secondary);font-size:12px;">No documents</div>';
    selectDocument(null);
    return;
  }
  container.innerHTML = activeProjectEnv.documents.map(d => {
    const id = d._id || '';
    const activeClass = activeProjectEnv.selectedDocumentId === id ? 'active' : '';
    return `<div class="fs-item ${activeClass}" onclick="selectDocument('${escapeHtml(id)}')">
      <span style="font-family:var(--font-mono);font-size:12px;">${escapeHtml(id)}</span>
    </div>`;
  }).join('');
  
  if (activeProjectEnv.selectedDocumentId) {
    const stillExists = activeProjectEnv.documents.some(d => d._id === activeProjectEnv.selectedDocumentId);
    if (stillExists) {
      selectDocument(activeProjectEnv.selectedDocumentId);
    } else {
      selectDocument(null);
    }
  }
}

function selectDocument(id) {
  activeProjectEnv.selectedDocumentId = id;
  document.querySelectorAll('#fs-documents-list .fs-item').forEach(el => {
    if (el.querySelector('span').textContent.trim() === id) el.classList.add('active');
    else el.classList.remove('active');
  });

  const deleteDocBtn = document.getElementById('btn-delete-doc-col3');
  const container = document.getElementById('fs-document-fields');

  if (!id) {
    activeProjectEnv.selectedDocument = null;
    if (deleteDocBtn) deleteDocBtn.style.display = 'none';
    if (container) {
      container.innerHTML = `
        <div style="text-align:center; padding-top:40px; color:var(--text-secondary);" id="fs-no-document-selected">
          <p style="font-size:11px;">Select a document</p>
        </div>`;
    }
    return;
  }

  if (deleteDocBtn) deleteDocBtn.style.display = 'block';

  const doc = activeProjectEnv.documents.find(d => d._id === id);
  activeProjectEnv.selectedDocument = doc;

  if (container) {
    container.innerHTML = '';
    // Render document metadata/edit bar
    const actionBar = document.createElement('div');
    actionBar.style.cssText = 'display:flex; justify-content:space-between; margin-bottom:12px; padding: 4px;';
    actionBar.innerHTML = `
      <div style="font-size:11px; font-family:var(--font-mono); color:var(--text-muted); align-self:center;">ID: ${escapeHtml(id)}</div>
      <button class="btn btn-xs btn-secondary" onclick="openEditDocModal('${escapeHtml(id)}')">Edit Fields</button>
    `;
    container.appendChild(actionBar);

    // Render tree container
    const treeContainer = document.createElement('div');
    treeContainer.className = 'tree-node-children';
    treeContainer.style.marginLeft = '0px';
    treeContainer.style.borderLeft = 'none';

    // Clone & delete internal _id for rendering
    const cleanDoc = { ...doc };
    delete cleanDoc._id;

    renderJSONTree(cleanDoc, treeContainer);
    container.appendChild(treeContainer);
  }
}

function renderJSONTree(val, container) {
  if (val === null) {
    const row = document.createElement('div');
    row.className = 'tree-row';
    row.innerHTML = `<span class="tree-val-null">null</span>`;
    container.appendChild(row);
    return;
  }

  if (typeof val === 'object') {
    const keys = Object.keys(val);

    keys.forEach(k => {
      const v = val[k];
      const node = document.createElement('div');
      node.className = 'tree-node';

      const row = document.createElement('div');
      row.className = 'tree-row';

      if (v !== null && typeof v === 'object') {
        const childCount = Array.isArray(v) ? v.length : Object.keys(v).length;
        const typeStr = Array.isArray(v) ? `Array [${childCount}]` : `Object {${childCount}}`;

        row.innerHTML = `
          <span class="tree-toggle expanded">▶</span>
          <span class="tree-key">${escapeHtml(k)}:</span>
          <span class="tree-type">${typeStr}</span>
        `;

        const childrenContainer = document.createElement('div');
        childrenContainer.className = 'tree-node-children';

        row.addEventListener('click', (e) => {
          e.stopPropagation();
          const toggle = row.querySelector('.tree-toggle');
          if (childrenContainer.classList.contains('collapsed')) {
            childrenContainer.classList.remove('collapsed');
            toggle.classList.add('expanded');
          } else {
            childrenContainer.classList.add('collapsed');
            toggle.classList.remove('expanded');
          }
        });

        node.appendChild(row);
        renderJSONTree(v, childrenContainer);
        node.appendChild(childrenContainer);
      } else {
        let typeStr = typeof v;
        if (v === null) typeStr = 'null';

        let valSpan = '';
        if (v === null) {
          valSpan = `<span class="tree-val-null">null</span>`;
        } else if (typeof v === 'string') {
          valSpan = `<span class="tree-val-string">"${escapeHtml(v)}"</span>`;
        } else if (typeof v === 'number') {
          valSpan = `<span class="tree-val-number">${v}</span>`;
        } else if (typeof v === 'boolean') {
          valSpan = `<span class="tree-val-boolean">${v}</span>`;
        }

        row.innerHTML = `
          <span class="tree-toggle" style="visibility:hidden;">▶</span>
          <span class="tree-key">${escapeHtml(k)}:</span>
          ${valSpan}
          <span class="tree-type">(${typeStr})</span>
        `;
        node.appendChild(row);
      }

      container.appendChild(node);
    });
  } else {
    const row = document.createElement('div');
    row.className = 'tree-row';
    row.innerHTML = `<span class="tree-val-string">${escapeHtml(String(val))}</span>`;
    container.appendChild(row);
  }
}

function openCreateCollectionModal() {
  document.getElementById('new-col-name').value = '';
  openSubModal('modal-create-col');
}

async function submitCreateCollection() {
  const name = document.getElementById('new-col-name').value.trim();
  if (!name) { showToast('Please enter a collection name', 'error'); return; }
  try {
    await api(`/admin/projects/${detailProjectId}/collections`, {
      method: 'POST',
      body: JSON.stringify({ name })
    });
    showToast(`Collection "${name}" created`);
    closeSubModal('modal-create-col');
    await loadCollections();
    selectCollection(name);
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function dropSelectedCollection() {
  const name = activeProjectEnv.selectedCollection;
  if (!name) return;
  if (!await showConfirm('Drop Collection', `Are you sure you want to drop collection "${name}" and all its documents?`, 'Drop', true)) return;
  try {
    await api(`/admin/projects/${detailProjectId}/collections/${name}`, {
      method: 'DELETE'
    });
    showToast(`Collection "${name}" dropped`);
    activeProjectEnv.selectedCollection = null;
    await loadCollections();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// === FIRESTORE-STYLE DYNAMIC DOCUMENT EDITOR ===

let editingDocId = null;

function addEditorFieldRoot() {
  const container = document.getElementById('doc-editor-fields-container');
  if (container) {
    container.appendChild(createFieldRow('', '', 'string', false));
  }
}

function openCreateDocModal() {
  editingDocId = null;
  document.getElementById('doc-modal-title').textContent = 'Create Document';
  
  const idInput = document.getElementById('doc-editor-id');
  idInput.value = '';
  idInput.disabled = false;

  const fieldsContainer = document.getElementById('doc-editor-fields-container');
  fieldsContainer.innerHTML = '';
  
  // Add one empty field row to start with for convenience
  fieldsContainer.appendChild(createFieldRow('', '', 'string', false));

  openSubModal('modal-create-doc');
}

function openEditDocModal(id) {
  editingDocId = id;
  const doc = activeProjectEnv.documents.find(d => d._id === id);
  document.getElementById('doc-modal-title').textContent = 'Edit Document';
  
  const idInput = document.getElementById('doc-editor-id');
  idInput.value = id;
  idInput.disabled = true;

  const fieldsContainer = document.getElementById('doc-editor-fields-container');
  fieldsContainer.innerHTML = '';

  // Populate fields from doc (excluding _id)
  const cleanDoc = { ...doc };
  delete cleanDoc._id;

  Object.keys(cleanDoc).forEach(k => {
    const v = cleanDoc[k];
    let t = 'string';
    if (v === null) t = 'null';
    else if (typeof v === 'number') t = 'number';
    else if (typeof v === 'boolean') t = 'boolean';
    else if (Array.isArray(v)) t = 'array';
    else if (typeof v === 'object') t = 'map';

    fieldsContainer.appendChild(createFieldRow(k, v, t, false));
  });

  if (fieldsContainer.children.length === 0) {
    fieldsContainer.appendChild(createFieldRow('', '', 'string', false));
  }

  openSubModal('modal-create-doc');
}

function createFieldRow(key = '', val = '', type = 'string', isArrayElement = false) {
  const row = document.createElement('div');
  row.className = 'editor-field-row';
  row.style.cssText = 'display:flex; flex-direction:column; gap:6px; padding:10px; background:rgba(255,255,255,0.01); border:1px solid var(--border); border-radius:var(--radius); margin-bottom: 4px;';

  // Upper bar: Key / index label, Type selector, Delete button
  const topBar = document.createElement('div');
  topBar.style.cssText = 'display:flex; gap:8px; align-items:center;';

  // Key Input / index label
  if (!isArrayElement) {
    const keyInput = document.createElement('input');
    keyInput.type = 'text';
    keyInput.className = 'field-key-input';
    keyInput.placeholder = 'field_name';
    keyInput.style.cssText = 'width:180px; padding:6px 10px; background:var(--bg-input); border:1px solid var(--border); border-radius:var(--radius); color:var(--text-primary); font-family:var(--font-mono); font-size:12.5px;';
    keyInput.value = key;
    topBar.appendChild(keyInput);
  } else {
    const idxLabel = document.createElement('span');
    idxLabel.style.cssText = 'width:60px; font-family:var(--font-mono); font-size:12px; color:var(--text-muted);';
    idxLabel.textContent = '[0]';
    idxLabel.className = 'array-index-label';
    topBar.appendChild(idxLabel);
  }

  // Type Selector
  const typeSelect = document.createElement('select');
  typeSelect.className = 'field-type-select';
  typeSelect.style.cssText = 'width:110px; padding:6px 10px; background:var(--bg-input); border:1px solid var(--border); border-radius:var(--radius); color:var(--text-primary); font-size:12px;';
  ['string', 'number', 'boolean', 'null', 'map', 'array'].forEach(t => {
    const opt = document.createElement('option');
    opt.value = t;
    opt.textContent = t;
    if (t === type) opt.selected = true;
    typeSelect.appendChild(opt);
  });
  topBar.appendChild(typeSelect);

  // Space filler
  const spacer = document.createElement('div');
  spacer.style.flex = '1';
  topBar.appendChild(spacer);

  // Delete button
  const delBtn = document.createElement('button');
  delBtn.className = 'btn btn-xs btn-danger';
  delBtn.style.cssText = 'padding:4px 8px; font-size:11px;';
  delBtn.innerHTML = '✕';
  delBtn.onclick = (e) => {
    e.preventDefault();
    const parent = row.parentElement;
    row.remove();
    if (isArrayElement) {
      updateArrayIndices(parent);
    }
  };
  topBar.appendChild(delBtn);
  row.appendChild(topBar);

  // Lower bar: Value container
  const valContainer = document.createElement('div');
  valContainer.className = 'field-val-container';
  valContainer.style.cssText = 'display:flex; flex-direction:column;';
  row.appendChild(valContainer);

  typeSelect.onchange = () => {
    updateValueField(valContainer, typeSelect.value);
    if (isArrayElement) {
      updateArrayIndices(row.parentElement);
    }
  };

  updateValueField(valContainer, type, val);

  return row;
}

function updateValueField(container, type, val) {
  container.innerHTML = '';
  
  if (type === 'string') {
    const input = document.createElement('input');
    input.type = 'text';
    input.className = 'field-val-input';
    input.placeholder = 'value';
    input.style.cssText = 'padding:6px 10px; background:var(--bg-input); border:1px solid var(--border); border-radius:var(--radius); color:var(--text-primary); font-size:12.5px;';
    input.value = typeof val === 'string' ? val : '';
    container.appendChild(input);
  } else if (type === 'number') {
    const input = document.createElement('input');
    input.type = 'number';
    input.step = 'any';
    input.className = 'field-val-input';
    input.style.cssText = 'padding:6px 10px; background:var(--bg-input); border:1px solid var(--border); border-radius:var(--radius); color:var(--text-primary); font-size:12.5px;';
    input.value = typeof val === 'number' ? val : 0;
    container.appendChild(input);
  } else if (type === 'boolean') {
    const select = document.createElement('select');
    select.className = 'field-val-input';
    select.style.cssText = 'padding:6px 10px; background:var(--bg-input); border:1px solid var(--border); border-radius:var(--radius); color:var(--text-primary); font-size:12.5px;';
    const optTrue = document.createElement('option');
    optTrue.value = 'true';
    optTrue.textContent = 'true';
    const optFalse = document.createElement('option');
    optFalse.value = 'false';
    optFalse.textContent = 'false';
    select.appendChild(optTrue);
    select.appendChild(optFalse);
    select.value = val === true ? 'true' : 'false';
    container.appendChild(select);
  } else if (type === 'null') {
    const span = document.createElement('span');
    span.style.cssText = 'padding:6px 10px; color:var(--text-muted); font-style:italic; font-size:12.5px;';
    span.textContent = 'null';
    container.appendChild(span);
  } else if (type === 'map') {
    const wrapper = document.createElement('div');
    wrapper.style.cssText = 'display:flex; flex-direction:column; gap:6px; border-left:2px solid var(--border); padding-left:12px; margin-top:4px;';
    
    const fieldsContainer = document.createElement('div');
    fieldsContainer.className = 'nested-fields-container';
    fieldsContainer.style.cssText = 'display:flex; flex-direction:column; gap:6px;';
    wrapper.appendChild(fieldsContainer);

    const addBtn = document.createElement('button');
    addBtn.className = 'btn btn-xs btn-secondary';
    addBtn.style.cssText = 'align-self:flex-start; margin-top:4px; font-size:10px;';
    addBtn.textContent = '+ Add Nested Field';
    addBtn.onclick = (e) => {
      e.preventDefault();
      fieldsContainer.appendChild(createFieldRow('', '', 'string', false));
    };
    wrapper.appendChild(addBtn);
    container.appendChild(wrapper);

    if (val && typeof val === 'object' && !Array.isArray(val)) {
      Object.keys(val).forEach(k => {
        const itemVal = val[k];
        let itemType = 'string';
        if (itemVal === null) itemType = 'null';
        else if (typeof itemVal === 'number') itemType = 'number';
        else if (typeof itemVal === 'boolean') itemType = 'boolean';
        else if (Array.isArray(itemVal)) itemType = 'array';
        else if (typeof itemVal === 'object') itemType = 'map';
        
        fieldsContainer.appendChild(createFieldRow(k, itemVal, itemType, false));
      });
    }
  } else if (type === 'array') {
    const wrapper = document.createElement('div');
    wrapper.style.cssText = 'display:flex; flex-direction:column; gap:6px; border-left:2px solid var(--border); padding-left:12px; margin-top:4px;';
    
    const arrayContainer = document.createElement('div');
    arrayContainer.className = 'nested-array-container';
    arrayContainer.style.cssText = 'display:flex; flex-direction:column; gap:6px;';
    wrapper.appendChild(arrayContainer);

    const addBtn = document.createElement('button');
    addBtn.className = 'btn btn-xs btn-secondary';
    addBtn.style.cssText = 'align-self:flex-start; margin-top:4px; font-size:10px;';
    addBtn.textContent = '+ Add Item';
    addBtn.onclick = (e) => {
      e.preventDefault();
      const newRow = createFieldRow('', '', 'string', true);
      arrayContainer.appendChild(newRow);
      updateArrayIndices(arrayContainer);
    };
    wrapper.appendChild(addBtn);
    container.appendChild(wrapper);

    if (Array.isArray(val)) {
      val.forEach(itemVal => {
        let itemType = 'string';
        if (itemVal === null) itemType = 'null';
        else if (typeof itemVal === 'number') itemType = 'number';
        else if (typeof itemVal === 'boolean') itemType = 'boolean';
        else if (Array.isArray(itemVal)) itemType = 'array';
        else if (typeof itemVal === 'object') itemType = 'map';
        
        arrayContainer.appendChild(createFieldRow('', itemVal, itemType, true));
      });
      updateArrayIndices(arrayContainer);
    }
  }
}

function updateArrayIndices(container) {
  const rows = container.querySelectorAll(':scope > .editor-field-row');
  rows.forEach((row, idx) => {
    const label = row.querySelector('.array-index-label');
    if (label) label.textContent = `[${idx}]`;
  });
}

function serializeFields(container) {
  const obj = {};
  const rows = container.querySelectorAll(':scope > .editor-field-row');
  
  rows.forEach(row => {
    const keyInput = row.querySelector('.field-key-input');
    const typeSelect = row.querySelector('.field-type-select');
    if (!keyInput || !typeSelect) return;

    const key = keyInput.value.trim();
    if (!key) return; // Skip empty keys

    const type = typeSelect.value;
    const valContainer = row.querySelector('.field-val-container');
    const val = getFieldValue(valContainer, type);

    obj[key] = val;
  });

  return obj;
}

function serializeArray(container) {
  const arr = [];
  const rows = container.querySelectorAll(':scope > .editor-field-row');

  rows.forEach(row => {
    const typeSelect = row.querySelector('.field-type-select');
    if (!typeSelect) return;

    const type = typeSelect.value;
    const valContainer = row.querySelector('.field-val-container');
    const val = getFieldValue(valContainer, type);

    arr.push(val);
  });

  return arr;
}

function getFieldValue(valContainer, type) {
  if (type === 'string') {
    const input = valContainer.querySelector('.field-val-input');
    return input ? input.value : '';
  } else if (type === 'number') {
    const input = valContainer.querySelector('.field-val-input');
    return input ? Number(input.value) : 0;
  } else if (type === 'boolean') {
    const select = valContainer.querySelector('.field-val-input');
    return select ? (select.value === 'true') : false;
  } else if (type === 'null') {
    return null;
  } else if (type === 'map') {
    const nestedContainer = valContainer.querySelector('.nested-fields-container');
    return nestedContainer ? serializeFields(nestedContainer) : {};
  } else if (type === 'array') {
    const nestedContainer = valContainer.querySelector('.nested-array-container');
    return nestedContainer ? serializeArray(nestedContainer) : [];
  }
  return null;
}

async function submitSaveDocument() {
  const fieldsContainer = document.getElementById('doc-editor-fields-container');
  if (!fieldsContainer) return;

  const doc = serializeFields(fieldsContainer);

  const idInput = document.getElementById('doc-editor-id');
  const docId = idInput.value.trim();

  const col = activeProjectEnv.selectedCollection;
  try {
    if (editingDocId) {
      await api(`/admin/projects/${detailProjectId}/collections/${col}/documents/${editingDocId}`, {
        method: 'PUT',
        body: JSON.stringify(doc)
      });
      showToast('Document updated');
    } else {
      if (docId) {
        doc._id = docId;
      }
      await api(`/admin/projects/${detailProjectId}/collections/${col}/documents`, {
        method: 'POST',
        body: JSON.stringify(doc)
      });
      showToast('Document created');
    }
    closeSubModal('modal-create-doc');
    await loadDocuments();
    if (editingDocId) {
      selectDocument(editingDocId);
    }
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function deleteSelectedDocument() {
  const id = activeProjectEnv.selectedDocumentId;
  if (!id) return;
  if (!await showConfirm('Delete Document', 'Are you sure you want to delete this document?', 'Delete', true)) return;
  const col = activeProjectEnv.selectedCollection;
  try {
    await api(`/admin/projects/${detailProjectId}/collections/${col}/documents/${id}`, {
      method: 'DELETE'
    });
    showToast('Document deleted');
    selectDocument(null);
    await loadDocuments();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

function triggerImportDoc() {
  document.getElementById('import-json-textarea').value = '[\n  \n]';
  openSubModal('modal-import-col');
}

async function submitImportCollection() {
  const raw = document.getElementById('import-json-textarea').value.trim();
  let docs;
  try {
    docs = JSON.parse(raw);
    if (!Array.isArray(docs)) {
      showToast('Input must be a JSON array of documents', 'error');
      return;
    }
  } catch (err) {
    showToast('Invalid JSON structure', 'error');
    return;
  }
  
  const col = activeProjectEnv.selectedCollection;
  try {
    const res = await api(`/admin/projects/${detailProjectId}/collections/${col}/import`, {
      method: 'POST',
      body: JSON.stringify(docs)
    });
    showToast(`Successfully imported ${res.count} documents`);
    closeSubModal('modal-import-col');
    selectDocument(null);
    await loadDocuments();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// ====== RLS RULES REAL-TIME SYNTAX HIGHLIGHTING & AUTOCOMPLETE ======

function highlightRuleSyntax(text) {
  let html = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

  // Strings
  html = html.replace(/(["'])(.*?)\1/g, '<span class="hl-str">$1$2$1</span>');

  // Keywords
  html = html.replace(/\b(auth|data|id)\b/g, '<span class="hl-key">$1</span>');

  // Properties
  html = html.replace(/\.(\w+)\b/g, '.<span class="hl-prop">$1</span>');

  // Constants
  html = html.replace(/\b(true|false|null)\b/g, '<span class="hl-const">$1</span>');

  // Operators
  html = html.replace(/(&amp;&amp;|\|\||==|!=|!)/g, '<span class="hl-op">$1</span>');

  return html;
}

function updateRuleHighlight(action) {
  const input = document.getElementById('rls-rule-' + action);
  const overlay = document.getElementById('hl-' + action);
  if (!input || !overlay) return;
  overlay.innerHTML = highlightRuleSyntax(input.value);
  overlay.scrollLeft = input.scrollLeft;
}

function syncRuleScroll(action) {
  const input = document.getElementById('rls-rule-' + action);
  const overlay = document.getElementById('hl-' + action);
  if (input && overlay) {
    overlay.scrollLeft = input.scrollLeft;
  }
}

let activeRulesInputId = null;

function handleRuleInputFocus(id) {
  activeRulesInputId = id;
}

function insertRuleKeyword(keyword) {
  if (!activeRulesInputId) return;
  const input = document.getElementById(activeRulesInputId);
  if (!input) return;

  const start = input.selectionStart;
  const end = input.selectionEnd;
  const text = input.value;
  input.value = text.substring(0, start) + keyword + text.substring(end);
  
  input.focus();
  input.selectionStart = input.selectionEnd = start + keyword.length;
  
  const action = activeRulesInputId.replace('rls-rule-', '');
  updateRuleHighlight(action);
}

// Autocomplete logic
const suggestionsList = ["auth", "auth.uid", "auth.role", "auth.email", "data", "true", "false", "null"];
let selectedAutocompleteIndex = 0;
let currentMatches = [];

function handleRuleInput(event, action) {
  updateRuleHighlight(action);
  
  const input = event.target;
  const val = input.value;
  const caretPos = input.selectionStart;
  
  const lastSpace = val.substring(0, caretPos).lastIndexOf(' ');
  const currentWord = val.substring(lastSpace + 1, caretPos).trim();
  
  const popover = document.getElementById('rules-autocomplete');
  if (!currentWord || currentWord.length < 1) {
    if (popover) popover.style.display = 'none';
    return;
  }
  
  currentMatches = suggestionsList.filter(item => item.startsWith(currentWord) && item !== currentWord);
  
  if (currentMatches.length === 0) {
    if (popover) popover.style.display = 'none';
    return;
  }
  
  selectedAutocompleteIndex = 0;
  renderAutocomplete(input);
}

function renderAutocomplete(input) {
  const popover = document.getElementById('rules-autocomplete');
  if (!popover) return;
  
  popover.innerHTML = currentMatches.map((m, idx) => {
    const activeClass = idx === selectedAutocompleteIndex ? 'active' : '';
    return `<div class="autocomplete-item ${activeClass}" onclick="insertSuggestion('${m}')" style="padding: 6px 10px; font-family: var(--font-mono); font-size: 11.5px; border-radius: 4px; cursor: pointer; transition: all var(--transition);">
      ${m}
    </div>`;
  }).join('');
  
  const wrapper = input.parentElement;
  popover.style.top = (wrapper.offsetTop + wrapper.offsetHeight + 4) + 'px';
  popover.style.left = wrapper.offsetLeft + 'px';
  popover.style.width = wrapper.offsetWidth + 'px';
  popover.style.display = 'flex';
}

function handleRuleKeydown(event, action) {
  const popover = document.getElementById('rules-autocomplete');
  if (!popover || popover.style.display === 'none') return;
  
  if (event.key === 'ArrowDown') {
    event.preventDefault();
    selectedAutocompleteIndex = (selectedAutocompleteIndex + 1) % currentMatches.length;
    renderAutocomplete(event.target);
  } else if (event.key === 'ArrowUp') {
    event.preventDefault();
    selectedAutocompleteIndex = (selectedAutocompleteIndex - 1 + currentMatches.length) % currentMatches.length;
    renderAutocomplete(event.target);
  } else if (event.key === 'Enter' || event.key === 'Tab') {
    event.preventDefault();
    if (currentMatches[selectedAutocompleteIndex]) {
      insertSuggestion(currentMatches[selectedAutocompleteIndex]);
    }
  } else if (event.key === 'Escape') {
    event.preventDefault();
    popover.style.display = 'none';
  }
}

function insertSuggestion(val) {
  if (!activeRulesInputId) return;
  const input = document.getElementById(activeRulesInputId);
  if (!input) return;
  
  const caretPos = input.selectionStart;
  const fullText = input.value;
  
  const lastSpace = fullText.substring(0, caretPos).lastIndexOf(' ');
  const start = lastSpace + 1;
  
  input.value = fullText.substring(0, start) + val + fullText.substring(caretPos);
  
  input.focus();
  input.selectionStart = input.selectionEnd = start + val.length;
  
  const action = activeRulesInputId.replace('rls-rule-', '');
  updateRuleHighlight(action);
  
  const popover = document.getElementById('rules-autocomplete');
  if (popover) popover.style.display = 'none';
}

// Close autocomplete popup when clicking outside rules fields
document.addEventListener('click', (e) => {
  const popover = document.getElementById('rules-autocomplete');
  if (popover && !e.target.classList.contains('rules-input') && !e.target.classList.contains('autocomplete-item')) {
    popover.style.display = 'none';
  }
});
