// usage: node capture.mjs <chrome> <repo-root> <seeded-url> <empty-url>
// Writes <name>-light.png and <name>-dark.png at 2x for every screen.
import puppeteer from "puppeteer-core";
import { join } from "node:path";

const [chrome, root, seeded, empty] = process.argv.slice(2);
const browser = await puppeteer.launch({ executablePath: chrome, headless: true, args: ["--no-sandbox"] });
const settle = (ms = 1200) => new Promise((r) => setTimeout(r, ms)); // entrance animations

async function session(scheme, width, height) {
  const page = await (await browser.createBrowserContext()).newPage(); // own cookies: the wizard keeps state in a session
  await page.setViewport({ width, height, deviceScaleFactor: 2 });
  await page.emulateMediaFeatures([{ name: "prefers-color-scheme", value: scheme }, { name: "prefers-reduced-motion", value: "reduce" }]);
  await page.evaluateOnNewDocument((s) => { try { localStorage.setItem("delistd-theme", s); } catch {} }, scheme);
  return page;
}

// the seeded home trips the upgrade/what's-new banners; dismiss them so the docs show the plain page
async function dismissBanners(page, url) {
  for (let i = 0; i < 5; i++) {
    await page.goto(url, { waitUntil: "networkidle0" });
    const btn = await page.$('.notice-dismiss, form[action$="/dismiss"] button');
    if (!btn) return;
    await Promise.all([page.waitForNavigation({ waitUntil: "load" }), btn.click()]);
  }
}

for (const scheme of ["light", "dark"]) {
  const out = (dir, name) => join(root, dir, `${name}-${scheme}.png`);

  // full-width pages, cropped to the top of the viewport
  for (const [path, dir, name, height] of [
    ["/", ".github/images", "dashboard", 545],
    ["/automation", ".github/images", "automation-approvals", 520],
  ]) {
    const page = await session(scheme, 1280, height);
    await dismissBanners(page, seeded + path);
    await settle();
    await page.screenshot({ path: out(dir, name) });
    if (name === "dashboard") await page.screenshot({ path: out("site/static/img", name) }); // homepage hero, same 2560x1090 crop
    await page.close();
  }

  // setup wizard: fill the profile with fictional data, shoot the card on each step
  const page = await session(scheme, 560, 900);
  await page.goto(empty + "/setup/profile", { waitUntil: "networkidle0" });
  await page.type("#first_name", "Jane");
  await page.type("#last_name", "Doe");
  await page.type("#email", "jane.doe@gmail.com");
  await settle();
  await (await page.$(".card")).screenshot({ path: out("docs/images", "wizard-profile") });
  await Promise.all([page.waitForNavigation({ waitUntil: "load" }), page.click("form button[type=submit]")]);
  await page.select("#mail_provider", "gmail");
  await page.type("#mail_password", "abcdabcdabcdabcd");
  await settle();
  await (await page.$(".card")).screenshot({ path: out("docs/images", "wizard-email") });
  await page.close();
}
await browser.close();
