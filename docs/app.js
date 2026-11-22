// The documentation's page. All it says comes from data.js, which `go test ./test -update` writes: the
// Markdown files of the repository, and the tables of the code (`rayo help json`).
'use strict'

const { markdown, rayo } = window.RAYO
const $ = s => document.querySelector(s)
const escape = t => String(t).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
const slug = t => t.toLowerCase().replace(/[^\p{L}\p{N}_\- ]/gu, '').trim().replace(/ /g, '-')

// --- Markdown: what the files of the repository use of it ---

function inline(text, page) {
  const kept = []
  const keep = html => `\u0000${kept.push(html) - 1}\u0000`
  text = text.replace(/`([^`]+)`/g, (_, c) => keep(`<code>${escape(c)}</code>`))
  text = escape(text)
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, label, to) => keep(`<a href="${link(to, page)}">`) + label + keep('</a>'))
    .replace(/\*\*([^*]+)\*\*/g, '<b>$1</b>')
    .replace(/(^|[\s(])\*([^*\s][^*]*)\*/g, '$1<i>$2</i>')
  return text.replace(/\u0000(\d+)\u0000/g, (_, i) => kept[i])
}

// link turns one of a file into one of this page.
function link(to, page) {
  if (/^https?:/.test(to)) return to
  const [file, anchor] = to.split('#')
  const from = file || page
  if (anchor) {
    const found = topics.find(t => t.file === from && t.anchors.includes(anchor))
    if (found) return found.slug === anchor ? `#/${found.path}` : `#/${found.path}/${anchor}`
  }
  const first = topics.find(t => t.file === from)
  return first ? `#/${first.path}` : `https://github.com/nanihadesuka/rayo/blob/main/${file}`
}

