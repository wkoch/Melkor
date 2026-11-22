const scene = document.querySelector(".scene");
const plate = document.querySelector(".plate");
const canvas = document.querySelector(".resonance");
const status = document.querySelector(".status");
const ink = canvas.getContext("2d");
const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");

// Cada estrela começa no centro do brilho da gravura de 1672 × 941.
const paths = [
  [[750, 821], [690, 788], [619, 810], [636, 755], [552, 762]],
  [
    [1008, 524], [951, 508], [924, 475], [884, 500], [890, 400], [788, 471],
    [802, 334], [727, 403], [749, 348], [675, 378], [602, 419], [526, 445],
  ],
  [[1001, 357], [971, 329], [908, 352], [926, 299], [850, 310]],
  [[1279, 370], [1190, 352], [1145, 383], [1162, 311], [1057, 316]],
  [[1356, 248], [1284, 219], [1238, 246], [1252, 192], [1175, 204]],
  [[1410, 174], [1356, 162], [1317, 182], [1332, 135], [1260, 145]],
  [[1438, 184], [1502, 169], [1488, 224], [1553, 210], [1526, 267]],
];

let frame;
let hoveredStar;
let activeStar;

function drawBolt(path, reach, opacity) {
  ink.clearRect(0, 0, canvas.width, canvas.height);
  if (reach <= 0 || opacity <= 0) return;

  const last = Math.ceil(reach);
  const [fromX, fromY] = path[last - 1];
  const [toX, toY] = path[last];
  const fraction = reach - last + 1;

  ink.beginPath();
  ink.moveTo(...path[0]);
  for (let point = 1; point < last; point += 1) ink.lineTo(...path[point]);
  ink.lineTo(fromX + (toX - fromX) * fraction, fromY + (toY - fromY) * fraction);

  ink.lineCap = "round";
  ink.lineJoin = "round";
  ink.shadowColor = "#ff8b4b";
  ink.shadowBlur = 29;
  ink.globalAlpha = opacity * 0.78;
  ink.strokeStyle = "#ff9d56";
  ink.lineWidth = 13;
  ink.stroke();

  ink.shadowBlur = 12;
  ink.globalAlpha = opacity;
  ink.strokeStyle = "#ffd19a";
  ink.lineWidth = 6;
  ink.stroke();

  ink.shadowBlur = 0;
  ink.strokeStyle = "#fff8de";
  ink.lineWidth = 3.5;
  ink.stroke();
}

function resonate(star, announce = false) {
  cancelAnimationFrame(frame);
  activeStar?.button.classList.remove("is-active");
  activeStar = star;
  star.button.classList.add("is-active");
  if (announce) status.textContent = `Raio da estrela ${star.number} de ${paths.length}.`;

  const started = performance.now();
  const still = reducedMotion.matches;
  const segments = star.path.length - 1;
  function animate(now) {
    const elapsed = now - started;
    const reach = still ? segments : segments * Math.min(elapsed / 180, 1);
    const opacity = still ? Number(elapsed < 680) : 1 - Math.min(Math.max(elapsed - 220, 0) / 760, 1);
    if (ink) drawBolt(star.path, reach, opacity);
    if (opacity > 0) {
      frame = requestAnimationFrame(animate);
    } else {
      star.button.classList.remove("is-active");
      activeStar = undefined;
    }
  }
  frame = requestAnimationFrame(animate);
}

function nearestStar(event) {
  const bounds = plate.getBoundingClientRect();
  let nearest;
  let distance = 23;
  for (const star of stars) {
    const [x, y] = star.path[0];
    const offsetX = event.clientX - (bounds.left + x * bounds.width / canvas.width);
    const offsetY = event.clientY - (bounds.top + y * bounds.height / canvas.height);
    const offset = Math.hypot(offsetX, offsetY);
    if (offset < distance) {
      distance = offset;
      nearest = star;
    }
  }
  return nearest;
}

const stars = paths.map((path, index) => {
  const [x, y] = path[0];
  const number = index + 1;
  const button = document.createElement("button");
  button.className = "star";
  button.type = "button";
  button.setAttribute("aria-label", `Acender a estrela ${number} de ${paths.length}`);
  button.style.setProperty("--x", `${100 * x / canvas.width}%`);
  button.style.setProperty("--y", `${100 * y / canvas.height}%`);
  button.style.setProperty("--delay", `${-index * 0.43}s`);
  const star = { path, button, number };
  button.addEventListener("click", () => resonate(star, true));
  plate.append(button);
  return star;
});

scene.addEventListener("pointermove", event => {
  if (event.pointerType !== "mouse") return;
  const star = nearestStar(event);
  if (star === hoveredStar) return;
  hoveredStar = star;
  scene.classList.toggle("is-over-star", Boolean(star));
  if (star) resonate(star);
});

scene.addEventListener("pointerleave", () => {
  hoveredStar = undefined;
  scene.classList.remove("is-over-star");
});

scene.addEventListener("click", event => {
  if (event.target.closest("button")) return;
  const star = nearestStar(event);
  if (star) resonate(star, true);
});
