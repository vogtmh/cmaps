'use strict';

// ---- DOM refs ----
const $ = (id) => document.getElementById(id);
const viewport = $('viewport');
const stage = $('stage');
const rubber = $('rubber');
const fileInput = $('fileInput');
const dropHint = $('dropHint');
const loading = $('loading');
const loadingText = $('loadingText');

// ---- state ----
const state = {
  id: null,
  page: 0,
  filename: '',
  layers: [],          // [{name}]
  off: new Set(),      // hidden layer names (pending until Apply)
  appliedOff: [],      // last applied
  svg: null,           // <svg> element
  content: null,       // wrapper <g> holding page content (for bbox)
  natW: 0, natH: 0,    // full page size (viewBox units == css px)
  cropX: 0, cropY: 0, cropW: 0, cropH: 0, // content bbox (+pad) used for fit + export
  s: 1, tx: 0, ty: 0,  // view transform
  selected: new Set(), // selected SVG elements
  hoverEl: null,
  undo: [],            // stack of removal batches
  spaceDown: false,
};

const SELECTABLE = new Set(['path', 'image', 'rect', 'circle', 'ellipse', 'line', 'polyline', 'polygon', 'text', 'use']);

// ============================================================ upload
fileInput.addEventListener('change', () => {
  if (fileInput.files.length) uploadFile(fileInput.files[0]);
});

['dragover', 'dragenter'].forEach((ev) =>
  viewport.addEventListener(ev, (e) => { e.preventDefault(); viewport.classList.add('dragover'); }));
['dragleave', 'drop'].forEach((ev) =>
  viewport.addEventListener(ev, (e) => { e.preventDefault(); viewport.classList.remove('dragover'); }));
viewport.addEventListener('drop', (e) => {
  const f = e.dataTransfer.files[0];
  if (f) uploadFile(f);
});

async function uploadFile(file) {
  if (!/\.pdf$/i.test(file.name)) { alert('Please choose a PDF file (raster tracing is not implemented yet).'); return; }
  showLoading('Uploading & analyzing…');
  try {
    const fd = new FormData();
    fd.append('file', file);
    const res = await fetch('/api/upload', { method: 'POST', body: fd });
    if (!res.ok) throw new Error(await res.text());
    const meta = await res.json();
    state.id = meta.id;
    state.page = 0;
    state.filename = meta.filename || file.name;
    state.layers = (meta.layers || []).map((n) => ({ name: n }));
    state.off = new Set();
    state.appliedOff = [];
    $('filename').textContent = state.filename + (meta.pages > 1 ? ` (page 1/${meta.pages})` : '');
    buildLayersPanel();
    await loadSVG();
  } catch (err) {
    hideLoading();
    alert('Upload failed: ' + err.message);
  }
}

// ============================================================ render / load svg
async function loadSVG() {
  showLoading('Rendering…');
  try {
    const res = await fetch(`/api/svg?id=${encodeURIComponent(state.id)}&page=${state.page}`);
    if (!res.ok) throw new Error(await res.text());
    const text = await res.text();
    stage.innerHTML = text;
    const svg = stage.querySelector('svg');
    if (!svg) throw new Error('no SVG returned');
    initSVG(svg);
  } catch (err) {
    alert('Render failed: ' + err.message);
  } finally {
    hideLoading();
  }
}

