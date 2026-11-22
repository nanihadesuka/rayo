// The documentation's page. All it says comes from data.js, which `go test ./test -update` writes: the
// Markdown files of the repository, the example, and the tables of the code (`rayo help json`).
'use strict'

const { markdown, rayo } = window.RAYO
const repository = 'https://github.com/nanihadesuka/rayo'
const $ = s => document.querySelector(s)
const escape = t => String(t).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
const slug = t => t.toLowerCase().replace(/[^\p{L}\p{N}_\- ]/gu, '').trim().replace(/ /g, '-')
const remember = (key, value) => { try { localStorage.setItem(key, JSON.stringify(value)) } catch {} }
const remembered = (key, otherwise) => { try { return JSON.parse(localStorage.getItem(key)) ?? otherwise } catch { return otherwise } }
const commands = Object.fromEntries(rayo.flow.commands.map(c => [c.name, c]))

// --- What goes where ---

// Each page is read by scrolling. Its sections are those of the files, by their titles; as: another title.
const R = (title, as) => ({ file: 'REFERENCE.md', title, as })
const pages = [
  { id: 'start', title: 'Get started', about: 'From nothing to a suite of tests, in seven steps.', all: 'GUIDE.md', part: 'Guide', numbered: true },
  { id: 'example', title: 'Full example', about: 'A whole project: tests, mocks, a suite, and what comes out.', all: 'EXAMPLE.md' },
  { id: 'recipes', title: 'Recipes', about: 'How do I… Answers to copy.', all: 'GUIDE.md', part: 'How do I...', folded: true },
  { id: 'commands', title: 'Commands', about: 'Everything a flow can do. Each line of a flow is one of them.' },
  { id: 'flows', title: 'Writing flows', about: 'The file, how to find elements, variables and scripts.', sections: [
    R('Flow file', 'The file'), R('Header', 'The header'), R('Common options', 'Options of every command'), R('Values'), R('Selectors', 'Finding elements'),
    R('Conditions', 'Selector keys'), R('Relations'), R('when', 'Conditions: when and while'), R('Variables'), R('Expressions', 'Expressions: ${…}'), R('JavaScript API'), R('Tags')] },
  { id: 'mocking', title: 'Mocking', about: 'Answer as the backend, or capture what the real one says.', all: 'MOCKING.md',
    sections: [R('Mock', 'In a flow'), R('Mock files', 'What a mapping can say'), R('Capture', 'Capture the real backend')] },
  { id: 'suites', title: 'Suites and devices', about: 'Many tests together: retries, videos, and how the device is.', sections: [
    R('Suites', 'A suite'), R('A flow of a suite'), R('The app of a suite'), R('What goes where'), R('Retries'), R('Recording a whole flow', 'Videos'), R('Device setup'), R('Clean state')] },
  { id: 'results', title: 'Results', about: 'How a run went: on the screen, as text, JSON or JUnit.', sections: [
    R('Results', 'What comes out'), R('Outcomes'), R('Causes', 'Causes, and when it stops'), R('Failures', 'What is saved of a failure'), R('Report', 'JUnit')] },
  { id: 'live', title: 'Live sessions', about: 'Write a test while it runs, by hand or from another program.', sections: [
    R('Live sessions', 'A session'), R('Live commands', 'Commands'), R('From other programs')] },
  { id: 'cli', title: 'Command line', about: 'Every option, and what it needs around it.', sections: [
    R('Options'), R('Continuous mode'), R('Profile'), R('Environment'), R('Schemas', 'Schemas for editors')] },
  { id: 'help', title: 'Help', about: 'What it can’t do, how long it waits, and how it is made.', sections: [
    R('Limitations'), R('Default times'), { file: 'CONTRIBUTING.md', title: 'Working on rayo' }] },
]
const commandGroups = ['Apps', 'Touch', 'Text and keys', 'Checks and waits', 'Control', 'Scripts', 'Device', 'Server', 'Mock']

// sectionsOf splits a file by its headings.
function sectionsOf(file, deepest) {
  const list = []
  let code = false, now = { title: '', depth: 0, lines: [] }
  for (const line of markdown[file].split('\n')) {
    if (line.startsWith('```')) code = !code
    const m = !code && line.match(new RegExp(`^(#{1,${deepest}}) (.+)`))
    if (m) { list.push(now); now = { title: m[2], depth: m[1].length, lines: [] } }
    else now.lines.push(line)
  }
  list.push(now)
  return list.filter(s => s.title).map(s => ({ file, title: s.title, depth: s.depth, id: slug(s.title), body: s.lines.join('\n').replace(/\n---\s*$/, '').trim() }))
}

