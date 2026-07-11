// scripts/e2e-tui-runner.js
//
// E2E TUI test runner — drives crex TUI via ttyd + Playwright.
// Captures a screenshot for each of 26 test cases and writes report.json.
//
// Usage: node scripts/e2e-tui-runner.js [--port 7682] [--cases 1,2,5]
//
// Requires:
//   - ttyd running on the specified port with crex tui
//   - Playwright (npm install playwright, or point PLAYWRIGHT_PATH at an install)

const PLAYWRIGHT_PATH = process.env.PLAYWRIGHT_PATH || 'playwright';
const { chromium } = require(PLAYWRIGHT_PATH);
const fs = require('fs');
const path = require('path');
const {
  getTerminalContent,
  getByText,
  waitForText,
  waitForStable,
  expectVisible,
  expectNotVisible,
  sendCommand: sendCmd,
  sleep: helperSleep,
} = require('./e2e-helpers');

// --- CLI args ---

const args = process.argv.slice(2);
let port = 7682;
let caseFilter = null;

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--port' && args[i + 1]) port = parseInt(args[i + 1], 10);
  if (args[i] === '--cases' && args[i + 1]) caseFilter = new Set(args[i + 1].split(',').map(Number));
}

const TTYD_URL = `http://localhost:${port}`;
const OUT_DIR = '/tmp/crex-e2e';
const SHOT_DIR = path.join(OUT_DIR, 'screenshots');

// --- Test definitions ---

const TESTS = [
  // Core Shell (1-6)
  { id: 1,  name: 'launch',              input: null,                        wait: 3000, expected: "Welcome message with 'crex' prompt, help/exit highlighted" },
  { id: 2,  name: 'help',                input: 'help\r',                    wait: 1500, expected: '6 groups with icons, colored headers, aligned columns' },
  { id: 3,  name: 'ls-browse',           input: 'ls\r',                      wait: 1500, expected: 'Numbered items in browse mode with cursor' },
  { id: 4,  name: 'browse-quit',         input: 'q',                         wait: 500,  expected: 'Return to prompt' },
  { id: 5,  name: 'templates-browse',    input: 'templates\r',               wait: 1500, expected: '16 templates in browse mode' },
  { id: 6,  name: 'templates-quit',      input: 'q',                         wait: 500,  expected: 'Return to prompt' },
  // Backend-dependent (7-8)
  { id: 7,  name: 'now-error',           input: 'now\r',                     wait: 3000, expected: 'Error message (no backend in ttyd), prompt recovery' },
  { id: 8,  name: 'save-error',          input: 'save test\r',               wait: 3000, expected: 'Error message, prompt recovery' },
  // Usage errors (9-14)
  { id: 9,  name: 'usage-restore',       input: 'restore\r',                 wait: 1000, expected: 'Red error: Usage: restore <name|#>' },
  { id: 10, name: 'usage-delete',        input: ['delet', 'e\r'],            wait: 1000, expected: 'Red error: Usage: delete <name|#>' },
  { id: 11, name: 'usage-use',           input: 'use\r',                     wait: 1000, expected: 'Red error: Usage: use <template|#>' },
  { id: 12, name: 'usage-bp-add',        input: 'bp add\r',                  wait: 1000, expected: 'Red error: Usage: bp add <name> <path>' },
  { id: 13, name: 'usage-bp-remove',     input: 'bp remove\r',               wait: 1000, expected: 'Red error: Usage: bp remove <name|#>' },
  { id: 14, name: 'usage-bp-toggle',     input: 'bp toggle\r',               wait: 1000, expected: 'Red error: Usage: bp toggle <name|#>' },
  // Features (15-18)
  { id: 15, name: 'watch-status',        input: 'watch status\r',            wait: 1000, expected: 'watch daemon is not running' },
  { id: 16, name: 'bp-list',             input: 'bp list\r',                 wait: 1000, expected: 'Blueprint entries in browse mode or empty message' },
  { id: 17, name: 'bp-list-quit',        input: 'q',                         wait: 500,  expected: 'Return to prompt' },
  { id: 18, name: 'unknown-cmd',         input: 'foobar\r',                  wait: 1000, expected: 'Red error: Unknown command: foobar' },
  // Tab Completion (19-22)
  { id: 19, name: 'tab-level1',          input: '\t',                        wait: 1000, expected: 'All commands with icons and descriptions' },
  { id: 20, name: 'tab-escape',          input: '\x1b',                      wait: 500,  expected: 'Completion dismissed, back to prompt' },
  { id: 21, name: 'tab-bp-level2',       input: { pre: 'bp', tab: true },    wait: 1000, expected: 'bp subcommands: add, list, ls, remove, rm, toggle' },
  { id: 22, name: 'tab-settings-level3', input: { pre: 'settings banner', tab: true }, wait: 1000, expected: 'banner subcommands: get, list, set' },
  // Settings Output (23-24)
  { id: 23, name: 'settings-banner-get', input: 'settings banner get\r',     wait: 1000, expected: 'Current banner style: flame (green text)' },
  { id: 24, name: 'settings-banner-list',input: 'settings banner list\r',    wait: 1000, expected: '3 styles with aligned descriptions' },
  // Edge Cases (25-27)
  { id: 25, name: 'history-up',          input: ['\x1b[A', '\x1b[A'],        wait: 500,  expected: 'Recalls previous commands in prompt' },
  { id: 26, name: 'empty-enter',         input: '\r',                        wait: 500,  expected: 'No output, stays in prompt' },
  { id: 27, name: 'exit',                input: 'exit\r',                    wait: 2000, expected: 'Phoenix bye message, ttyd shows reconnect' },
];