function initSVG(svg) {
  state.svg = svg;
  state.selected.clear();
  state.hoverEl = null;
  updateSelInfo();

  // Determine natural size from viewBox (fallback to width/height attrs).
  let W = 0, H = 0;
  const vb = svg.getAttribute('viewBox');
  if (vb) {
    const p = vb.trim().split(/[\s,]+/).map(Number);
    W = p[2]; H = p[3];
  }
  if (!W || !H) {
    W = parseFloat(svg.getAttribute('width')) || svg.getBoundingClientRect().width || 1000;
    H = parseFloat(svg.getAttribute('height')) || svg.getBoundingClientRect().height || 1000;
    if (!svg.getAttribute('viewBox')) svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  }
  // Wrap all page content in a group so we can measure its bounding box
  // reliably (getBBox on the root <svg> is unreliable across browsers).
  const wrap = document.createElementNS(SVG_NS, 'g');
  while (svg.firstChild) wrap.appendChild(svg.firstChild);
  svg.appendChild(wrap);
  state.content = wrap;

  // Keep the live SVG at full page size + origin (0,0) so screen<->user mapping
  // and getEnclosureList/getIntersectionList stay simple. Auto-crop is done by
  // fitting the VIEW to the content box, and by cropping only the export clone.
  state.natW = W; state.natH = H;
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  svg.style.width = W + 'px';
  svg.style.height = H + 'px';
  svg.setAttribute('preserveAspectRatio', 'xMinYMin meet');
  state.cropX = 0; state.cropY = 0; state.cropW = W; state.cropH = H;

  dropHint.style.display = 'none';
  $('exportBtn').disabled = false;
  updateUndoBtn();

  const n = svg.querySelectorAll(SELECTABLE_SELECTOR).length;
  $('statsHint').textContent = `${n.toLocaleString()} objects on this page.` +
    (n > 60000 ? ' Large file — drop layers first for smoother editing.' : '');

  computeCrop();
  fitView();
}

const SVG_NS = 'http://www.w3.org/2000/svg';
const SELECTABLE_SELECTOR = 'path,image,rect,circle,ellipse,line,polyline,polygon,text,use';

// Measure the content bounding box (+ small padding) for view-fit and export.
// Reversible: called after every (re)render, so toggling a hidden layer back on
// re-expands the crop. Does NOT alter the live viewBox (that would offset
// selection hit-testing).
function computeCrop() {
  let bbox = null;
  try { bbox = state.content.getBBox(); } catch (_) { bbox = null; }
  if (!bbox || !isFinite(bbox.width) || !isFinite(bbox.height) || bbox.width <= 0 || bbox.height <= 0) {
    state.cropX = 0; state.cropY = 0; state.cropW = state.natW; state.cropH = state.natH;
    return;
  }
  const pad = Math.max(bbox.width, bbox.height) * 0.02; // small padding
  state.cropX = bbox.x - pad;
  state.cropY = bbox.y - pad;
  state.cropW = bbox.width + pad * 2;
  state.cropH = bbox.height + pad * 2;
  updateCropMask();
}

// Grey out the margins that would be trimmed on export (instead of cropping the
// UI). Rebuilt whenever the crop changes, so it reflects layer toggles.
function updateCropMask() {
  if (!state.svg) return;
  state.svg.querySelectorAll(':scope > g.me-cropmask').forEach((n) => n.remove());
  const { cropX, cropY, cropW, cropH, natW, natH } = state;
  const eps = 0.5;
  if (cropX <= eps && cropY <= eps && cropW >= natW - eps && cropH >= natH - eps) return;
  const g = document.createElementNS(SVG_NS, 'g');
  g.setAttribute('class', 'me-cropmask');
  g.setAttribute('pointer-events', 'none');
  const rects = [
    [0, 0, natW, cropY],                                   // top
    [0, cropY + cropH, natW, natH - (cropY + cropH)],      // bottom
    [0, cropY, cropX, cropH],                              // left
    [cropX + cropW, cropY, natW - (cropX + cropW), cropH], // right
  ];
  for (const [x, y, w, h] of rects) {
    if (w <= 0 || h <= 0) continue;
    const r = document.createElementNS(SVG_NS, 'rect');
    r.setAttribute('x', x); r.setAttribute('y', y);
    r.setAttribute('width', w); r.setAttribute('height', h);
    r.setAttribute('fill', '#5b616b');
    r.setAttribute('fill-opacity', '0.5');
    g.appendChild(r);
  }
  state.svg.appendChild(g);
}

// ============================================================ view transform
function applyTransform() {
  stage.style.transform = `translate(${state.tx}px, ${state.ty}px) scale(${state.s})`;
  $('zoomLabel').textContent = Math.round(state.s * 100) + '%';
}

