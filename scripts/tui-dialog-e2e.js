// scripts/tui-dialog-e2e.js — end-to-end check of the TUI browse→restore
// dialog against a live backend, asserting on backend ground truth.
//
// Requires: ttyd, node, and Playwright (npm i playwright, or set
// PLAYWRIGHT_PATH / CHROMIUM_PATH). Drive it via a running ttyd:
//
//   CREX_BACKEND=cmux ttyd -W -p 7683 ./bin/crex --layouts-dir <dir> tui &
//   OUTDIR=/tmp/crex-tui node scripts/tui-dialog-e2e.js
//
// Exits 0 when the browse list → restore dialog → Add flow creates the
// expected workspace/tab in the backend, non-zero otherwise. Screenshots
// land in $OUTDIR for visual review.
const PLAYWRIGHT_PATH = process.env.PLAYWRIGHT_PATH || 'playwright';
const { chromium } = require(PLAYWRIGHT_PATH);
const { execSync } = require('child_process');
const OUT = process.env.OUTDIR || '/tmp/crex-tui';
const PORT = process.env.PORT || 7683;
const BACKEND = process.env.CREX_BACKEND || 'cmux';
const CMUX = process.env.CMUX_BIN || 'cmux';
const sleep = ms => new Promise(r => setTimeout(r, ms));
const osa = s => execSync(`osascript -e '${s}'`).toString().trim();

(async () => {
  require('fs').mkdirSync(OUT, { recursive: true });
  let winsBefore = 0, tabsBefore = 0;
  if (BACKEND === 'ghostty') {
    winsBefore = parseInt(osa('tell application "Ghostty" to count of windows')) || 0;
    tabsBefore = parseInt(osa('tell application "Ghostty" to count of tabs of front window')) || 0;
  }
  const browser = await chromium.launch(
    process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {});
  const page = await browser.newPage({ viewport: { width: 1100, height: 700 } });
  await page.goto(`http://localhost:${PORT}`);
  await sleep(3000);
  const type = async (s, w) => { await page.evaluate(x => window.term.input(x), s); await sleep(w); };
  await type('ls\r', 2000); await page.screenshot({ path: `${OUT}/01-browse.png` });
  await type('\r', 2000);   await page.screenshot({ path: `${OUT}/02-dialog.png` });
  await type('a', 1000);
  await sleep(10000);       await page.screenshot({ path: `${OUT}/03-after.png` });

  let ok;
  if (BACKEND === 'ghostty') {
    const w = parseInt(osa('tell application "Ghostty" to count of windows'));
    const t = parseInt(osa('tell application "Ghostty" to count of tabs of front window'));
    ok = w === winsBefore && t === tabsBefore + 1;
    console.log(`ASSERT ghostty windows ${winsBefore}->${w} tabs ${tabsBefore}->${t}`);
  } else {
    const list = execSync(`${CMUX} workspace list`, { env: { ...process.env, CMUX_QUIET: '1' } }).toString();
    ok = /audit-tui-ws/.test(list);
    console.log(`ASSERT cmux audit-tui-ws present: ${ok}`);
  }
  await type('exit\r', 500);
  await browser.close();
  process.exit(ok ? 0 : 1);
})().catch(e => { console.error(e); process.exit(2); });