function highlight(code, language) {
  if (language === 'yaml') {
    return code.split('\n').map(line => {
      const at = line.search(/(^|\s)#/)
      const comment = at >= 0 ? line.slice(at) : ''
      return escape(at >= 0 ? line.slice(0, at) : line)
        .replace(/(&quot;.*?&quot;)/g, '<span class="s">$1</span>')
        .replace(/(^|[\s{,-])([\w.]+)(?=:(\s|$))/g, '$1<span class="k">$2</span>')
        .replace(/(:\s)(true|false|-?\d[\w.]*)(?=[\s,}\]]|$)/g, '$1<span class="n">$2</span>')
        + (comment && `<span class="c">${escape(comment)}</span>`)
    }).join('\n')
  }
  if (language === 'json') return escape(code).replace(/(&quot;[^&]*?&quot;)(\s*:)/g, '<span class="k">$1</span>$2')
  return escape(code).replace(/(^|\s)(#.*)$/gm, '$1<span class="c">$2</span>')
}

const codeBlock = (code, language) =>
  `<div class="code"><pre tabindex="0"><code>${highlight(code, language)}</code></pre><button type="button">Copy</button></div>`

// languageOf guesses, as the files don't always say.
const languageOf = (code, said) => said || (/^\s*[{[]\s*$|^\{/.test(code) ? 'json' : '')

function render(text, page) {
  const lines = text.split('\n')
  let html = '', i = 0
  const paragraph = []
  const flush = () => { if (paragraph.length) html += `<p>${inline(paragraph.join(' '), page)}</p>`; paragraph.length = 0 }
  while (i < lines.length) {
    const line = lines[i]
    let m
    if ((m = line.match(/^(\s*)```(\w*)/))) {
      flush()
      const code = []
      for (i++; i < lines.length && !lines[i].trim().startsWith('```'); i++) code.push(lines[i].slice(m[1].length))
      html += codeBlock(code.join('\n'), languageOf(code.join('\n'), m[2]))
    } else if ((m = line.match(/^(#{2,5}) (.+)/))) {
      flush()
      const level = Math.min(m[1].length, 3)
      html += `<h${level} id="${slug(m[2])}">${inline(m[2], page)}<a class="here" href="#" data-anchor="${slug(m[2])}" aria-label="Link to this">#</a></h${level}>`
    } else if (/^\|/.test(line)) {
      flush()
      const rows = []
      for (; i < lines.length && /^\|/.test(lines[i]); i++) rows.push(lines[i].split(/(?<!\\)\|/).slice(1, -1).map(c => c.trim().replace(/\\\|/g, '|')))
      i--
      const [head, , ...body] = rows
      html += '<div class="table" tabindex="0"><table>' + (head.some(Boolean) ? `<tr>${head.map(c => `<th>${inline(c, page)}</th>`).join('')}</tr>` : '') +
        body.map(r => `<tr>${r.map(c => `<td>${inline(c, page)}</td>`).join('')}</tr>`).join('') + '</table></div>'
    } else if (/^(- |\d+\. )/.test(line)) {
      flush()
      const ordered = /^\d/.test(line)
      const items = []
      for (; i < lines.length; i++) {
        if (/^(- |\d+\. )/.test(lines[i])) items.push([lines[i].replace(/^(- |\d+\. )/, '')])
        else if (/^ {2,}\S/.test(lines[i]) || (lines[i] === '' && /^ {2,}\S/.test(lines[i + 1] || ''))) items.at(-1).push(lines[i].replace(/^ {2,3}/, ''))
        else break
      }
      i--
      html += `<${ordered ? 'ol' : 'ul'}>${items.map(t => `<li>${t.length > 1 && t.some(l => l.startsWith('```')) ? render(t.join('\n'), page) : inline(t.join(' '), page)}</li>`).join('')}</${ordered ? 'ol' : 'ul'}>`
    } else if (line === '---') { flush(); html += '<hr>' }
    else if (line.trim() === '' || /^<!--.*-->$/.test(line)) flush()
    else paragraph.push(line.trim())
    i++
  }
  flush()
  return html
}

// --- Topics: each is a page here ---

const commands = Object.fromEntries(rayo.flow.commands.map(c => [c.name, c]))
const commandOf = title => title.split(', ').map(t => t.replace(/ step$/, '')).filter(t => commands[t] || t.startsWith('inputRandom'))
const topics = []

// sections splits a file by its headings of some level.
function sections(text, level) {
  const list = []
  let code = false, now = { title: '', lines: [] }
  for (const line of text.split('\n')) {
    if (line.startsWith('```')) code = !code
    const m = !code && line.match(new RegExp(`^#{1,${level}} (.+)`))
    if (m) { list.push(now); now = { title: m[1], depth: line.indexOf(' '), lines: [] } }
    else now.lines.push(line)
  }
  list.push(now)
  return list.filter(s => s.title)
}

function topic(group, title, file, body) {
  const anchors = [slug(title)]
  for (const m of body.matchAll(/^#{2,5} (.+)$/gm)) anchors.push(slug(m[1]))
  topics.push({ group, title, file, body: body.trim(), slug: slug(title), path: '', anchors, commands: file === 'REFERENCE.md' ? commandOf(title) : [] })
}

function build() {
  let group = 'Start'
  for (const s of sections(markdown['GUIDE.md'], 2)) {
    if (s.depth === 1) { group = s.title === 'Guide' ? 'Start' : 'How do I…'; continue }
    topic(group, s.title, 'GUIDE.md', s.lines.join('\n'))
  }
  topic('Mocking', 'Mocking', 'MOCKING.md', markdown['MOCKING.md'].replace(/^# .*\n/, ''))
  const skip = ['Index']
  let intro = null
  for (const s of sections(markdown['REFERENCE.md'], 3)) {
    if (s.depth === 1 || skip.includes(s.title)) continue
    const body = s.lines.join('\n').replace(/\n---\n*$/, '\n').trim()
    if (s.depth === 2) {
      group = s.title
      intro = null
      if (body) { topic(group, s.title, 'REFERENCE.md', body); intro = topics.at(-1) }
      else topics.push({ group, title: s.title, file: 'REFERENCE.md', body: '', slug: slug(s.title), path: '', anchors: [slug(s.title)], commands: [], list: true })
    } else topic(group, s.title, 'REFERENCE.md', body)
  }
  topic('Working on rayo', 'Working on rayo', 'CONTRIBUTING.md', markdown['CONTRIBUTING.md'].replace(/^# .*\n/, ''))
  const used = {}
  for (const t of topics) {
    let path = slug(t.group) === t.slug ? t.slug : `${slug(t.group)}/${t.slug}`
    if (used[path]) path += '-' + ++used[path]
    used[path] = 1
    t.path = path
  }
}

// --- What a page shows ---

const keyRows = (keys, prefix) => keys.map(k =>
  `<tr id="${prefix}${k.name}"><td><code>${escape(k.name)}</code>${k.short ? ' <span class="badge">short form</span>' : ''}</td>` +
  `<td>${k.values && k.values.length ? k.values.map(v => `<code>${escape(v)}</code>`).join(' ') : escape(k.takes)}</td><td>${inline(k.about, 'REFERENCE.md')}</td></tr>`).join('')
const keyTable = (keys, prefix) => `<div class="table" tabindex="0"><table><tr><th>Key</th><th>Takes</th><th></th></tr>${keyRows(keys, prefix)}</table></div>`
const more = (title, html, open) => `<details class="more"${open ? ' open' : ''}><summary>${title}</summary>${html}</details>`

// skeleton is the whole form of a command, to copy and cut down.
function skeleton(c) {
  const all = [...(c.element ? rayo.flow.selector.slice(0, 3) : []), ...c.keys.filter(k => !(c.element && rayo.flow.selector.slice(0, 3).some(s => s.name === k.name))), ...rayo.flow.options]
  const value = k => k.values && k.values.length ? k.values[0] : ({ number: 1, time: '10s', point: '"50%, 50%"', 'true/false': true, selector: '{ id: an_id }', selectors: '[{ id: an_id }]', conditions: '{ visible: { id: an_id } }', commands: '[ back ]', values: '{ name: value }', permissions: '{ all: allow }', locations: '["41.4,2.2"]', JavaScript: '"${true}"', pattern: '".*"' }[k.takes] ?? `a_${k.takes.replace(/\W+/g, '_')}`)
  const width = Math.max(...all.map(k => `${k.name}: ${value(k)}`.length)) + 2
  return `- ${c.name}:\n` + all.map(k => `    ${`${k.name}: ${value(k)}`.padEnd(width)}# ${k.about}`).join('\n')
}

function commandPage(t) {
  const list = t.commands.flatMap(n => commands[n] ? [n] : Object.keys(commands).filter(c => c.startsWith('inputRandom')))
  const details = t.body.replace(/<!-- generated: keys[^>]*-->[\s\S]*?<!-- end -->/g, '').replace(/^🧪 ?/, '').trim()
  let html = ''
  for (const name of list) {
    const c = commands[name]
    if (list.length > 1) html += `<h2 id="${slug(name)}">${name}${c.tested ? '' : ' <span class="badge warn">not tested on a device</span>'}</h2>`
    html += `<p class="${list.length > 1 ? '' : 'lead'}">${escape(c.about)}.</p>` + codeBlock(c.example, 'yaml')
    if (c.alone) html += '<p>Works without a value.</p>'
    if (c.keys.length) html += (list.length > 1 ? '' : '<h2 id="keys">Keys</h2>') + keyTable(c.keys, list.length > 1 ? slug(name) + '-' : 'key-')
    if (c.element) html += more('Which element: selector keys', keyTable(rayo.flow.selector, 'selector-') + `<p><a href="${link('#selectors', 'REFERENCE.md')}">All about selectors</a></p>`)
    html += more('Options of every command', keyTable(rayo.flow.options, 'option-'))
    html += more('Every key, as YAML', codeBlock(skeleton(c), 'yaml') + '<p>Not all go together: copy what you need. <code>rayo check</code> says what is wrong.</p>')
  }
  if (details) html += '<h2 id="details">Details</h2>' + render(details, t.file)
  return html
}

const related = t => {
  const mine = new Set([...t.body.matchAll(/\]\(([^)]*#[^)]+)\)/g)].map(m => link(m[1], t.file)))
  const same = topics.filter(o => o.group === t.group && o !== t && o.commands.length).slice(0, 8)
  const list = [...topics.filter(o => mine.has(`#/${o.path}`)), ...(t.commands.length ? same : [])]
  return list.length ? `<h2 id="see-also">See also</h2><ul class="listing">${[...new Set(list)].map(o => `<li><a href="#/${o.path}">${escape(o.title)}</a></li>`).join('')}</ul>` : ''
}

const groupTitles = () => [...new Set(topics.map(t => t.group))]
const aboutOf = t => t.commands.length ? commands[t.commands[0]]?.about ?? 'Adds something random'
  : t.body.replace(/<!--[\s\S]*?-->|```[\s\S]*?```|^\|.*$/gm, '').replace(/\[([^\]]+)\]\([^)]+\)/g, '$1').replace(/[`*🧪]/gu, '').trim().split('\n\n')[0].replace(/\n/g, ' ').replace(/^#+ .*/, '')

function home() {
  const card = (path, title, text) => `<a class="card" href="#/${path}"><b>${title}</b><span>${text}</span></a>`
  const of = (file, title) => topics.find(t => t.file === file && t.title === title).path
  return `<div class="hero"><img src="icon.svg" width="120" height="120" alt=""><h1>rayo</h1><p>End-to-end tests for Android, fast as lightning.<br>Android only. One portable executable. Mocking, capture and videos built in.</p></div>` +
    codeBlock('appId: com.example.app\nmock: mock\n---\n- launchApp: { clearState: true, arguments: { mock: true } }\n- tapOn: { id: login }\n- inputText: me@example.com\n- extendedWaitUntil: { visible: { id: home }, timeout: 20s }\n- waitForRequest: { method: POST, path: ".*/session" }', 'yaml') +
    `<div class="cards">${[
      card(of('GUIDE.md', 'Install'), 'Start', 'Install it and write your first test'),
      card('commands', 'Commands', `All ${rayo.flow.commands.length}, with examples and keys`),
      card(of('REFERENCE.md', 'Suites'), 'Suites', 'Run many tests, with retries and videos'),
      card(of('MOCKING.md', 'Mocking'), 'Mocking', 'Answer as the backend'),
      card(of('REFERENCE.md', 'Results'), 'Results', 'What a failure tells, as text, JSON or JUnit'),
      card(of('REFERENCE.md', 'Live sessions'), 'Live', 'Write a test while it runs, also from another program'),
      card(of('GUIDE.md', 'Log in before each test'), 'How do I…', 'Answers to copy'),
      card(of('REFERENCE.md', 'Options'), 'Options', 'Of the command line'),
    ].join('')}</div>`
}

function commandsPage() {
  const groups = groupTitles().map(g => [g, topics.filter(t => t.group === g && t.commands.length)]).filter(([, l]) => l.length)
  return `<h1>Commands</h1><p class="lead">Everything a flow can do. Each line of a flow is one of them.</p>
    <input class="filter" type="search" placeholder="Filter" aria-label="Filter the commands">` +
    groups.map(([g, list]) => `<section><h2 id="${slug(g)}">${escape(g)}</h2><div class="table"><table>` +
      list.map(t => `<tr data-text="${escape((t.title + ' ' + aboutOf(t)).toLowerCase())}"><td><a href="#/${t.path}"><code>${escape(t.title)}</code></a></td><td>${escape(aboutOf(t))}</td></tr>`).join('') + '</table></div></section>').join('')
}

function topicPage(t) {
  const untested = t.commands.length === 1 && commands[t.commands[0]] && !commands[t.commands[0]].tested
  let html = `<div class="crumbs">${escape(t.group)}</div><h1 id="${t.slug}">${escape(t.title)}${untested ? ' <span class="badge warn">not tested on a device</span>' : ''}</h1>`
  if (t.list) html += `<div class="table"><table>${topics.filter(o => o.group === t.group && o !== t).map(o => `<tr><td><a href="#/${o.path}">${escape(o.title)}</a></td><td>${escape(aboutOf(o))}</td></tr>`).join('')}</table></div>`
  else if (t.commands.length) html += commandPage(t)
  else html += render(t.body, t.file)
  for (const [title, file] of [['Suites', 'suite'], ['Header', 'flow'], ['Results', 'results']]) {
    if (t.title === title && t.file === 'REFERENCE.md') html += more(`JSON Schema: <code>${file}.json</code>`, `<p><code>rayo schema ${file}</code> prints it, and it is in the <a href="https://github.com/nanihadesuka/rayo/tree/main/schema">repository</a>.</p>` + (markdown[`schema/${file}.json`] ? codeBlock(markdown[`schema/${file}.json`], 'json') : ''))
  }
  html += related(t)
  const shown = topics.filter(o => !o.list)
  const at = shown.indexOf(t)
  if (at >= 0) {
    const way = (o, text) => o ? `<a href="#/${o.path}">${text(escape(o.title))}</a>` : '<span></span>'
    html += `<div class="next">${way(shown[at - 1], x => '← ' + x)}${way(shown[at + 1], x => x + ' →')}</div>`
  }
  return html
}

// --- The page itself ---

function side() {
  let html = `<ul style="border:0;margin-left:0;padding:0"><li><a href="#/commands" data-path="commands"><b>All commands</b></a></li></ul>`
  for (const g of groupTitles()) {
    const list = topics.filter(t => t.group === g)
    if (list.length === 1) { html += `<ul style="border:0;margin-left:0;padding:0"><li><a href="#/${list[0].path}" data-path="${list[0].path}">${escape(g)}</a></li></ul>`; continue }
    html += `<details data-group="${escape(g)}"><summary>${escape(g)}</summary><ul>${list.map(t => `<li><a href="#/${t.path}" data-path="${t.path}">${escape(t.list || t.title === g ? 'Overview' : t.title)}</a></li>`).join('')}</ul></details>`
  }
  $('#side').innerHTML = html
}

function show() {
  let path
  try { path = decodeURIComponent(location.hash.replace(/^#\/?/, '')) } catch { path = '' }
  let anchor = '', found = topics.find(t => t.path === path)
  if (!found && path && path !== 'commands') {
    const cut = path.lastIndexOf('/')
    found = topics.find(t => t.path === path.slice(0, cut))
    anchor = path.slice(cut + 1)
  }
  const page = $('#page')
  page.innerHTML = found ? topicPage(found) : path === 'commands' ? commandsPage()
    : path ? `<h1>Nothing here</h1><p>There is no page <code>${escape(path)}</code>. Try the search, or <a href="#/">begin again</a>.</p>` : home()
  document.title = found ? `${found.title} · rayo` : path === 'commands' ? 'Commands · rayo' : 'rayo'
  for (const a of document.querySelectorAll('#side a')) {
    const here = a.dataset.path === (found ? found.path : path)
    a.toggleAttribute('aria-current', here)
    if (here) { a.setAttribute('aria-current', 'page'); const d = a.closest('details'); if (d) d.open = true }
  }
  if (!found && !path) { const d = $('#side details'); if (d) d.open = true }
  const base = found ? found.path : path
  for (const a of page.querySelectorAll('a.here')) a.href = `#/${base}/${a.dataset.anchor}`
  const heads = [...page.querySelectorAll('h2[id], h3[id]')].filter(h => !h.closest('details'))
  $('#toc').innerHTML = heads.length > 1 ? '<b>On this page</b>' + heads.map(h =>
    `<a class="${h.tagName === 'H3' ? 'deep' : ''}" href="#/${base}/${h.id}">${escape(h.firstChild.textContent)}</a>`).join('') : ''
  closeMenu()
  const target = anchor && document.getElementById(anchor)
  if (target) {
    const d = target.closest('details')
    if (d) d.open = true
    target.scrollIntoView()
  } else window.scrollTo(0, 0)
}

function closeMenu() {
  $('#side').classList.remove('open')
  $('#shade').hidden = true
  $('#menu').setAttribute('aria-expanded', 'false')
}

// --- Search ---

let index = []
function buildIndex() {
  for (const t of topics) {
    if (t.list) continue
    index.push({ title: t.title, where: t.group, text: aboutOf(t), body: t.body.replace(/[`*|#]|<!--.*?-->/g, ' '), path: t.path, weight: t.commands.length ? 3 : 2 })
    for (const m of t.body.matchAll(/^#{2,5} (.+)$/gm)) index.push({ title: m[1], where: t.title, text: '', body: '', path: `${t.path}/${slug(m[1])}`, weight: 1.5 })
    if (t.commands.length === 1 && commands[t.commands[0]]) for (const k of commands[t.commands[0]].keys)
      index.push({ title: k.name, where: t.title, text: `${k.takes}. ${k.about}`, body: (k.values || []).join(' '), path: `${t.path}/key-${k.name}`, weight: 1 })
  }
  const of = title => topics.find(t => t.file === 'REFERENCE.md' && t.title === title)
  const keys = (list, title) => { const t = of(title); if (t) for (const k of list) index.push({ title: k.name, where: title, text: `${k.takes}. ${k.about}`, body: '', path: t.path, weight: 1 }) }
  keys(rayo.flow.selector, 'Conditions')
  keys(rayo.suite, 'Suites')
  keys(rayo.device, 'Device setup')
  keys(rayo.header, 'Header')
  for (const o of rayo.options) index.push({ title: `--${o.name}`, where: `rayo ${o.commands.join(', ')}`, text: o.about, body: o.short ? `-${o.short}` : '', path: of('Options').path, weight: 1.5 })
}

function find(query) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (!words.length) return []
  const scored = []
  for (const e of index) {
    const title = e.title.toLowerCase(), text = e.text.toLowerCase(), body = e.body.toLowerCase(), where = e.where.toLowerCase()
    let score = 0
    for (const w of words) {
      const s = title === w || title === '--' + w ? 100 : title.startsWith(w) || title.startsWith('--' + w) ? 60 : title.includes(w) ? 40 : where.includes(w) ? 18 : text.includes(w) ? 14 : body.includes(w) ? 5 : 0
      if (!s) { score = 0; break }
      score += s
    }
    if (score) scored.push([score + e.weight * 4 - e.title.length / 20, e])
  }
  const seen = new Set()
  return scored.sort((a, b) => b[0] - a[0]).map(s => s[1]).filter(e => !seen.has(e.path + e.title) && seen.add(e.path + e.title)).slice(0, 14)
}

function marked(text, query) {
  const words = query.split(/\s+/).filter(Boolean).map(w => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
  return escape(text).replace(new RegExp(`(${words.map(escape).join('|')})(?![^&;]*;)`, 'gi'), '<mark>$1</mark>')
}

function setUpSearch() {
  const input = $('#search'), found = $('#found')
  let chosen = -1
  const close = () => { found.hidden = true; input.setAttribute('aria-expanded', 'false'); input.removeAttribute('aria-activedescendant'); chosen = -1 }
  const choose = n => {
    const items = [...found.querySelectorAll('li[role=option]')]
    if (!items.length) return
    chosen = (n + items.length) % items.length
    items.forEach((li, i) => li.setAttribute('aria-selected', i === chosen))
    input.setAttribute('aria-activedescendant', items[chosen].id)
    items[chosen].scrollIntoView({ block: 'nearest' })
  }
  const update = () => {
    const q = input.value.trim()
    if (!q) return close()
    const list = find(q)
    found.innerHTML = list.length ? list.map((e, i) => `<li role="option" id="found-${i}" aria-selected="false"><a href="#/${e.path}" tabindex="-1"><span class="where">${escape(e.where)}</span><b>${marked(e.title, q)}</b><small>${marked(e.text, q)}</small></a></li>`).join('')
      : `<li class="none">Nothing about “${escape(q)}”</li>`
    found.hidden = false
    input.setAttribute('aria-expanded', 'true')
    choose(0)
  }
  input.addEventListener('input', update)
  input.addEventListener('focus', update)
  input.addEventListener('keydown', e => {
    if (e.key === 'ArrowDown') { e.preventDefault(); choose(chosen + 1) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); choose(chosen - 1) }
    else if (e.key === 'Escape') { input.value = ''; close(); input.blur() }
    else if (e.key === 'Enter') { const a = found.querySelector('[aria-selected=true] a'); if (a) { location.hash = a.getAttribute('href'); close(); input.blur() } }
  })
  found.addEventListener('click', e => { if (e.target.closest('a')) { close(); input.blur() } })
  document.addEventListener('click', e => { if (!e.target.closest('.search')) close() })
  document.addEventListener('keydown', e => {
    const typing = /^(INPUT|TEXTAREA)$/.test(document.activeElement.tagName)
    if ((e.key === '/' && !typing) || (e.key === 'k' && (e.ctrlKey || e.metaKey))) { e.preventDefault(); input.focus(); input.select() }
  })
}

build()
buildIndex()
side()
setUpSearch()
$('#menu').addEventListener('click', () => {
  const open = $('#side').classList.toggle('open')
  $('#shade').hidden = !open
  $('#menu').setAttribute('aria-expanded', open)
})
$('#shade').addEventListener('click', closeMenu)
$('#theme').addEventListener('click', () => {
  const root = document.documentElement
  const dark = root.dataset.theme ? root.dataset.theme === 'dark' : matchMedia('(prefers-color-scheme: dark)').matches
  root.dataset.theme = dark ? 'light' : 'dark'
  try { localStorage.setItem('theme', root.dataset.theme) } catch {}
})
$('#page').addEventListener('click', async e => {
  const button = e.target.closest('.code button')
  if (!button) return
  try { await navigator.clipboard.writeText(button.parentElement.querySelector('code').textContent); button.textContent = 'Copied' } catch { button.textContent = 'Select it' }
  setTimeout(() => { button.textContent = 'Copy' }, 1500)
})
$('#page').addEventListener('input', e => {
  if (!e.target.matches('.filter')) return
  const q = e.target.value.toLowerCase()
  for (const row of document.querySelectorAll('tr[data-text]')) row.hidden = !row.dataset.text.includes(q)
  for (const s of document.querySelectorAll('#page section')) s.hidden = !s.querySelector('tr:not([hidden])')
})
addEventListener('hashchange', show)
show()