// --- Helpers ---

async function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}

async function clearPrompt(page) {
  await page.evaluate(() => window.term.input('\x15'));
  await sleep(200);
}

async function sendInput(page, input) {
  if (input === null) return; // launch — no input needed
  if (Array.isArray(input)) {
    for (const chunk of input) {
      await page.evaluate((c) => window.term.input(c), chunk);
      await sleep(300);
    }
  } else if (typeof input === 'object' && input.tab) {
    await page.evaluate((t) => window.term.input(t), input.pre);
    await sleep(300);
    await page.evaluate(() => window.term.input('\t'));
  } else {
    await page.evaluate((t) => window.term.input(t), input);
  }
}

async function shot(page, name) {
  const file = path.join(SHOT_DIR, name);
  await page.screenshot({ path: file, fullPage: true });
}

// --- Main ---

let browser;

(async () => {
  fs.mkdirSync(SHOT_DIR, { recursive: true });

  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1200, height: 800 } });

  console.log(JSON.stringify({ event: 'start', url: TTYD_URL }));

  await page.goto(TTYD_URL, { timeout: 10000 });
  await sleep(3000); // wait for xterm.js + crex init

  const tests = TESTS.filter(t => !caseFilter || caseFilter.has(t.id));
  const results = [];

  for (const t of tests) {
    const padId = String(t.id).padStart(2, '0');
    const screenshotName = `${padId}-${t.name}.png`;

    // Clear prompt before each test (except launch and cases that follow browse quit)
    if (t.id > 1 && t.input !== 'q' && !(Array.isArray(t.input) && t.input[0] === '\x1b[A')) {
      await clearPrompt(page);
    }

    await sendInput(page, t.input);
    await sleep(t.wait);
    await shot(page, screenshotName);

    results.push({
      id: t.id,
      name: t.name,
      screenshot: `screenshots/${screenshotName}`,
      input: t.input,
      expected: t.expected,
      status: 'captured',
    });

    console.log(JSON.stringify({ event: 'captured', id: t.id, name: t.name, screenshot: screenshotName }));
  }

  const report = {
    timestamp: new Date().toISOString(),
    binary: '/tmp/crex-e2e/crex-test',
    port,
    testCount: results.length,
    tests: results,
  };

  fs.writeFileSync(path.join(OUT_DIR, 'report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ event: 'done', testCount: results.length, reportPath: path.join(OUT_DIR, 'report.json') }));

  await browser.close();
})().catch(async err => {
  console.error(JSON.stringify({ event: 'error', message: err.message, stack: err.stack }));
  if (browser) await browser.close().catch(() => {});
  process.exit(1);
});
