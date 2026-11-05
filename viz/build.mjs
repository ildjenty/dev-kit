// Throwaway viewer: renders Archon workflow YAML files as SVG graphs in one HTML page.
import fs from 'node:fs';
import path from 'node:path';
import dagre from '@dagrejs/dagre';
import YAML from 'yaml';

const dir = process.argv[2] ?? '../drafts/workflows';
const KINDS = ['prompt', 'command', 'bash', 'script', 'wait', 'loop', 'loop_group', 'workflow', 'include', 'cancel', 'approval'];
const COLOR = { ai: '#dbeafe', mech: '#e5e7eb', human: '#fecaca', child: '#dcfce7', other: '#fef9c3' };
const W = 260, PAD = 8, LINE = 16;
const textW = (t, px) => [...String(t)].reduce((a, c) => a + (c.charCodeAt(0) > 0x2e80 ? px : px * 0.58), 0);
const lines = (t, px) => (t ? Math.max(1, Math.ceil(textW(t, px) / (W - PAD * 2 - 4))) : 0);
function parts(n, k) {
  const badges = [n.provider, n.context === 'fresh' ? 'fresh' : '', n.fan_out ? 'fan_out' : '', n.trigger_rule ?? '', n.output_format ? 'schema' : ''].filter(Boolean).join(' · ');
  return { title: `${n.id}  [${k}]`, main: trunc(mainValue(n, k), 120), desc: n.description ?? '', when: n.when ? 'when: ' + n.when : '', badges };
}
function heightOf(n, k) {
  const p = parts(n, k);
  return PAD * 2 + LINE * (1 + lines(p.main, 11) + lines(p.desc, 12) + lines(p.when, 11) + (p.badges ? 1 : 0));
}
const esc = s => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const trunc = (s, n) => (s.length > n ? s.slice(0, n - 1) + '…' : s);

function kindOf(n) { return KINDS.find(k => n[k] !== undefined) ?? 'prompt'; }
function group(n, k) {
  if (k === 'wait' || k === 'approval' || n.labels?.to === 'human') return 'human';
  if (k === 'prompt' || k === 'command' || k === 'loop') return 'ai';
  if (k === 'bash' || k === 'script') return 'mech';
  if (k === 'workflow' || k === 'include') return 'child';
  return 'other';
}
function mainValue(n, k) {
  const v = n[k];
  if (k === 'wait') return `${v.form ?? 'message'}: ${typeof v.attention === 'string' ? v.attention : ''}`;
  if (k === 'loop_group') return '';
  return typeof v === 'string' ? v : JSON.stringify(v);
}

function layout(wf) {
  const g = new dagre.graphlib.Graph({ compound: true, multigraph: true });
  g.setGraph({ rankdir: 'TB', ranksep: 70, nodesep: 30, marginx: 20, marginy: 20 });
  g.setDefaultEdgeLabel(() => ({}));
  const info = {};
  const add = (n, parent) => {
    const k = kindOf(n);
    info[n.id] = { n, k, parent };
    if (k === 'loop_group') {
      g.setNode(n.id, { cluster: true, paddingTop: 40 });
      if (parent) g.setParent(n.id, parent);
      for (const c of n.loop_group.nodes ?? []) add(c, n.id);
    } else {
      g.setNode(n.id, { width: W, height: heightOf(n, k) });
      if (parent) g.setParent(n.id, parent);
    }
  };
  for (const n of wf.nodes ?? []) add(n);
  const edges = [];
  for (const [id, { n }] of Object.entries(info)) {
    for (const d of n.depends_on ?? []) if (info[d]) edges.push({ from: d, to: id, type: 'dep' });
    const r = n.context?.resume;
    if (r && info[r]) edges.push({ from: r, to: id, type: 'resume' });
  }
  // dagre cannot route edges touching clusters; attach to first/last body node instead.
  const firstOf = id => info[id].k === 'loop_group' ? (info[id].n.loop_group.nodes ?? [])[0]?.id ?? id : id;
  const lastOf = id => {
    if (info[id].k !== 'loop_group') return id;
    const body = info[id].n.loop_group.nodes ?? [];
    const used = new Set(body.flatMap(b => b.depends_on ?? []));
    return body.filter(b => !used.has(b.id)).at(-1)?.id ?? id;
  };
  edges.forEach((e, i) => {
    e.src = lastOf(e.from); e.dst = firstOf(e.to);
    g.setEdge(e.src, e.dst, { label: e.type === 'resume' ? 'resume' : '', width: 40, height: 14 }, String(i));
  });
  dagre.layout(g);
  return { g, info, edges };
}