function fitView() {
  const r = viewport.getBoundingClientRect();
  const cw = state.natW, ch = state.natH;
  if (!cw || !ch) return;
  const s = Math.min(r.width / cw, r.height / ch) * 0.95;
  state.s = s;
  // Center the full page in the viewport (margins shown greyed via the mask).
  state.tx = (r.width - cw * s) / 2;
  state.ty = (r.height - ch * s) / 2;
  applyTransform();
}

function zoomAt(clientX, clientY, factor) {
  const r = viewport.getBoundingClientRect();
  const px = clientX - r.left, py = clientY - r.top;
  const ux = (px - state.tx) / state.s;
  const uy = (py - state.ty) / state.s;
  state.s = Math.min(64, Math.max(0.02, state.s * factor));
  state.tx = px - ux * state.s;
  state.ty = py - uy * state.s;
  applyTransform();
}

// Wheel zoom, coalesced into one transform update per animation frame. Applying
// a transform on every wheel event caused paint trails on large SVGs.
let wheelAccum = 0, wheelX = 0, wheelY = 0, wheelRAF = null;
viewport.addEventListener('wheel', (e) => {
  if (!state.svg) return;
  e.preventDefault();
  wheelAccum += e.deltaY;
  wheelX = e.clientX; wheelY = e.clientY;
  if (wheelRAF) return;
  wheelRAF = requestAnimationFrame(() => {
    const dy = wheelAccum; wheelAccum = 0; wheelRAF = null;
    if (dy !== 0) zoomAt(wheelX, wheelY, Math.pow(1.12, -dy / 100));
  });
}, { passive: false });

$('zoomIn').addEventListener('click', () => centerZoom(1.2));
$('zoomOut').addEventListener('click', () => centerZoom(1 / 1.2));
$('zoomFit').addEventListener('click', fitView);
function centerZoom(f) {
  const r = viewport.getBoundingClientRect();
  zoomAt(r.left + r.width / 2, r.top + r.height / 2, f);
}

// screen (client) -> svg user coords (viewBox origin is 0,0)
function toUser(clientX, clientY) {
  const r = viewport.getBoundingClientRect();
  return {
    x: (clientX - r.left - state.tx) / state.s,
    y: (clientY - r.top - state.ty) / state.s,
  };
}

// ============================================================ pointer: select / pan / rubber-band
let drag = null; // {startX,startY,mode,downEl,moved}

viewport.addEventListener('pointerdown', (e) => {
  if (!state.svg) return;
  const pan = state.spaceDown || e.button === 1;
  if (e.button !== 0 && !pan) return;
  viewport.setPointerCapture(e.pointerId);
  drag = {
    startX: e.clientX, startY: e.clientY,
    mode: pan ? 'pan' : 'select',
    downEl: pan ? null : selectableFrom(e.target),
    moved: false,
    baseTx: state.tx, baseTy: state.ty,
  };
  if (pan) viewport.classList.add('panning');
});

viewport.addEventListener('pointermove', (e) => {
  if (!state.svg) return;

  if (!drag) { updateHover(e.target); return; }

  const dx = e.clientX - drag.startX, dy = e.clientY - drag.startY;
  if (!drag.moved && Math.hypot(dx, dy) > 4) drag.moved = true;

  if (drag.mode === 'pan') {
    state.tx = drag.baseTx + dx;
    state.ty = drag.baseTy + dy;
    applyTransform();
    return;
  }

  // select mode -> draw rubber-band once moved
  if (drag.moved) {
    clearHover();
    const r = viewport.getBoundingClientRect();
    const x = Math.min(e.clientX, drag.startX) - r.left;
    const y = Math.min(e.clientY, drag.startY) - r.top;
    rubber.style.display = 'block';
    rubber.style.left = x + 'px';
    rubber.style.top = y + 'px';
    rubber.style.width = Math.abs(dx) + 'px';
    rubber.style.height = Math.abs(dy) + 'px';
  }
});

viewport.addEventListener('pointerup', (e) => {
  if (!drag) return;
  const d = drag; drag = null;
  viewport.releasePointerCapture(e.pointerId);
  viewport.classList.remove('panning');

  if (d.mode === 'pan') return;

  if (d.moved) {
    rubber.style.display = 'none';
    boxSelect(d.startX, d.startY, e.clientX, e.clientY, e.shiftKey, e.altKey);
  } else {
    // plain click
    if (d.downEl) {
      if (e.shiftKey) toggleSelect(d.downEl);
      else setSelection([d.downEl]);
    } else if (!e.shiftKey) {
      setSelection([]);
    }
  }
});

