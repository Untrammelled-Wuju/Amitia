const { chromium } = require("C:/Users/Untrammelled/AppData/Local/npm-cache/_npx/e41f203b7505f1fb/node_modules/playwright");

async function getState(page) {
  const world = await page.$(".onboarding-world");
  const ds = await world?.getAttribute("data-stage-current");
  const cp = await world?.getAttribute("data-core-reveal-pending");
  const ep = await world?.getAttribute("data-stage-transitioning");
  const is = await world?.getAttribute("data-identity-state");
  const ms = await world?.getAttribute("data-memory-step");
  return { ds, cp, ep, is, ms };
}

(async () => {
  const browser = await chromium.launch({ 
    headless: false,
    channel: "chrome",
    args: ["--no-sandbox"]
  });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

  await page.goto("http://localhost:5178/onboarding", { waitUntil: "networkidle" });
  await page.waitForTimeout(2000);

  // Stage 0: click intro
  const introGhost = await page.$(".ob-stage.active .ob-primary-ghost");
  if (introGhost) {
    await introGhost.click();
    console.log("Stage 0: clicked primary-ghost");
  }
  await page.waitForTimeout(2000);

  // Stage 1: select local, then click next
  const localBtn = await page.$(".ob-stage.active .ob-path-choice.local");
  if (localBtn) { await localBtn.click(); console.log("Stage 1: selected local"); }
  await page.waitForTimeout(500);
  const stage1Next = await page.$(".ob-stage.active .ob-stage-action");
  if (stage1Next) { await stage1Next.click(); console.log("Stage 1: clicked next"); }
  await page.waitForTimeout(3000);

  // Stage 2: admin setup - try to set account
  const envURLInput = await page.$(".ob-stage.active .ob-field");
  if (envURLInput) {
    await envURLInput.fill("http://localhost:18000");
    console.log("Stage 2: filled env URL");
    await page.waitForTimeout(500);
    const healthBtn = await page.$(".ob-stage.active .ob-small-action");
    if (healthBtn) { await healthBtn.click(); console.log("Stage 2: clicked health check"); }
  }
  await page.waitForTimeout(3000);
  
  // Try filling admin credentials
  const usernameInputs = await page.$$(".ob-stage.active input[type='text']");
  const passwordInputs = await page.$$(".ob-stage.active input[type='password']");
  for (const input of usernameInputs) {
    try { await input.fill("admin"); console.log("Stage 2: filled username"); break; } catch(e) {}
  }
  for (const input of passwordInputs) {
    try { await input.fill("admin123"); console.log("Stage 2: filled password"); break; } catch(e) {}
  }
  await page.waitForTimeout(500);
  
  // Submit
  const submitBtn = await page.$(".ob-stage.active .ob-setup-inline-action");
  if (submitBtn) { await submitBtn.click(); console.log("Stage 2: clicked submit"); }
  await page.waitForTimeout(4000);

  // Stage 3-6: config stages - just click next
  for (let i = 3; i <= 6; i++) {
    await page.waitForTimeout(2500);
    const nextBtn = await page.$(".ob-stage.active .ob-stage-action");
    if (nextBtn) {
      const visible = await nextBtn.isVisible();
      if (visible) { await nextBtn.click(); console.log(`Stage ${i}: clicked next`); }
    }
  }
  await page.waitForTimeout(3000);

  // Stage 7: Identity - fill in answers
  console.log("\n=== Stage 7: Identity ===");
  for (let q = 0; q < 3; q++) {
    await page.waitForTimeout(1000);
    let state = await getState(page);
    console.log(`  Identity Q${q}:`, state);
    
    const ta = await page.$(".ob-stage.active .ob-identity-answer textarea");
    if (ta) {
      await ta.fill("测试答案" + (q+1));
      console.log(`  Q${q}: filled textarea`);
      await page.keyboard.press("Enter");
      console.log(`  Q${q}: pressed Enter`);
      await page.waitForTimeout(1500);
    } else {
      console.log(`  Q${q}: textarea NOT FOUND`);
      break;
    }
  }
  
  // Avatar step (step 3)
  await page.waitForTimeout(1500);
  const skipBtn = await page.$(".ob-stage.active .ob-skip-btn");
  if (skipBtn) {
    await skipBtn.click();
    console.log("  Avatar: clicked skip");
  }
  
  // Wait for spotlight + complete animation
  await page.waitForTimeout(4000);
  
  // Click "继续" on complete view
  const completeNext = await page.$(".ob-stage.active .ob-identity-complete-next");
  if (completeNext) {
    await completeNext.click();
    console.log("Stage 7: clicked complete-next");
  }
  
  // Wait for transition to stage 8
  await page.waitForTimeout(5000);

  // === Stage 8: Memory ===
  console.log("\n=== Stage 8: Memory ===");
  
  // Print current state
  let s8State = await getState(page);
  console.log("  State:", s8State);
  
  // Check all stages
  const allStages = await page.$$(".ob-stage");
  for (let i = 0; i < allStages.length; i++) {
    const s = allStages[i];
    const cl = await s.getAttribute("class");
    const cs = await s.evaluate(el => {
      const st = getComputedStyle(el);
      return `pe:${st.pointerEvents} op:${st.opacity} vis:${st.visibility} zi:${st.zIndex}`;
    });
    console.log(`  Stage[${i}] ${cl.substring(0,30)}... => ${cs}`);
  }

  // Stage 8 is nth-child(12) based on template structure
  const stage8 = await page.$$(".ob-stage");
  const s8 = stage8[8]; // 0-indexed, stage 8
  if (s8) {
    const s8Active = (await s8.getAttribute("class")).includes("active");
    console.log("\n  Stage 8 active:", s8Active);
    
    if (s8Active) {
      const inner = await s8.$(".ob-stage-inner");
      if (inner) {
        const html = await inner.evaluate(el => el.innerHTML.substring(0, 600));
        console.log("  Stage 8 inner:", html);
      }
    }
  }

  // Try to find the memory textarea using active stage context
  const memTextarea = await page.$(".ob-stage.active .ob-identity-answer textarea");
  if (memTextarea) {
    const visible = await memTextarea.isVisible();
    console.log("\n  Memory textarea visible:", visible);
    const taStyle = await memTextarea.evaluate(el => {
      const s = getComputedStyle(el);
      const r = el.getBoundingClientRect();
      return { pe: s.pointerEvents, op: s.opacity, display: s.display, rect: r };
    });
    console.log("  Memory textarea style:", JSON.stringify(taStyle));
    
    try {
      await memTextarea.click();
      console.log("  Memory textarea: click OK");
      await memTextarea.fill("test");
      console.log("  Memory textarea: fill OK");
    } catch(e) {
      console.log("  Memory textarea: action failed -", e.message);
    }
  } else {
    console.log("\n  Memory textarea NOT FOUND in active stage");
  }

  const memSendBtn = await page.$(".ob-stage.active .ob-identity-answer button");
  if (memSendBtn) {
    const bs = await memSendBtn.evaluate(el => {
      const s = getComputedStyle(el);
      return { pe: s.pointerEvents, display: s.display, disabled: el.disabled, op: s.opacity };
    });
    console.log("  Memory send button:", JSON.stringify(bs));
  }

  await page.screenshot({ path: "D:/桌面/跟进项目/U-Ai/stage8-debug.png" });
  console.log("\nScreenshot saved.");
  
  // Keep browser open for manual inspection
  console.log("\nKeeping browser open for 30s for inspection...");
  await page.waitForTimeout(30000);
  await browser.close();
})();
