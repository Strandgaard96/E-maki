// The full-screen view: the frame's picture, fetched again when it changes.
const img = document.getElementById("page");
const empty = document.getElementById("empty");
let shown = "";
async function check() {
  try {
    const r = await fetch("api/page", { cache: "no-store" });
    const { page } = await r.json();
    empty.hidden = !!page;
    img.hidden = !page;
    if (page && page !== shown) {
      shown = page;
      img.src = "preview.png?v=" + encodeURIComponent(page);
    }
  } catch { /* the brain is restarting: try again next round */ }
}
check();
setInterval(check, 30000);