function selectableFrom(node) {
  while (node && node !== state.svg) {
    if (node.nodeType === 1 && SELECTABLE.has(node.tagName.toLowerCase())) return node;
    node = node.parentNode;
  }
  return null;
}

// Box-select. By default only elements fully ENCLOSED by the box are picked
// (so a big wall/room path that merely crosses the box is ignored). Hold Alt
// for a "crossing" select that also grabs anything the box touches.
function boxSelect(x0, y0, x1, y1, additive, crossing) {
  const a = toUser(Math.min(x0, x1), Math.min(y0, y1));
  const b = toUser(Math.max(x0, x1), Math.max(y0, y1));
  const w = b.x - a.x, h = b.y - a.y;
  if (w <= 0 || h <= 0) return;

  const method = crossing ? 'getIntersectionList' : 'getEnclosureList';
  let hits = [];
  if (state.svg[method]) {
    try {
      const rect = state.svg.createSVGRect();
      rect.x = a.x; rect.y = a.y; rect.width = w; rect.height = h;
      const list = state.svg[method](rect, null);
      for (const el of list) {
        if (SELECTABLE.has(el.tagName.toLowerCase()) && !el.closest('.me-cropmask, .me-layerhi')) hits.push(el);
      }
    } catch (_) { hits = boxSelectFallback(x0, y0, x1, y1, crossing); }
  } else {
    hits = boxSelectFallback(x0, y0, x1, y1, crossing);
  }

  const next = additive ? new Set(state.selected) : new Set();
  hits.forEach((el) => next.add(el));
  setSelectionSet(next);
}

function boxSelectFallback(x0, y0, x1, y1, crossing) {
  const L = Math.min(x0, x1), T = Math.min(y0, y1), R = Math.max(x0, x1), B = Math.max(y0, y1);
  const out = [];
  state.svg.querySelectorAll(SELECTABLE_SELECTOR).forEach((el) => {
    if (el.closest('.me-cropmask, .me-layerhi')) return;
    const r = el.getBoundingClientRect();
    const touches = r.right >= L && r.left <= R && r.bottom >= T && r.top <= B;
    const inside = r.left >= L && r.right <= R && r.top >= T && r.bottom <= B;
    if (crossing ? touches : inside) out.push(el);
  });
  return out;
}

// ============================================================ selection model
function setSelection(arr) { setSelectionSet(new Set(arr)); }

function setSelectionSet(next) {
  // remove class from those no longer selected
  state.selected.forEach((el) => { if (!next.has(el)) el.classList.remove('me-selected'); });
  // add class to newly selected
  next.forEach((el) => { if (!state.selected.has(el)) el.classList.add('me-selected'); });
  state.selected = next;
  updateSelInfo();
}

function toggleSelect(el) {
  const next = new Set(state.selected);
  if (next.has(el)) next.delete(el); else next.add(el);
  setSelectionSet(next);
}

function updateSelInfo() {
  const n = state.selected.size;
  $('selInfo').textContent = n ? `${n} selected` : '';
  $('deleteBtn').disabled = n === 0;
}

function updateHover(target) {
  const el = selectableFrom(target);
  if (el === state.hoverEl) return;
  clearHover();
  if (el && !state.selected.has(el)) { el.classList.add('me-hover'); state.hoverEl = el; }
}
function clearHover() {
  if (state.hoverEl) { state.hoverEl.classList.remove('me-hover'); state.hoverEl = null; }
}
viewport.addEventListener('pointerleave', clearHover);

// ============================================================ delete / undo
function deleteSelection() {
  if (!state.selected.size) return;
  const batch = [];
  state.selected.forEach((el) => {
    el.classList.remove('me-selected');
    batch.push({ node: el, parent: el.parentNode, next: el.nextSibling });
    el.parentNode.removeChild(el);
  });
  state.undo.push(batch);
  state.selected = new Set();
  clearHover();
  updateSelInfo();
  updateUndoBtn();
}

