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
  mode: 'vector',      // 'vector' (PDF/SVG) or 'raster' (PNG/JPG)
  layers: [],          // [{name}]
  off: new Set(),      // hidden layer names (pending until Apply)
  appliedOff: [],      // last applied
  svg: null,           // <svg> element (vector mode)
  content: null,       // wrapper <g> holding page content (for bbox)
  canvas: null,        // <canvas> (raster mode)
  ctx: null,           // 2d context (raster mode)
  bgColor: '#ffffff',  // erase colour (raster mode)
  rtool: 'rect',       // raster tool: 'rect' | 'brush' | 'pick'
  brush: 48,           // brush diameter in image px
  rasterUndo: [],      // stack of ImageData snapshots
  natW: 0, natH: 0,    // full page size (viewBox units == css px)
  cropX: 0, cropY: 0, cropW: 0, cropH: 0, // content bbox (+pad) used for fit + export
  s: 1, tx: 0, ty: 0,  // view transform
  selected: new Set(), // selected SVG elements
  hoverEl: null,
  undo: [],            // stack of removal batches
  deletedSigs: new Map(), // geometry signature -> count of manual deletions (survives re-render)
  sigToLayer: new Map(),  // geometry signature -> layer name (reverse index)
  layerIndexBuilt: false,
  spaceDown: false,
};

const docLoaded = () => state.svg || state.canvas;

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
  if (!/\.(pdf|png|jpe?g)$/i.test(file.name)) { alert('Please choose a PDF or image (PNG/JPG) file.'); return; }
  const isPDF = /\.pdf$/i.test(file.name);
  showLoading(isPDF ? 'Uploading & analyzing…' : 'Uploading…');
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
    state.deletedSigs = new Map();
    state.sigToLayer = new Map();
    state.layerIndexBuilt = false;
    state.undo = [];
    $('filename').textContent = state.filename + (meta.pages > 1 ? ` (page 1/${meta.pages})` : '');
    applyMode((meta.kind || (isPDF ? 'pdf' : 'png')) === 'pdf' ? 'vector' : 'raster');
    if (state.mode === 'vector') {
      buildLayersPanel();
      await loadSVG();
      buildLayerIndex(); // background: enables map-hover -> layer-row highlight
    } else {
      await loadRaster();
    }
  } catch (err) {
    hideLoading();
    alert('Upload failed: ' + err.message);
  }
}

// Toggle the UI between vector (PDF) and raster (image) editing.
function applyMode(mode) {
  state.mode = mode;
  $('vectorPanel').hidden = mode !== 'vector';
  $('rasterPanel').hidden = mode !== 'raster';
  $('deleteBtn').style.display = mode === 'raster' ? 'none' : '';
}

