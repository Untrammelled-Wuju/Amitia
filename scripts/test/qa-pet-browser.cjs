const { app, BrowserWindow, session, ipcMain } = require('electron')
const assert = require('node:assert/strict')
const { existsSync, readFileSync } = require('node:fs')
const { join, resolve } = require('node:path')
const root = resolve(process.argv[2])
assert(existsSync(join(root, '.qa-pet-isolated')))
app.setPath('userData', join(root, 'qa-electron'))
const host = 'http://127.0.0.1:5178'
const getEndpoint = () => JSON.parse(readFileSync(join(root, 'run', 'endpoint.json'))).httpBaseURL
setTimeout(() => { console.error('Browser deadline exceeded'); app.exit(1) }, 45000)
app.whenReady().then(async () => {
  const browser = session.fromPartition('qa-pet-web')
  let heldPreview
  let previewRequests = 0
  let releasePreview
  let latestPreviewStatus
  const release = new Promise(resolve => { releasePreview = resolve })
  browser.protocol.handle('http', async request => {
    const url = new URL(request.url)
    if (url.origin !== host) return new Response(null, { status: 403 })
    if (!/^\/(api|internal|media|livez|readyz)(\/|$)/.test(url.pathname)) {
      return fetch(url)
    }
    const headers = new Headers(request.headers)
    headers.delete('origin')
    headers.delete('host')
    headers.set('X-Amitia-Local-Token', readFileSync(join(root, 'security', 'local-token'), 'utf8').trim())
    const previewNumber = url.pathname === '/api/extensions/packages/artifacts' ? ++previewRequests : 0
    try {
      const endpoint = new URL(getEndpoint())
      assert.equal(endpoint.hostname, '127.0.0.1')
      assert.notEqual(endpoint.port, '3000')
      const response = await fetch(endpoint.origin + url.pathname + url.search, {
        method: request.method, headers,
        body: request.body ? Buffer.from(await request.arrayBuffer()) : undefined,
      })
      console.log(request.method, url.pathname, response.status)
      if (previewNumber === 1) {
        heldPreview = response.status
        await release
      }
      if (previewNumber === 2) latestPreviewStatus = response.status
      return response
    } catch (error) {
      console.log(request.method, url.pathname, error.message)
      return new Response(null, { status: 502 })
    }
  })
  browser.webRequest.onBeforeRequest({ urls: ['https://*/*', 'ws://*/*', 'wss://*/*'] }, (_, callback) => callback({ cancel: true }))
  ipcMain.handle('qa-mesh-identity', () => JSON.parse(readFileSync(join(root, 'device-mesh', 'identity.json'))))
  const win = new BrowserWindow({ show: false, webPreferences: { session: browser, sandbox: true, preload: join(__dirname, 'qa-pet-browser-preload.cjs') } })
  win.webContents.on('preload-error', (_event, _preload, error) => console.error(error))
  await win.loadURL(host + '/#/creative-workshop/pet/installations')
  console.log('Page loaded')
  const js = script => win.webContents.executeJavaScript(script)
  async function waitFor(check, message) {
    const end = Date.now() + 15000
    do {
      if (await check()) return
      await new Promise(resolve => setTimeout(resolve, 100))
    } while (Date.now() < end)
    console.log(await js('document.body.innerText.slice(0, 1800)'))
    throw new Error(message)
  }
  const shown = text => js(`document.body.innerText.includes(${JSON.stringify(text)})`)
  const click = (selector, text) => waitFor(() => js(`(() => {
    const target = [...document.querySelectorAll(${JSON.stringify(selector)})].find(el =>
      el.getClientRects().length && !el.disabled && el.textContent.trim() === ${JSON.stringify(text)})
    if (!target) return false
    target.click()
    return true
  })()`), 'Missing control: ' + text)
  if (await shown('开始使用')) {
    await js("document.querySelector('.consent-footer input').click()")
    await click('.consent-footer button', '开始使用')
  }
  await waitFor(() => shown('com.amitia.builtin.desktop-pet'), 'Built-in plugin list did not load')
  await click('.pet-plugin-manager .section-head button', '安装插件')
  console.log('Opening dialog')
  async function choose(file) {
    await waitFor(() => js("Boolean(document.querySelector('.package-drop-zone input[type=file]'))"), 'File input not found')
    if (!win.webContents.debugger.isAttached()) win.webContents.debugger.attach('1.3')
    const { root: { nodeId } } = await win.webContents.debugger.sendCommand('DOM.getDocument')
    const { nodeId: input } = await win.webContents.debugger.sendCommand('DOM.querySelector', {
      nodeId, selector: '.package-drop-zone input[type=file]',
    })
    assert(input, 'File input not found')
    await win.webContents.debugger.sendCommand('DOM.setFileInputFiles', { nodeId: input, files: [join(root, file)] })
  }
  console.log('Selecting first fixture')
  await choose('qa-good.petx')
  await waitFor(() => heldPreview, 'First upload did not return')
  assert.equal(heldPreview, 201)
  await choose('qa-bad.petx')
  await waitFor(() => latestPreviewStatus, 'Second upload did not return')
  assert.equal(latestPreviewStatus, 422)
  await waitFor(() => shown('qa-bad.petx'), 'Second selection did not replace the first')
  releasePreview()
  await new Promise(resolve => setTimeout(resolve, 800))
  assert.equal(await js("document.querySelector('.package-preview')?.innerText ?? null"), null, 'Old successful upload replaced the failed latest selection')
  async function confirmPackage(file, action) {
    await choose(file)
    await waitFor(() => shown('QA Desktop Pet Plugin'), 'Valid preview not rendered')
    await waitFor(() => js(`(() => {
      const input = document.querySelector('.install-confirmation input')
      if (!input) return false
      input.click()
      return true
    })()`), 'Package confirmation checkbox not rendered')
    await click('.el-dialog button', action)
  }
  await confirmPackage('qa-good.petx', '确认安装')
  const qaRow = '.pet-plugin-manager .plugin-item'
  const row = () => js(`[...document.querySelectorAll('${qaRow}')].find(el => el.innerText.includes('com.amitia.qa/desktop-pet-plugin'))?.innerText ?? ''`)
  await waitFor(async () => (await row()).includes('1.0.0'), 'Installed package not rendered in list')
  const clickAction = action => js(`(() => {
    const row = [...document.querySelectorAll('${qaRow}')].find(el => el.innerText.includes('com.amitia.qa/desktop-pet-plugin'))
    const button = [...row.querySelectorAll('button')].find(el => el.textContent.trim() === ${JSON.stringify(action)})
    if (!button || button.disabled) throw new Error('Action is missing: ' + ${JSON.stringify(action)})
    button.click()
  })()`)
  if ((await row()).includes('已启用')) {
    await clickAction('禁用')
    await waitFor(async () => (await row()).includes('已禁用'), 'Disable state not rendered')
  }
  await clickAction('启用')
  await waitFor(async () => (await row()).includes('已启用'), 'Enable state not rendered')
  await clickAction('更新')
  await choose('qa-good.petx')
  await waitFor(() => shown('版本 1.0.0 已安装'), 'Same-version package was not recognized')
  assert(await js("[...document.querySelectorAll('.el-dialog button')].some(button => button.textContent.trim() === '确认更新' && button.disabled)"))
  await confirmPackage('qa-update.petx', '确认更新')
  await waitFor(async () => (await row()).includes('1.0.1'), 'Updated version not rendered')
  await win.webContents.reload()
  await waitFor(async () => {
    const state = await row()
    return state.includes('1.0.1') && state.includes('已启用')
  }, 'Reload lost updated/enabled state')
  await clickAction('卸载')
  await click('.el-message-box button', '确认卸载')
  await waitFor(async () => !(await row()), 'Uninstall did not remove plugin')
  await click('.pet-plugin-manager .section-head button', '安装插件')
  await confirmPackage('qa-good.petx', '确认安装')
  await waitFor(async () => (await row()).includes('1.0.0'), 'Reinstall after uninstall did not restore the package')
  await clickAction('卸载')
  await click('.el-message-box button', '确认卸载')
  await waitFor(async () => !(await row()), 'Second uninstall did not remove plugin')
  console.log('PASS file upload race, install, enable, same-version rejection, update, reload, uninstall, reinstall and second uninstall')

  app.exit(0)
}).catch(error => {
  console.error(error)
  app.exit(1)
})