function undo() {
  const batch = state.undo.pop();
  if (!batch) return;
  batch.forEach(({ node, parent, next }) => parent.insertBefore(node, next));
  updateUndoBtn();
}

function updateUndoBtn() { $('undoBtn').disabled = state.undo.length === 0; }
$('deleteBtn').addEventListener('click', deleteSelection);
$('undoBtn').addEventListener('click', undo);

window.addEventListener('keydown', (e) => {
  if (e.target.tagName === 'INPUT') return;
  if (e.code === 'Space') { state.spaceDown = true; viewport.classList.add('panready'); e.preventDefault(); return; }
  if (e.key === 'Delete' || e.key === 'Backspace') { e.preventDefault(); deleteSelection(); }
  else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z') { e.preventDefault(); undo(); }
  else if (e.key === 'Escape') setSelection([]);
});
window.addEventListener('keyup', (e) => {
  if (e.code === 'Space') { state.spaceDown = false; viewport.classList.remove('panready'); }
});

// ============================================================ layers panel
function buildLayersPanel() {
  const box = $('layers');
  layerGroupCache.clear();
  box.innerHTML = '';
  const has = state.layers.length > 0;
  $('noLayers').style.display = has ? 'none' : 'block';
  $('layersAll').disabled = !has;
  $('layersNone').disabled = !has;

  state.layers.forEach((ly) => {
    const row = document.createElement('div');
    row.className = 'layer-row';
    const tg = document.createElement('button');
    tg.type = 'button';
    tg.className = 'layer-toggle';
    tg.setAttribute('aria-pressed', 'true');
    tg.title = 'Show / hide layer';
    const lb = document.createElement('label');
    lb.textContent = ly.name; lb.title = ly.name;
    const toggle = () => {
      const hide = !tg.classList.contains('off'); // currently shown -> hide it
      tg.classList.toggle('off', hide);
      tg.setAttribute('aria-pressed', String(!hide));
      if (hide) state.off.add(ly.name); else state.off.delete(ly.name);
      row.classList.toggle('hidden', hide);
      applyLayers();
    };
    tg.addEventListener('click', toggle);
    lb.addEventListener('click', toggle);
    row.addEventListener('mouseenter', () => highlightLayer(ly.name));
    row.addEventListener('mouseleave', () => clearLayerHighlight());
    row.appendChild(tg); row.appendChild(lb);
    box.appendChild(row);
  });
}

// ---- layer hover highlight ----
// The flat SVG has no layer tags, so we fetch each layer in isolation from the
// server (cached), then overlay its shapes tinted on top of the current map.
const layerGroupCache = new Map(); // name -> <g> (detached, reusable)
let hoverLayer = null;
let layerFetchToken = 0;

async function highlightLayer(name) {
  if (!state.svg) return;
  hoverLayer = name;
  if (layerGroupCache.has(name)) { showLayerHighlight(name); return; }
  const token = ++layerFetchToken;
  try {
    const res = await fetch(`/api/svg?id=${encodeURIComponent(state.id)}&page=${state.page}&only=${encodeURIComponent(name)}`);
    if (!res.ok) return;
    const text = await res.text();
    const src = new DOMParser().parseFromString(text, 'image/svg+xml').querySelector('svg');
    const g = document.createElementNS(SVG_NS, 'g');
    g.setAttribute('class', 'me-layerhi');
    g.setAttribute('pointer-events', 'none');
    if (src) {
      // Snapshot the child list: importNode COPIES (does not remove) the source
      // node, so iterating src.firstChild would loop forever.
      Array.from(src.childNodes).forEach((n) => g.appendChild(document.importNode(n, true)));
    }
    layerGroupCache.set(name, g);
    if (hoverLayer === name && token === layerFetchToken) showLayerHighlight(name);
  } catch (_) { /* ignore */ }
}

function showLayerHighlight(name) {
  clearHighlightDom();
  const g = layerGroupCache.get(name);
  if (g && state.svg) state.svg.appendChild(g);
}

function clearLayerHighlight() {
  hoverLayer = null;
  clearHighlightDom();
}