function render(wf) {
  const { g, info, edges } = layout(wf);
  const gr = g.graph();
  let svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${gr.width + 40}" height="${gr.height + 40}"><defs><marker id="a" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0,0L10,5L0,10z" fill="#555"/></marker></defs>`;
  for (const [id, { n, k }] of Object.entries(info)) {
    if (k !== 'loop_group') continue;
    const p = g.node(id), lg = n.loop_group;
    svg += `<g class="nd" data-id="${esc(id)}"><rect x="${p.x - p.width / 2}" y="${p.y - p.height / 2}" width="${p.width}" height="${p.height}" fill="#fff7ed" stroke="#f97316" stroke-dasharray="6 3" rx="8"/>`;
    svg += `<text x="${p.x - p.width / 2 + 8}" y="${p.y - p.height / 2 - 22}" font-weight="bold" fill="#c2410c">⟳ ${esc(id)} (max ${lg.max_iterations ?? '?'})</text>`;
    svg += `<text x="${p.x - p.width / 2 + 8}" y="${p.y - p.height / 2 - 8}" font-size="10" fill="#c2410c">until: ${esc(trunc(lg.until_bash ?? lg.until ?? '', 60))}</text></g>`;
  }
  edges.forEach((e, i) => {
    const ed = g.edge(e.src, e.dst, String(i));
    const pts = ed.points.map(p => `${p.x},${p.y}`).join(' ');
    const dash = e.type === 'resume' ? ' stroke-dasharray="5 4" stroke="#7c3aed"' : ' stroke="#555"';
    svg += `<polyline points="${pts}" fill="none"${dash} marker-end="url(#a)"/>`;
    const tgt = info[e.to].n;
    const lbl = e.type === 'resume' ? 'resume' : '';
    if (lbl && ed.x !== undefined) svg += `<text x="${ed.x}" y="${ed.y}" font-size="10" fill="${e.type === 'resume' ? '#7c3aed' : '#b45309'}" text-anchor="middle">${esc(trunc(lbl, 40))}</text>`;
  });
  for (const [id, { n, k }] of Object.entries(info)) {
    if (k === 'loop_group') continue;
    const p = g.node(id), h = p.height, x = p.x - W / 2, y = p.y - h / 2, grp = group(n, k), t = parts(n, k);
    const link = k === 'workflow' ? ` data-link="${esc(n.workflow)}"` : '';
    svg += `<g class="nd" data-id="${esc(id)}"${link}><rect x="${x}" y="${y}" width="${W}" height="${h}" rx="6" fill="${COLOR[grp]}" stroke="${grp === 'human' ? '#dc2626' : '#666'}" stroke-width="${grp === 'human' ? 2 : 1}"/>`;
    svg += `<foreignObject x="${x}" y="${y}" width="${W}" height="${h}"><div xmlns="http://www.w3.org/1999/xhtml" class="box">`
      + `<div class="ti">${esc(t.title)}</div>`
      + (t.desc ? `<div class="de">${esc(t.desc)}</div>` : '')
      + (t.main ? `<div class="mv">${esc(t.main)}</div>` : '')
      + (t.when ? `<div class="wh">${esc(t.when)}</div>` : '')
      + (t.badges ? `<div class="bd">${esc(t.badges)}</div>` : '')
      + `</div></foreignObject>`;
    svg += `</g>`;
  }
  const yamlById = Object.fromEntries(Object.entries(info).map(([id, { n }]) => [id, YAML.stringify(n)]));
  return { svg: svg + '</svg>', yamlById };
}

const files = fs.readdirSync(dir).filter(f => /\.ya?ml$/.test(f)).sort();
const wfs = files.map(f => YAML.parse(fs.readFileSync(path.join(dir, f), 'utf8')));
const data = Object.fromEntries(wfs.map(wf => [wf.name, { desc: wf.description ?? '', meta: { returns: wf.returns, worktree: wf.worktree }, ...render(wf) }]));
const legend = Object.entries({ 'AI(prompt/command)': COLOR.ai, '機械の検査(bash/script)': COLOR.mech, '人(wait / to: human)': COLOR.human, '子ワークフロー': COLOR.child }).map(([k, c]) => `<span style="background:${c};padding:2px 6px;margin-right:6px;border:1px solid #999">${k}</span>`).join('') + '<span style="color:#7c3aed">- - resume(セッション引き継ぎ)</span> <span style="color:#f97316">⟳ loop_group(繰り返し)</span>';
const html = `<!doctype html><meta charset="utf-8"><title>Workflow viz</title>
<style>body{margin:0;font:13px sans-serif;display:flex;height:100vh}#nav{width:160px;border-right:1px solid #ccc;padding:8px;overflow:auto}#nav a{display:block;padding:4px;cursor:pointer}#nav a.on{background:#eee;font-weight:bold}#main{flex:1;overflow:auto;padding:12px}#side{width:360px;border-left:1px solid #ccc;padding:8px;overflow:auto;white-space:pre-wrap;font:12px monospace}.nd{cursor:pointer}.box{padding:8px;font:12px/16px sans-serif;word-break:break-all;overflow:hidden;height:100%;box-sizing:border-box}.ti{font-weight:bold}.de{color:#111}.mv{font:11px/16px monospace;color:#374151}.wh{font:11px/16px monospace;color:#b45309}.bd{font-size:10px;color:#6b7280}text{font-family:sans-serif;font-size:12px}</style>
<div id="nav"></div><div id="main"><div>${legend}</div><h2 id="t"></h2><p id="d"></p><div id="g"></div></div><div id="side">ノードをクリックすると YAML を表示</div>
<script>const D=${JSON.stringify(data)};
function show(n){const w=D[n];if(!w)return;document.querySelectorAll('#nav a').forEach(a=>a.className=a.textContent===n?'on':'');t.textContent=n;d.textContent=w.desc+(w.meta.returns?'  [returns: '+w.meta.returns+']':'')+(w.meta.worktree?'  [worktree: '+JSON.stringify(w.meta.worktree)+']':'');g.innerHTML=w.svg;
g.querySelectorAll('.nd').forEach(e=>e.onclick=ev=>{ev.stopPropagation();side.textContent=w.yamlById[e.dataset.id];});
g.querySelectorAll('[data-link]').forEach(e=>e.ondblclick=()=>show(e.dataset.link));location.hash=n;}
for(const n of Object.keys(D)){const a=document.createElement('a');a.textContent=n;a.onclick=()=>show(n);nav.appendChild(a);}
show(decodeURIComponent(location.hash.slice(1))||Object.keys(D)[0]);</script>`;
fs.mkdirSync('out', { recursive: true });
fs.writeFileSync('out/index.html', html);
console.log(`wrote out/index.html (${wfs.length} workflows)`);
