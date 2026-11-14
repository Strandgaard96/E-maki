// Opens every tab and section of the settings page and fails on a script
// error or an empty section. Needs a running E-maki, e.g.
//   PANEL_OPEN=1 emaki -script testdata/party.txt &
//   node smoke.mjs http://localhost:8090/
// Uses the installed Chrome, or the browser at $CHROME.
import { chromium } from "playwright-core";

const url = process.argv[2] || "http://localhost:8090/";
const browser = await chromium.launch(
  process.env.CHROME ? { executablePath: process.env.CHROME } : { channel: "chrome" },
);
const errors = [];
const page = await browser.newPage();
page.on("pageerror", (e) => errors.push(e.message));
page.on("console", (m) => m.type() === "error" && errors.push(m.text()));

const check = async (where) => {
  await page.waitForTimeout(300);
  const text = await page.locator("main").innerText();
  if (text.trim().length < 20) errors.push(`${where}: nothing shown`);
  console.log(where);
};

await page.goto(url);
await page.waitForSelector("[role=tab]");
for (const tab of await page.locator("[role=tab]").all()) {
  const name = (await tab.innerText()).trim();
  await tab.click();
  await check(name);
  for (const label of await page.locator("[role=radiogroup][aria-label=Pictures] label").all()) {
    await label.click();
    await check(`${name} › ${(await label.innerText()).trim()}`);
  }
  for (const section of await page.locator('nav[aria-label$=" sections"] button').all()) {
    await section.click();
    await check(`${name} › ${(await section.innerText()).trim()}`);
  }
}
await browser.close();
if (errors.length) {
  console.error(errors.join("\n"));
  process.exit(1);
}