function clearHighlightDom() {
  if (state.svg) state.svg.querySelectorAll(':scope > g.me-layerhi').forEach((n) => n.remove());
}

$('layersAll').addEventListener('click', () => setAllLayers(true));
$('layersNone').addEventListener('click', () => setAllLayers(false));
function setAllLayers(on) {
  state.off = on ? new Set() : new Set(state.layers.map((l) => l.name));
  $('layers').querySelectorAll('.layer-row').forEach((row) => {
    const tg = row.querySelector('.layer-toggle');
    tg.classList.toggle('off', !on);
    tg.setAttribute('aria-pressed', String(on));
    row.classList.toggle('hidden', !on);
  });
  applyLayers();
}

// Apply the current hidden-layer set to the server and re-render. Toggles made
// while a render is in flight are coalesced into one follow-up render.
let applyingLayers = false;
let applyPending = false;
async function applyLayers() {
  if (applyingLayers) { applyPending = true; return; }
  applyingLayers = true;
  const off = [...state.off];
  showLoading('Applying layers…');
  try {
    const res = await fetch('/api/layers', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: state.id, off }),
    });
    if (!res.ok) throw new Error(await res.text());
    state.appliedOff = off;
    state.undo = []; updateUndoBtn();
    await loadSVG();
  } catch (err) {
    hideLoading();
    alert('Apply failed: ' + err.message);
  } finally {
    applyingLayers = false;
    if (applyPending) { applyPending = false; applyLayers(); }
  }
}

// ============================================================ export
$('exportBtn').addEventListener('click', exportPNG);

async function exportPNG() {
  if (!state.svg) return;
  // Re-crop to the current content so the export is tight (also reflects any
  // manual deletions since the last render).
  computeCrop();
  const width = Math.max(200, Math.min(16000, parseInt($('exportWidth').value, 10) || 3000));
  const height = Math.round(width * state.cropH / state.cropW);

  // Clone, strip editor-only styling/transform.
  const clone = state.svg.cloneNode(true);
  clone.querySelectorAll('.me-selected, .me-hover').forEach((el) => {
    el.classList.remove('me-selected'); el.classList.remove('me-hover');
  });
  clone.querySelectorAll('g.me-layerhi, g.me-cropmask').forEach((el) => el.remove());
  clone.style.transform = '';
  clone.setAttribute('width', width);
  clone.setAttribute('height', height);
  clone.setAttribute('viewBox', `${state.cropX} ${state.cropY} ${state.cropW} ${state.cropH}`);

  const svgText = new XMLSerializer().serializeToString(clone);
  const blob = new Blob([svgText], { type: 'image/svg+xml;charset=utf-8' });
  const url = URL.createObjectURL(blob);

  showLoading('Rasterizing…');
  try {
    const img = await loadImage(url);
    const canvas = document.createElement('canvas');
    canvas.width = width; canvas.height = height;
    const ctx = canvas.getContext('2d');
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, width, height);
    ctx.drawImage(img, 0, 0, width, height);
    URL.revokeObjectURL(url);

    const png = await new Promise((r) => canvas.toBlob(r, 'image/png'));
    const name = ($('exportName').value.trim() || baseName(state.filename) || 'map');

    // download locally
    const a = document.createElement('a');
    a.href = URL.createObjectURL(png);
    a.download = name + '.png';
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000);

    // also save server-side
    const fd = new FormData();
    fd.append('id', state.id);
    fd.append('name', name);
    fd.append('image', png, name + '.png');
    await fetch('/api/export', { method: 'POST', body: fd });
  } catch (err) {
    alert('Export failed: ' + err.message + '\n(A very large width can exceed browser canvas limits — try a smaller width.)');
  } finally {
    hideLoading();
  }
}

function loadImage(url) {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error('image decode failed'));
    img.src = url;
  });
}

function baseName(f) { return (f || '').replace(/\.[^.]+$/, ''); }

// ============================================================ helpers
function showLoading(t) { loadingText.textContent = t || 'Working…'; loading.hidden = false; }
function hideLoading() { loading.hidden = true; }
window.addEventListener('resize', () => { if (state.svg) applyTransform(); });