const reference = sectionsOf('REFERENCE.md', 4)
const isCommand = s => s.file === 'REFERENCE.md' && s.depth === 3 && s.title.split(', ').some(t => commands[t.replace(/ step$/, '')] || t === 'inputRandom…')
const namesIn = s => s.title.split(', ').flatMap(t => t === 'inputRandom…' ? Object.keys(commands).filter(c => c.startsWith('inputRandom')) : commands[t.replace(/ step$/, '')] ? [t.replace(/ step$/, '')] : [])

function build() {
  const placed = new Set()
  const pick = ({ file, title, as }) => {
    if (file === 'CONTRIBUTING.md') return { file, title, as: title, id: slug(title), body: markdown[file].replace(/^# .*\n/, '') }
    const s = reference.find(s => s.title === title && !isCommand(s))
    if (!s) { console.error('no section', title); return null }
    placed.add(s)
    return { ...s, as: as || s.title }
  }
  for (const p of pages) {
    p.list = []
    if (p.all) {
      let part = '', first = true
      for (const s of sectionsOf(p.all, 2)) {
        if (s.depth === 1) {
          part = s.title
          if (first && !p.part) { p.intro = s.body }
          first = false
        } else if (!p.part || part === p.part) p.list.push({ ...s, as: s.title })
      }
    }
    p.list.push(...(p.sections || []).map(pick).filter(Boolean))
  }
  const page = pages.find(p => p.id === 'commands')
  let group = ''
  for (const s of reference) {
    if (s.depth === 2) group = s.title
    if (!isCommand(s)) continue
    for (const name of namesIn(s)) page.list.push({ ...s, as: name, id: slug(name), command: commands[name], group })
    placed.add(s)
  }
  // Nothing is lost: what no page asks for goes to the last one.
  const skip = new Set(['Index', 'Running several', 'Features', ...commandGroups])
  for (const s of reference) if (s.depth > 1 && !placed.has(s) && s.body && !skip.has(s.title)) {
    console.error('not placed', s.title)
    pages.at(-1).list.push({ ...s, as: s.title })
  }
  for (const p of pages) for (const s of p.list) s.page = p
}

const every = () => pages.flatMap(p => p.list)

// place is where an anchor of a file is in this page.
function place(file, anchor) {
  for (const s of every()) {
    if (s.file !== file) continue
    if (s.id === anchor || slug(s.title) === anchor) return `#/${s.page.id}/${s.id}`
    if ([...s.body.matchAll(/^#{2,5} (.+)$/gm)].some(m => slug(m[1]) === anchor)) return `#/${s.page.id}/${anchor}`
  }
}

function link(to, file) {
  if (/^https?:/.test(to)) return to
  const [name, anchor] = to.split('#')
  const from = name || file
  if (anchor) {
    const at = place(from, anchor)
    if (at) return at
    if (from === 'REFERENCE.md' && commandGroups.some(g => slug(g) === anchor)) return `#/commands/group-${anchor}`
    if (from === 'REFERENCE.md') return anchor === 'running-several' ? '#/suites' : '#/flows'
  }
  const whole = pages.find(p => p.all === from && !p.part) || (from === 'GUIDE.md' && pages[0]) || (from === 'REFERENCE.md' && pages.find(p => p.id === 'commands'))
  return whole ? `#/${whole.id}` : from === 'README.md' ? '#/' : `${repository}/tree/main/${from}`
}

// --- Markdown: what the files of the repository use of it ---

function inline(text, file) {
  const kept = []
  const keep = html => `\u0000${kept.push(html) - 1}\u0000`
  text = text.replace(/`([^`]+)`/g, (_, c) => keep(`<code>${escape(c)}</code>`))
  text = escape(text)
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, label, to) => keep(`<a href="${link(to, file)}">`) + label + keep('</a>'))
    .replace(/\*\*([^*]+)\*\*/g, '<b>$1</b>')
    .replace(/(^|[\s(])\*([^*\s][^*]*)\*/g, '$1<i>$2</i>')
  return text.replace(/\u0000(\d+)\u0000/g, (_, i) => kept[i])
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

const codeBlock = (code, language, name) =>
  `<div class="code">${name ? `<div class="name">${escape(name)}</div>` : ''}<pre tabindex="0"><code>${highlight(code, language)}</code></pre><button type="button">Copy</button></div>`
const languageOf = (code, said) => said || (/^\{/.test(code) ? 'json' : '')
const more = (title, html, open) => `<details class="more"${open ? ' open' : ''}><summary>${title}</summary>${html}</details>`
const tableTitles = { results: 'Every key of the run', 'results flow': 'Every key of a flow', 'results failure': 'Every key of a failure' }

function render(text, file) {
  const lines = text.split('\n')
  let html = '', i = 0, fold = ''
  const paragraph = []
  const flush = () => { if (paragraph.length) html += `<p>${inline(paragraph.join(' '), file)}</p>`; paragraph.length = 0 }
  for (; i < lines.length; i++) {
    const line = lines[i]
    let m
    if ((m = line.match(/^(\s*)```(\w*)/))) {
      flush()
      const code = []
      for (i++; i < lines.length && !lines[i].trim().startsWith('```'); i++) code.push(lines[i].slice(m[1].length))
      const text = code.join('\n'), block = codeBlock(text, languageOf(text, m[2]))
      html += code.length > 34 && file === 'REFERENCE.md' ? more('Show it whole', block) : block
    } else if ((m = line.match(/^<!-- file: (.+?) -->$/))) {
      flush()
      html += codeBlock(markdown[m[1]].trimEnd(), m[1].endsWith('.json') ? 'json' : 'yaml', m[1].replace('examples/shop/', ''))
    } else if ((m = line.match(/^<!-- generated: (.+?) -->$/))) { flush(); fold = tableTitles[m[1]] || '' }
    else if ((m = line.match(/^(#{2,5}) (.+)/))) {
      flush()
      html += `<h3 id="${slug(m[2])}">${inline(m[2], file)}</h3>`
    } else if (/^\|/.test(line)) {
      flush()
      const rows = []
      for (; i < lines.length && /^\|/.test(lines[i]); i++) rows.push(lines[i].split(/(?<!\\)\|/).slice(1, -1).map(c => c.trim().replace(/\\\|/g, '|')))
      i--
      const [head, , ...body] = rows
      const table = '<div class="table" tabindex="0"><table>' + (head.some(Boolean) ? `<tr>${head.map(c => `<th>${inline(c, file)}</th>`).join('')}</tr>` : '') +
        body.map(r => `<tr>${r.map(c => `<td>${inline(c, file)}</td>`).join('')}</tr>`).join('') + '</table></div>'
      html += fold ? more(`${fold} (${body.length})`, table) : table
      fold = ''
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
      html += `<${ordered ? 'ol' : 'ul'}>${items.map(t => `<li>${t.some(l => l.startsWith('```')) ? render(t.join('\n'), file) : inline(t.join(' '), file)}</li>`).join('')}</${ordered ? 'ol' : 'ul'}>`
    } else if (line === '---') { flush(); html += '<hr>' }
    else if (line.trim() === '' || /^<!--.*-->$/.test(line)) flush()
    else paragraph.push(line.trim())
  }
  flush()
  return html
}

// --- Pages ---

const keyTable = (keys, prefix) => `<div class="table" tabindex="0"><table><tr><th>Key</th><th>Takes</th><th></th></tr>${keys.map(k =>
  `<tr${prefix ? ` id="${prefix}${k.name.toLowerCase()}"` : ''}><td><code>${escape(k.name)}</code>${k.short ? ' <span class="badge">short form</span>' : ''}</td>` +
  `<td>${k.values && k.values.length ? k.values.map(v => `<code>${escape(v)}</code>`).join(' ') : escape(k.takes)}</td><td>${inline(k.about, 'REFERENCE.md')}</td></tr>`).join('')}</table></div>`

// skeleton is the whole form of a command, to copy and cut down.
function skeleton(c) {
  const some = c.element ? rayo.flow.selector.slice(0, 3) : []
  const all = [...some, ...c.keys.filter(k => !some.some(s => s.name === k.name)), ...rayo.flow.options]
  const value = k => k.values && k.values.length ? k.values[0] : ({ number: 1, time: '10s', point: '"50%, 50%"', selector: '{ id: an_id }', selectors: '[{ id: an_id }]', conditions: '{ visible: { id: an_id } }', commands: '[ back ]', values: '{ name: value }', permissions: '{ all: allow }', locations: '["41.4,2.2"]', JavaScript: '"${true}"', pattern: '".*"', '400 to 599': 503 }[k.takes] ?? `a_${k.takes.replace(/\W+/g, '_')}`)
  const width = Math.max(...all.map(k => `${k.name}: ${value(k)}`.length)) + 2
  return `- ${c.name}:\n` + all.map(k => `    ${`${k.name}: ${value(k)}`.padEnd(width)}# ${k.about}`).join('\n')
}

// detailsOf is what the reference says of a command besides its keys and examples: its first words are
// the introduction.
function detailsOf(s) {
  const body = s.body.replace(/<!-- generated: keys[^>]*-->[\s\S]*?<!-- end -->/g, '').replace(/```yaml\n[\s\S]*?```\n?/g, '').replace(/^🧪 ?/, '').trim()
  const [first, ...rest] = body.split('\n\n')
  const prose = first && !/^[|<#-]/.test(first) && s.title.split(', ').length === 1 && s.title !== 'inputRandom…'
  return { intro: prose ? inline(first.replace(/\n/g, ' '), s.file) : escape(s.command.about) + '.', rest: (prose ? rest : s.title === 'inputRandom…' ? [] : [first, ...rest].filter(Boolean)).join('\n\n').trim() }
}

function commandEntry(s) {
  const c = s.command, { intro, rest } = detailsOf(s)
  return `<details class="command" id="${s.id}" data-text="${escape(`${c.name} ${c.about} ${c.keys.map(k => k.name).join(' ')}`.toLowerCase())}">` +
    `<summary><code>${c.name}</code><span>${escape(c.about)}</span>${c.tested ? '' : '<i class="badge warn" title="Not tested on a device yet">untested</i>'}</summary><div>` +
    `<p>${intro}${c.alone ? ' Works without a value.' : ''}${c.element ? ' Takes the <a href="#/commands/selector">selector keys</a>.' : ''}</p>` + codeBlock(c.example, 'yaml') +
    (c.keys.length ? keyTable(c.keys, `${s.id}-`) : '') + (rest ? render(rest, s.file) : '') +
    more('Every key, as YAML', codeBlock(skeleton(c), 'yaml') + '<p>Not all go together: copy what you need. <code>rayo check</code> says what is wrong.</p>') + '</div></details>'
}

const filter = label => `<input class="filter" type="search" placeholder="Filter" aria-label="${label}">`

function commandsPage(p) {
  const intro = g => reference.find(s => s.depth === 2 && s.title === g && g === 'Checks and waits')
  return filter('Filter the commands') +
    `<div id="selector">${more('Selector keys: for the commands that take an element', keyTable(rayo.flow.selector) + `<p><a href="#/flows/selectors">All about finding elements</a></p>`)}</div>` +
    `<div id="common-keys">${more('Options of every command', keyTable(rayo.flow.options))}</div>` +
    commandGroups.map(g => [g, p.list.filter(s => s.group === g)]).filter(([, l]) => l.length).map(([g, list]) =>
      `<section class="group" id="group-${slug(g)}"><h2>${escape(g)}</h2>${intro(g) ? render(intro(g).body, 'REFERENCE.md') : ''}${list.map(commandEntry).join('')}</section>`).join('')
}

function sectionHtml(s, n) {
  const p = s.page
  if (p.folded) return `<details class="command recipe" id="${s.id}" data-text="${escape((s.as + ' ' + s.body).toLowerCase())}"><summary><span>${escape(s.as)}</span></summary><div>${render(s.body, s.file)}</div></details>`
  let html = `<section id="${s.id}"><h2>${p.numbered ? `<span class="number">${n + 1}</span>` : ''}${escape(s.as)}<a class="here" href="#/${p.id}/${s.id}" aria-label="Link to this">#</a></h2>${render(s.body, s.file)}`
  const schema = { Suites: 'suite', Header: 'flow', Results: 'results' }[s.title]
  if (schema && s.file === 'REFERENCE.md') html += more(`JSON Schema: <code>${schema}.json</code>`, `<p><code>rayo schema ${schema}</code> prints it. It is also in the <a href="${repository}/tree/main/schema">repository</a>.</p>` + (markdown[`schema/${schema}.json`] ? codeBlock(markdown[`schema/${schema}.json`], 'json') : ''))
  return html + '</section>'
}

function pageHtml(p) {
  const next = pages[pages.indexOf(p) + 1], before = pages[pages.indexOf(p) - 1]
  return `<h1>${escape(p.title)}</h1><p class="lead">${escape(p.about)}</p>` + (p.intro ? render(p.intro, p.all) : '') +
    (p.folded ? filter('Filter the recipes') : '') + (p.id === 'commands' ? commandsPage(p) : p.list.map(sectionHtml).join('')) +
    (next ? `<a class="onward" href="#/${next.id}"><small>Next</small><b>${escape(next.title)} →</b><span>${escape(next.about)}</span></a>` : '') +
    (before ? `<p class="back"><a href="#/${before.id}">← ${escape(before.title)}</a></p>` : '')
}

function home() {
  const feature = (to, title, text) => `<a class="card" href="${to}"><b>${title}</b><span>${text}</span></a>`
  const step = (n, title, html) => `<div class="step"><h3><span class="number">${n}</span>${title}</h3>${html}</div>`
  return `<div class="hero"><img src="icon.svg" width="120" height="120" alt=""><h1>rayo</h1>
    <p>End-to-end tests for Android, fast as lightning.<br>One portable executable. Nothing to install on the device.</p>
    <a class="button" href="#/start">Get started</a> <a class="button plain" href="${repository}/releases/latest">Download</a></div>` +
    '<div class="steps">' +
    step(1, 'Download', '<p>One file for Windows, macOS or Linux. It only needs adb.</p>' + codeBlock('rayo devices', '') + '<p>Then see what is on screen:</p>' + codeBlock('rayo hierarchy', '')) +
    step(2, 'Write', codeBlock('appId: com.example.app\n---\n- launchApp: { clearState: true }\n- tapOn: Log in\n- inputText: me@example.com\n- assertVisible: "Welcome.*"', 'yaml', 'login.yaml')) +
    step(3, 'Run', codeBlock('$ rayo test --emulator login.yaml\n  ✓ launchApp clearState  812 ms\n  ✓ tapOn Log in  95 ms\n  ✓ inputText me@example.com  41 ms\n  ✓ assertVisible Welcome.*  388 ms\n[1/1] PASS login  1.4 s', '')) +
    `</div><p class="center"><a href="#/example">See a full example: tests, mocks, a suite and its results →</a></p><div class="cards">${[
      feature('#/mocking', 'Mock backend', 'Built in: states, slow or broken networks. Or capture the real one'),
      feature(place('REFERENCE.md', 'setlocation'), 'Location and trips', 'Put the device anywhere, or move it along a route'),
      feature('#/suites', 'Suites', 'Many tests together, with retries and screen sizes'),
      feature('#/suites/recording-a-whole-flow', 'Videos', 'A small one of each test, or of some steps'),
      feature('#/results', 'Honest results', 'Why, where, the screen, the calls and the crash'),
      feature('#/live', 'Live', 'Write a test while it runs'),
    ].join('')}</div>`
}

// --- Moving around ---

let shown = null

function side() {
  const open = remembered('open', ['start'])
  $('#side').innerHTML = pages.map(p => {
    const links = p.id === 'commands' ? commandGroups.map(g => [`group-${slug(g)}`, g]) : p.folded ? [] : p.list.map(s => [s.id, s.as])
    if (!links.length) return `<div class="page"><a href="#/${p.id}" data-page="${p.id}">${escape(p.title)}</a></div>`
    return `<details class="page" data-id="${p.id}"${open.includes(p.id) ? ' open' : ''}><summary><a href="#/${p.id}" data-page="${p.id}">${escape(p.title)}</a></summary>` +
      `<ul>${links.map(([id, title]) => `<li><a href="#/${p.id}/${id}" data-section="${p.id}/${id}">${escape(title)}</a></li>`).join('')}</ul></details>`
  }).join('')
  for (const d of document.querySelectorAll('#side details')) d.addEventListener('toggle', () =>
    remember('open', [...document.querySelectorAll('#side details[open]')].map(d => d.dataset.id)))
}

// where is the page and the section of an address, also of one from before.
function where(hash) {
  let path
  try { path = decodeURIComponent(hash.replace(/^#\/?/, '')) } catch { path = '' }
  if (!path) return {}
  const [first, ...rest] = path.split('/')
  const p = pages.find(p => p.id === first)
  if (p) {
    const anchor = rest.join('/').toLowerCase()
    return { page: p, anchor }
  }
  const last = path.split('/').filter(x => !x.startsWith('key-')).at(-1).toLowerCase()
  const s = every().find(s => s.id === last || slug(s.title) === last)
  if (s) return { page: s.page, anchor: s.id, moved: true }
  if (commandGroups.some(g => slug(g) === last)) return { page: pages.find(p => p.id === 'commands'), anchor: `group-${last}`, moved: true }
  return { lost: path }
}

function show() {
  const at = where(location.hash)
  if (at.moved) return location.replace(`#/${at.page.id}${at.anchor ? '/' + at.anchor : ''}`)
  const key = at.page ? at.page.id : at.lost ? 'lost' : 'home'
  if (shown !== key) {
    $('#page').innerHTML = at.page ? pageHtml(at.page) : at.lost
      ? `<h1>Nothing here</h1><p>There is no page <code>${escape(at.lost)}</code>. Try the search, or <a href="#/">begin again</a>.</p>` : home()
    document.body.classList.toggle('front', key === 'home')
    document.title = at.page ? `${at.page.title} · rayo` : 'rayo'
    for (const a of document.querySelectorAll('#side a[data-page]')) a.dataset.page === key ? a.setAttribute('aria-current', 'page') : a.removeAttribute('aria-current')
    const d = $(`#side details[data-id="${key}"]`)
    if (d) d.open = true
    shown = key
    scrollTo(0, 0)
    spy()
  }
  closeMenu()
  const target = at.anchor && document.getElementById(at.anchor)
  if (target) {
    for (let d = target.closest('details'); d; d = d.parentElement.closest('details')) d.open = true
    if (target.id === 'selector' || target.id === 'common-keys') target.querySelector('details').open = true
    target.scrollIntoView()
    target.classList.add('arrived')
    setTimeout(() => target.classList.remove('arrived'), 1600)
  } else if (at.page && !at.anchor) scrollTo(0, 0)
  mark()
}

// The left bar says which section is being read.
let parts = []
function spy() { parts = [...document.querySelectorAll('#page section[id]')] }
function mark() {
  const at = innerHeight * 0.3
  let now = parts.filter(s => s.getBoundingClientRect().top <= at).at(-1) || parts[0]
  if (parts.length && innerHeight + scrollY >= document.documentElement.scrollHeight - 4 && scrollY > 0) now = parts.at(-1)
  for (const a of document.querySelectorAll('#side a[data-section]')) {
    const here = !!now && a.dataset.section === `${shown}/${now.id}`
    if (here !== a.classList.contains('reading')) {
      a.classList.toggle('reading', here)
      if (here) a.scrollIntoView({ block: 'nearest' })
    }
  }
}

function closeMenu() {
  $('#side').classList.remove('open')
  $('#shade').hidden = true
  $('#menu').setAttribute('aria-expanded', 'false')
}

// --- Search ---

const index = []
const kinds = ['Recipes', 'Commands', 'Documentation', 'Keys and options']

function buildIndex() {
  const plain = s => s.body.replace(/<!--[\s\S]*?-->|```[\s\S]*?```|^\|.*$/gm, '').replace(/\[([^\]]+)\]\([^)]+\)/g, '$1').replace(/[`*#🧪]/gu, '').trim()
  for (const s of every()) {
    const kind = s.page.folded ? 'Recipes' : s.command ? 'Commands' : 'Documentation'
    index.push({ kind, title: s.as, where: s.command ? s.group : s.page.title, text: s.command ? s.command.about : plain(s).split('\n\n')[0].replace(/\n/g, ' '), body: s.body, path: `${s.page.id}/${s.id}`, weight: { Recipes: 14, Commands: 16, Documentation: 8 }[kind] })
    if (s.command) for (const k of s.command.keys) index.push({ kind: 'Keys and options', title: k.name, where: s.as, text: `${k.takes}. ${k.about}`, body: (k.values || []).join(' '), path: `${s.page.id}/${s.id}-${k.name.toLowerCase()}`, weight: 0 })
    else for (const m of s.body.matchAll(/^#{2,5} (.+)$/gm)) index.push({ kind, title: m[1], where: s.as, text: '', body: '', path: `${s.page.id}/${slug(m[1])}`, weight: 4 })
  }
  for (const p of pages) index.push({ kind: 'Documentation', title: p.title, where: '', text: p.about, body: '', path: p.id, weight: 12 })
  const keys = (list, where, path) => { for (const k of list) index.push({ kind: 'Keys and options', title: k.name, where, text: `${k.takes}. ${k.about}`, body: '', path, weight: 0 }) }
  keys(rayo.flow.selector, 'Selector', 'commands/selector')
  keys(rayo.flow.options, 'Every command', 'commands/common-keys')
  keys(rayo.suite, 'Suite', 'suites/suites')
  keys(rayo.device, 'Device setup', 'suites/device-setup')
  keys(rayo.header, 'Flow header', 'flows/header')
  for (const o of rayo.options) index.push({ kind: 'Keys and options', title: `--${o.name}`, where: `rayo ${o.commands.join(', ')}`, text: o.about, body: o.short ? `-${o.short}` : '', path: 'cli/options', weight: 2 })
}

// find ranks, and groups by kind: the group with the best comes first.
function find(query) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (!words.length) return []
  const scored = []
  for (const e of index) {
    const title = e.title.toLowerCase(), text = e.text.toLowerCase(), body = e.body.toLowerCase(), place = e.where.toLowerCase()
    let score = 0
    for (const w of words) {
      const s = title === w || title === '--' + w ? 100 : title.startsWith(w) || title.startsWith('--' + w) ? 60 : title.includes(w) ? 42 : title.replace(/ /g, '').includes(w) ? 40 : place.includes(w) ? 16 : text.includes(w) ? 18 : body.includes(w) ? 8 : 0
      if (!s) { score = 0; break }
      score += s
    }
    if (score) scored.push({ ...e, score: score / words.length + e.weight - e.title.length / 20 })
  }
  scored.sort((a, b) => b.score - a.score)
  const seen = new Set(), groups = {}
  for (const e of scored) {
    if (seen.has(e.path + e.title)) continue
    seen.add(e.path + e.title)
    const list = groups[e.kind] ??= []
    if (list.length < 5) list.push(e)
  }
  return kinds.filter(k => groups[k]).map(k => [k, groups[k]]).sort((a, b) => b[1][0].score - a[1][0].score)
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
    const groups = find(q)
    let n = 0
    found.innerHTML = groups.length ? groups.map(([kind, list]) => `<li class="kind" role="presentation">${kind}</li>` + list.map(e =>
      `<li role="option" id="found-${n++}" aria-selected="false"><a href="#/${e.path}" tabindex="-1"><span class="where">${escape(e.where)}</span><b>${marked(e.title, q)}</b><small>${marked(e.text, q)}</small></a></li>`).join('')).join('')
      : `<li class="none">Nothing about “${escape(q)}”</li>`
    found.hidden = false
    input.setAttribute('aria-expanded', 'true')
    choose(0)
    found.scrollTop = 0
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

// --- Begin ---

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
document.addEventListener('click', async e => {
  const button = e.target.closest('.code button')
  if (button) {
    try { await navigator.clipboard.writeText(button.parentElement.querySelector('code').textContent); button.textContent = 'Copied' } catch { button.textContent = 'Select it' }
    setTimeout(() => { button.textContent = 'Copy' }, 1500)
    return
  }
  // A link to where one already is still goes there.
  const a = e.target.closest('a[href^="#/"]')
  if (a && a.getAttribute('href') === location.hash) show()
})
$('#page').addEventListener('input', e => {
  if (!e.target.matches('.filter')) return
  const q = e.target.value.toLowerCase().trim()
  const all = [...document.querySelectorAll('#page details[data-text]')]
  for (const d of all) d.hidden = !d.dataset.text.includes(q)
  const left = all.filter(d => !d.hidden)
  if (q && left.length === 1) left[0].open = true
  for (const s of document.querySelectorAll('#page .group')) s.hidden = !s.querySelector('details[data-text]:not([hidden])')
})
let waiting = false
addEventListener('scroll', () => {
  if (waiting) return
  waiting = true
  requestAnimationFrame(() => { waiting = false; mark() })
}, { passive: true })
addEventListener('hashchange', show)
show()