// ============================================================ render / load svg
async function loadSVG() {
  showLoading('Rendering…');
  try {
    const res = await fetch(`/api/svg?id=${encodeURIComponent(state.id)}&page=${state.page}`);
    if (!res.ok) throw new Error(await res.text());
    const text = await res.text();
    state.canvas = null; state.ctx = null;
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

// ============================================================ raster editor
async function loadRaster() {
  showLoading('Loading image…');
  try {
    const img = await loadImage(`/api/image?id=${encodeURIComponent(state.id)}`);
    const canvas = document.createElement('canvas');
    canvas.width = img.naturalWidth || img.width;
    canvas.height = img.naturalHeight || img.height;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    ctx.drawImage(img, 0, 0);
    state.svg = null; state.content = null;
    state.canvas = canvas; state.ctx = ctx;
    state.natW = canvas.width; state.natH = canvas.height;
    state.cropX = 0; state.cropY = 0; state.cropW = canvas.width; state.cropH = canvas.height;
    state.rasterUndo = [];
    stage.innerHTML = '';
    stage.appendChild(canvas);
    dropHint.style.display = 'none';
    $('exportBtn').disabled = false;
    updateUndoBtn();
    rasterComputeCrop();
    fitView();
  } catch (err) {
    alert('Load failed: ' + err.message);
  } finally {
    hideLoading();
  }
}

// image px painting helpers
function pushRasterUndo() {
  try {
    state.rasterUndo.push(state.ctx.getImageData(0, 0, state.canvas.width, state.canvas.height));
    if (state.rasterUndo.length > 15) state.rasterUndo.shift();
  } catch (_) { /* ignore */ }
  updateUndoBtn();
}
function brushDab(x, y) {
  const ctx = state.ctx;
  ctx.fillStyle = state.bgColor;
  ctx.beginPath();
  ctx.arc(x, y, state.brush / 2, 0, Math.PI * 2);
  ctx.fill();
}
function brushLine(x0, y0, x1, y1) {
  const ctx = state.ctx;
  ctx.strokeStyle = state.bgColor;
  ctx.lineWidth = state.brush;
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';
  ctx.beginPath();
  ctx.moveTo(x0, y0);
  ctx.lineTo(x1, y1);
  ctx.stroke();
}
function eraseRectScreen(x0, y0, x1, y1) {
  const a = toUser(Math.min(x0, x1), Math.min(y0, y1));
  const b = toUser(Math.max(x0, x1), Math.max(y0, y1));
  state.ctx.fillStyle = state.bgColor;
  state.ctx.fillRect(a.x, a.y, b.x - a.x, b.y - a.y);
}
function pickColorAt(clientX, clientY) {
  const p = toUser(clientX, clientY);
  const x = Math.max(0, Math.min(state.canvas.width - 1, Math.round(p.x)));
  const y = Math.max(0, Math.min(state.canvas.height - 1, Math.round(p.y)));
  const d = state.ctx.getImageData(x, y, 1, 1).data;
  const hex = '#' + [d[0], d[1], d[2]].map((v) => v.toString(16).padStart(2, '0')).join('');
  state.bgColor = hex;
  $('bgColorInput').value = hex;
  // revert to rectangle tool after picking
  setRasterTool('rect');
}
function setRasterTool(t) {
  state.rtool = t;
  const radio = document.querySelector(`input[name=rtool][value=${t}]`);
  if (radio) radio.checked = true;
  $('pickColor').classList.toggle('active', t === 'pick');
  updateBrushCursorVisibility();
}
function updateBrushCursorVisibility() {
  const show = state.mode === 'raster' && state.rtool === 'brush';
  $('brushCursor').style.display = show ? 'block' : 'none';
}
function updateBrushCursor(e) {
  if (state.mode !== 'raster' || state.rtool !== 'brush') { $('brushCursor').style.display = 'none'; return; }
  const c = $('brushCursor');
  const size = state.brush * state.s;
  c.style.display = 'block';
  c.style.width = size + 'px';
  c.style.height = size + 'px';
  c.style.left = e.clientX + 'px';
  c.style.top = e.clientY + 'px';
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

  reapplyDeletions();
  computeCrop();
  fitView();
}

const SVG_NS = 'http://www.w3.org/2000/svg';
const SELECTABLE_SELECTOR = 'path,image,rect,circle,ellipse,line,polyline,polygon,text,use';

// Recompute the content crop for the current mode (updates the grey mask).
function recomputeCrop() {
  if (state.mode === 'raster') rasterComputeCrop(); else computeCrop();
}

// Measure the content bounding box (+ small padding) for view-fit and export.
// Reversible: called after every (re)render, so toggling a hidden layer back on
// re-expands the crop. Does NOT alter the live viewBox (that would offset
// selection hit-testing).
function computeCrop() {
  let bbox = null;
  try { bbox = state.content.getBBox(); } catch (_) { bbox = null; }
  if (!bbox || !isFinite(bbox.width) || !isFinite(bbox.height) || bbox.width <= 0 || bbox.height <= 0) {
    state.cropX = 0; state.cropY = 0; state.cropW = state.natW; state.cropH = state.natH;
    updateCropMask();
    return;
  }
  const padX = bbox.width * 0.02, padY = bbox.height * 0.02; // small per-axis padding
  state.cropX = bbox.x - padX;
  state.cropY = bbox.y - padY;
  state.cropW = bbox.width + padX * 2;
  state.cropH = bbox.height + padY * 2;
  updateCropMask();
}

// Raster crop: bounding box of non-background (non-white) pixels + padding.
function rasterComputeCrop() {
  if (!state.ctx) return;
  const w = state.canvas.width, h = state.canvas.height;
  let data;
  try { data = state.ctx.getImageData(0, 0, w, h).data; } catch (_) { return; }
  const step = Math.max(1, Math.floor(Math.sqrt((w * h) / 1000000))); // ~1MP scan budget
  let minX = w, minY = h, maxX = -1, maxY = -1;
  for (let y = 0; y < h; y += step) {
    for (let x = 0; x < w; x += step) {
      const i = (y * w + x) * 4;
      const bg = data[i + 3] < 10 || (data[i] > 244 && data[i + 1] > 244 && data[i + 2] > 244);
      if (!bg) {
        if (x < minX) minX = x; if (x > maxX) maxX = x;
        if (y < minY) minY = y; if (y > maxY) maxY = y;
      }
    }
  }
  if (maxX < 0) { state.cropX = 0; state.cropY = 0; state.cropW = w; state.cropH = h; updateCropMask(); return; }
  const padX = (maxX - minX) * 0.02, padY = (maxY - minY) * 0.02;
  state.cropX = Math.max(0, minX - padX);
  state.cropY = Math.max(0, minY - padY);
  state.cropW = Math.min(w, maxX + 1 + padX) - state.cropX;
  state.cropH = Math.min(h, maxY + 1 + padY) - state.cropY;
  updateCropMask();
}

// Grey out the margins that would be trimmed on export (instead of cropping the
// UI). Rebuilt whenever the crop changes, so it reflects edits + layer toggles.
function updateCropMask() {
  if (state.mode === 'raster') { updateRasterMask(); return; }
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

// Raster mask: a DOM overlay inside #stage. A "hole" div at the crop box casts a
// large box-shadow that greys everything else, clipped to the canvas bounds.
function updateRasterMask() {
  const old = document.getElementById('rasterMask');
  if (old) old.remove();
  const { cropX, cropY, cropW, cropH, natW, natH } = state;
  const eps = 0.5;
  if (cropX <= eps && cropY <= eps && cropW >= natW - eps && cropH >= natH - eps) return;
  const mask = document.createElement('div');
  mask.id = 'rasterMask';
  mask.style.cssText = `position:absolute;left:0;top:0;width:${natW}px;height:${natH}px;overflow:hidden;pointer-events:none;`;
  const hole = document.createElement('div');
  hole.style.cssText = `position:absolute;left:${cropX}px;top:${cropY}px;width:${cropW}px;height:${cropH}px;box-shadow:0 0 0 100000px rgba(91,97,107,0.5);`;
  mask.appendChild(hole);
  stage.appendChild(mask);
}

// ============================================================ view transform
let willChangeTimer = null;
function applyTransform() {
  stage.style.transform = `translate(${state.tx}px, ${state.ty}px) scale(${state.s})`;
  $('zoomLabel').textContent = Math.round(state.s * 100) + '%';
  // Promote to a GPU layer during interaction (prevents paint trails), then drop
  // it after things settle so the SVG re-rasterizes crisply at the final zoom.
  stage.style.willChange = 'transform';
  clearTimeout(willChangeTimer);
  willChangeTimer = setTimeout(() => { stage.style.willChange = 'auto'; }, 200);
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
  if (!docLoaded()) return;
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
  if (!docLoaded()) return;
  const pan = state.spaceDown || e.button === 1;
  if (e.button !== 0 && !pan) return;
  viewport.setPointerCapture(e.pointerId);

  if (pan) {
    drag = { startX: e.clientX, startY: e.clientY, mode: 'pan', moved: false, baseTx: state.tx, baseTy: state.ty };
    viewport.classList.add('panning');
    return;
  }

  if (state.mode === 'raster') {
    if (state.rtool === 'pick') { pickColorAt(e.clientX, e.clientY); return; }
    if (state.rtool === 'brush') {
      pushRasterUndo();
      const p = toUser(e.clientX, e.clientY);
      brushDab(p.x, p.y);
      drag = { startX: e.clientX, startY: e.clientY, mode: 'rbrush', moved: false, last: p };
    } else {
      drag = { startX: e.clientX, startY: e.clientY, mode: 'rrect', moved: false };
    }
    return;
  }

  drag = { startX: e.clientX, startY: e.clientY, mode: 'select', downEl: selectableFrom(e.target), moved: false };
});

viewport.addEventListener('pointermove', (e) => {
  if (!docLoaded()) return;
  if (state.mode === 'raster') updateBrushCursor(e);

  if (!drag) { if (state.mode === 'vector') updateHover(e.target); return; }

  const dx = e.clientX - drag.startX, dy = e.clientY - drag.startY;
  if (!drag.moved && Math.hypot(dx, dy) > 4) drag.moved = true;

  if (drag.mode === 'pan') {
    state.tx = drag.baseTx + dx;
    state.ty = drag.baseTy + dy;
    applyTransform();
    return;
  }

  if (drag.mode === 'rbrush') {
    const p = toUser(e.clientX, e.clientY);
    brushLine(drag.last.x, drag.last.y, p.x, p.y);
    drag.last = p;
    return;
  }

  // rrect (raster) or select (vector) -> draw rubber-band once moved
  if (drag.moved) {
    if (state.mode === 'vector') clearHover();
    const r = viewport.getBoundingClientRect();
    rubber.style.display = 'block';
    rubber.style.left = (Math.min(e.clientX, drag.startX) - r.left) + 'px';
    rubber.style.top = (Math.min(e.clientY, drag.startY) - r.top) + 'px';
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

  if (d.mode === 'rbrush') { recomputeCrop(); updateUndoBtn(); return; }

  if (d.mode === 'rrect') {
    rubber.style.display = 'none';
    if (d.moved) { pushRasterUndo(); eraseRectScreen(d.startX, d.startY, e.clientX, e.clientY); recomputeCrop(); }
    return;
  }

  // vector select
  if (d.moved) {
    rubber.style.display = 'none';
    boxSelect(d.startX, d.startY, e.clientX, e.clientY, e.shiftKey, e.altKey);
  } else {
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
  if (el && !state.selected.has(el)) { el.classList.add('me-hover'); state.hoverEl = el; highlightLayerRowFor(el); }
}
function clearHover() {
  if (state.hoverEl) { state.hoverEl.classList.remove('me-hover'); state.hoverEl = null; }
  highlightLayerRowFor(null);
}
viewport.addEventListener('pointerleave', () => { clearHover(); $('brushCursor').style.display = 'none'; });

// ---- raster tool controls ----
document.querySelectorAll('input[name=rtool]').forEach((r) => {
  r.addEventListener('change', () => { if (r.checked) setRasterTool(r.value); });
});
$('pickColor').addEventListener('click', () => setRasterTool('pick'));
$('bgColorInput').addEventListener('input', (e) => { state.bgColor = e.target.value; });
$('brushSize').addEventListener('input', (e) => {
  state.brush = parseInt(e.target.value, 10) || 48;
  $('brushSizeVal').textContent = state.brush;
});

// ---- layer hover highlight colour ----
function setHighlightColor(c) { document.documentElement.style.setProperty('--hl', c); }
$('hlColor').addEventListener('input', (e) => setHighlightColor(e.target.value));
setHighlightColor($('hlColor').value);

// ============================================================ delete / undo
function deleteSelection() {
  if (!state.selected.size) return;
  const batch = [];
  state.selected.forEach((el) => {
    el.classList.remove('me-selected');
    const s = signature(el);
    bumpSig(s, 1);
    batch.push({ node: el, parent: el.parentNode, next: el.nextSibling, sig: s });
    el.parentNode.removeChild(el);
  });
  state.undo.push(batch);
  state.selected = new Set();
  clearHover();
  updateSelInfo();
  updateUndoBtn();
  computeCrop();
}

function undo() {
  if (state.mode === 'raster') {
    const img = state.rasterUndo.pop();
    if (img) state.ctx.putImageData(img, 0, 0);
    updateUndoBtn();
    return;
  }
  const batch = state.undo.pop();
  if (!batch) return;
  batch.forEach(({ node, parent, next, sig }) => { parent.insertBefore(node, next); bumpSig(sig, -1); });
  updateUndoBtn();
  computeCrop();
}

// ---- persistent deletions (survive server re-renders on layer toggle) ----
// The SVG is regenerated on every layer change, so we remember deleted elements
// by a geometry signature and re-remove matching elements after each render.
function signature(el) {
  const t = el.tagName.toLowerCase();
  const d = el.getAttribute('d');
  if (d != null) return t + '|d|' + d;
  const href = el.getAttribute('href') || el.getAttribute('xlink:href');
  if (href != null) return t + '|h|' + href;
  const pts = el.getAttribute('points');
  if (pts != null) return t + '|p|' + pts;
  return t + '|a|' + ['x', 'y', 'width', 'height', 'cx', 'cy', 'r', 'rx', 'ry', 'x1', 'y1', 'x2', 'y2']
    .map((a) => el.getAttribute(a) || '').join(',');
}
function bumpSig(s, delta) {
  const n = (state.deletedSigs.get(s) || 0) + delta;
  if (n <= 0) state.deletedSigs.delete(s); else state.deletedSigs.set(s, n);
}
function reapplyDeletions() {
  if (!state.deletedSigs.size || !state.content) return;
  const remaining = new Map(state.deletedSigs);
  state.content.querySelectorAll(SELECTABLE_SELECTOR).forEach((el) => {
    const s = signature(el);
    const c = remaining.get(s);
    if (c) {
      el.remove();
      if (c - 1 <= 0) remaining.delete(s); else remaining.set(s, c - 1);
    }
  });
}

function updateUndoBtn() {
  const has = state.mode === 'raster' ? state.rasterUndo.length > 0 : state.undo.length > 0;
  $('undoBtn').disabled = !has;
}
$('deleteBtn').addEventListener('click', deleteSelection);
$('undoBtn').addEventListener('click', undo);

window.addEventListener('keydown', (e) => {
  if (e.target.tagName === 'INPUT') return;
  if (e.code === 'Space') { state.spaceDown = true; viewport.classList.add('panready'); e.preventDefault(); return; }
  if (e.key === 'Delete' || e.key === 'Backspace') { if (state.mode === 'vector') { e.preventDefault(); deleteSelection(); } }
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
  layerRowByName.clear();
  activeLayerName = null;
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
    layerRowByName.set(ly.name, row);
  });
}

// ---- layer hover highlight ----
// The flat SVG has no layer tags, so we fetch each layer in isolation from the
// server (cached), then overlay its shapes tinted on top of the current map.
const layerGroupCache = new Map(); // name -> <g> (detached, reusable)
const layerRowByName = new Map();  // name -> sidebar row element
let hoverLayer = null;
let layerFetchToken = 0;
let layerIndexToken = 0;
let activeLayerName = null;

// Fetch (and cache) a layer's isolated content as a highlight <g>.
async function getLayerGroup(name) {
  if (layerGroupCache.has(name)) return layerGroupCache.get(name);
  const res = await fetch(`/api/svg?id=${encodeURIComponent(state.id)}&page=${state.page}&only=${encodeURIComponent(name)}`);
  if (!res.ok) return null;
  const src = new DOMParser().parseFromString(await res.text(), 'image/svg+xml').querySelector('svg');
  const g = document.createElementNS(SVG_NS, 'g');
  g.setAttribute('class', 'me-layerhi');
  g.setAttribute('pointer-events', 'none');
  if (src) Array.from(src.childNodes).forEach((n) => g.appendChild(document.importNode(n, true)));
  layerGroupCache.set(name, g);
  return g;
}

async function highlightLayer(name) {
  if (!state.svg) return;
  hoverLayer = name;
  if (layerGroupCache.has(name)) { showLayerHighlight(name); return; }
  const token = ++layerFetchToken;
  const g = await getLayerGroup(name);
  if (g && hoverLayer === name && token === layerFetchToken) showLayerHighlight(name);
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

// Reverse index: signature -> layer name, so hovering a shape on the map can
// highlight the owning layer row. Built once per file in the background.
async function buildLayerIndex() {
  if (state.mode !== 'vector' || !state.layers.length) return;
  const token = ++layerIndexToken;
  state.sigToLayer = new Map();
  state.layerIndexBuilt = false;
  const total = state.layers.length;
  let done = 0;
  for (const ly of state.layers) {
    if (token !== layerIndexToken) return; // cancelled by a new upload
    const g = await getLayerGroup(ly.name);
    if (token !== layerIndexToken) return;
    if (g) {
      g.querySelectorAll(SELECTABLE_SELECTOR).forEach((el) => {
        const s = signature(el);
        if (!state.sigToLayer.has(s)) state.sigToLayer.set(s, ly.name);
      });
    }
    done++;
    $('statsHint').textContent = `Mapping layers for hover… ${done}/${total}`;
  }
  if (token === layerIndexToken) { state.layerIndexBuilt = true; $('statsHint').textContent = ''; }
}

// Highlight the sidebar row of the layer that owns the hovered element.
function highlightLayerRowFor(el) {
  const name = el ? (state.sigToLayer.get(signature(el)) || null) : null;
  if (name === activeLayerName) return;
  if (activeLayerName) { const r = layerRowByName.get(activeLayerName); if (r) r.classList.remove('active'); }
  activeLayerName = name;
  if (name) {
    const r = layerRowByName.get(name);
    if (r) { r.classList.add('active'); r.scrollIntoView({ block: 'nearest' }); }
  }
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
$('exportProfile').addEventListener('change', () => {
  const adjustable = $('exportProfile').value !== 'original';
  $('profileToggle').hidden = !adjustable;
  if (!adjustable) $('profileOpts').hidden = true;
});
$('profileToggle').addEventListener('click', () => {
  $('profileOpts').hidden = !$('profileOpts').hidden;
});

function hexToRgb(hex) {
  const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex || '');
  return m ? { r: parseInt(m[1], 16), g: parseInt(m[2], 16), b: parseInt(m[3], 16) } : { r: 255, g: 255, b: 255 };
}

// Post-process the rasterized export in place. "whiteTransparent" /
// "blackTransparent": key the background colour to transparent, turn remaining
// content white/black, with opacity from each pixel's distance to the background
// (× a brightness factor).
function applyExportProfile(ctx, w, h) {
  const profile = $('exportProfile').value;
  if (profile !== 'whiteTransparent' && profile !== 'blackTransparent') return;
  const lineVal = profile === 'blackTransparent' ? 0 : 255;
  const key = hexToRgb($('keyColor').value);
  const tol = parseInt($('keyTol').value, 10) || 0;
  const bright = (parseInt($('keyBright').value, 10) || 150) / 100;
  const gain = 255 / 441.673; // max colour distance = sqrt(3*255^2)
  const img = ctx.getImageData(0, 0, w, h);
  const d = img.data;
  for (let i = 0; i < d.length; i += 4) {
    const dr = d[i] - key.r, dg = d[i + 1] - key.g, db = d[i + 2] - key.b;
    const dist = Math.sqrt(dr * dr + dg * dg + db * db);
    if (dist <= tol) { d[i + 3] = 0; continue; } // background -> transparent
    d[i] = lineVal; d[i + 1] = lineVal; d[i + 2] = lineVal; // content -> white/black
    d[i + 3] = Math.min(255, Math.round(dist * gain * bright));
  }
  ctx.putImageData(img, 0, 0);
}

async function exportPNG() {
  if (state.mode === 'raster') { rasterExport(); return; }
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
  // Drop the inline style (full-page width/height + transform) so the width/height
  // ATTRIBUTES below actually take effect — otherwise the export keeps the full-page
  // aspect and gets stretched into the crop-aspect canvas.
  clone.removeAttribute('style');
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

    applyExportProfile(ctx, width, height);
    const png = await new Promise((r) => canvas.toBlob(r, 'image/png'));
    const name = baseName(state.filename) || 'map';

    // download locally
    const a = document.createElement('a');
    a.href = URL.createObjectURL(png);
    a.download = name + '.png';
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000);

    // also save server-side (only for server-backed projects, i.e. PDFs)
    if (state.id) {
      const fd = new FormData();
      fd.append('id', state.id);
      fd.append('name', name);
      fd.append('image', png, name + '.png');
      await fetch('/api/export', { method: 'POST', body: fd });
    }
  } catch (err) {
    alert('Export failed: ' + err.message + '\n(A very large width can exceed browser canvas limits — try a smaller width.)');
  } finally {
    hideLoading();
  }
}

// Raster export: the edited canvas is already the final image (native resolution).
async function rasterExport() {
  if (!state.canvas) return;
  rasterComputeCrop();
  showLoading('Exporting…');
  try {
    let sx = Math.round(state.cropX || 0), sy = Math.round(state.cropY || 0);
    let sw = Math.round(state.cropW || state.canvas.width), sh = Math.round(state.cropH || state.canvas.height);
    if (sw <= 0 || sh <= 0) { sx = 0; sy = 0; sw = state.canvas.width; sh = state.canvas.height; }
    const out = document.createElement('canvas');
    out.width = sw; out.height = sh;
    const octx = out.getContext('2d');
    octx.drawImage(state.canvas, sx, sy, sw, sh, 0, 0, sw, sh);
    applyExportProfile(octx, sw, sh);
    const png = await new Promise((r) => out.toBlob(r, 'image/png'));
    const name = baseName(state.filename) || 'map';
    const a = document.createElement('a');
    a.href = URL.createObjectURL(png);
    a.download = name + '.png';
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000);
    if (state.id) {
      const fd = new FormData();
      fd.append('id', state.id);
      fd.append('name', name);
      fd.append('image', png, name + '.png');
      await fetch('/api/export', { method: 'POST', body: fd });
    }
  } catch (err) {
    alert('Export failed: ' + err.message);
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
window.addEventListener('resize', () => { if (docLoaded()) applyTransform(); });

// ---- resizable sidebar ----
(() => {
  const resizer = $('sidebarResizer'), sidebar = $('sidebar'), main = $('main');
  let dragging = false;
  resizer.addEventListener('pointerdown', (e) => {
    dragging = true;
    resizer.setPointerCapture(e.pointerId);
    resizer.classList.add('dragging');
    e.preventDefault();
  });
  resizer.addEventListener('pointermove', (e) => {
    if (!dragging) return;
    const w = Math.max(180, Math.min(700, main.getBoundingClientRect().right - e.clientX));
    sidebar.style.flex = '0 0 ' + w + 'px';
  });
  resizer.addEventListener('pointerup', (e) => {
    if (!dragging) return;
    dragging = false;
    resizer.classList.remove('dragging');
    resizer.releasePointerCapture(e.pointerId);
    if (docLoaded()) applyTransform();
  });
})();
