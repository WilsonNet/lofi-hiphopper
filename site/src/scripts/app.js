const terminal = document.querySelector("#terminal");
const canvas = document.querySelector("#visualizer");
const modeLabel = document.querySelector("#mode-label");
const peakLabel = document.querySelector("#peak-label");
const dialog = document.querySelector("#shortcut-dialog");
const dialogClose = document.querySelector(".dialog-close");
const toast = document.querySelector("#copy-toast");
const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

const palette = {
  violet: "#7b5cff",
  magenta: "#d568ff",
  pink: "#ff86c2",
  peach: "#ffc48a",
  mint: "#7fe7d4",
  blue: "#6d8bff",
  cream: "#fff3dd",
  panel: "#0d0d18",
  border: "#23233a",
  dim: "#4e4a66",
  muted: "#8a85a3",
};

const MODES = ["BARS", "MIRROR", "SCOPE", "HALO", "AURORA", "MATRIX"];
const BAND_COUNT = 48;
const WAVE_COUNT = 256;
const F_MIN = 40;
const F_MAX = 16000;

let mode = 0;
let width = 0;
let height = 0;
let dpr = 1;
let ctx = null;
let last = 0;
let cycleTimer = 0;
let manualUntil = 0;
const bands = new Float32Array(BAND_COUNT);
const peaks = new Float32Array(BAND_COUNT);
const reflection = new Float32Array(BAND_COUNT);
const target = new Float32Array(BAND_COUNT);
const wave = new Float32Array(WAVE_COUNT);
const previousWave = new Float32Array(WAVE_COUNT);

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function hexToRgb(hex) {
  const value = Number.parseInt(hex.slice(1), 16);
  return { r: (value >> 16) & 255, g: (value >> 8) & 255, b: value & 255 };
}

function mix(a, b, t) {
  const ca = hexToRgb(a);
  const cb = hexToRgb(b);
  return `rgb(${Math.round(ca.r + (cb.r - ca.r) * t)}, ${Math.round(ca.g + (cb.g - ca.g) * t)}, ${Math.round(ca.b + (cb.b - ca.b) * t)})`;
}

function gradient(t) {
  t = clamp(t, 0, 1);
  if (t < 0.4) return mix(palette.violet, palette.magenta, t / 0.4);
  if (t < 0.75) return mix(palette.magenta, palette.pink, (t - 0.4) / 0.35);
  return mix(palette.pink, palette.peach, (t - 0.75) / 0.25);
}

function auroraGradient(t) {
  t = clamp(t, 0, 1);
  if (t < 0.35) return mix(palette.mint, palette.blue, t / 0.35);
  if (t < 0.7) return mix(palette.blue, palette.violet, (t - 0.35) / 0.35);
  return mix(palette.violet, palette.pink, (t - 0.7) / 0.3);
}

function resize() {
  if (!canvas) return;
  const rect = canvas.getBoundingClientRect();
  if (!rect.width || !rect.height) return;
  dpr = Math.min(2, window.devicePixelRatio || 1);
  width = rect.width;
  height = rect.height;
  canvas.width = Math.round(width * dpr);
  canvas.height = Math.round(height * dpr);
  ctx = canvas.getContext("2d");
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  drawStatic();
}

function makeTarget(time) {
  const t = time * 0.001;
  for (let i = 0; i < BAND_COUNT; i++) {
    let value = 0.055 + 0.11 * (0.5 + 0.5 * Math.sin(t * 0.8 + i * 0.27));
    const centers = [4, 13, 24, 36, 42];
    for (let b = 0; b < centers.length; b++) {
      const center = centers[b] + Math.sin(t * (0.5 + b * 0.08) + b) * (1.3 + b * 0.4);
      const distance = i - center;
      value += (0.46 - b * 0.055) * Math.exp(-(distance * distance) / (8 + b * 2)) * (0.55 + 0.45 * Math.sin(t * (1.1 + b * 0.2) + b));
    }
    value *= 0.72 + 0.28 * Math.exp(-i / 34);
    target[i] = clamp(value, 0, 1);
  }
}

function makeWave(time) {
  const t = time * 0.001;
  for (let i = 0; i < WAVE_COUNT; i++) {
    wave[i] =
      0.26 * Math.sin(i * 0.038 + t * 4.2) +
      0.1 * Math.sin(i * 0.082 - t * 2.6 + 1.1) +
      0.045 * Math.sin(i * 0.017 + t * 1.5);
  }
}

function smooth(dt) {
  makeTarget(performance.now());
  makeWave(performance.now());
  const attack = 1 - Math.pow(0.001, dt * 2.8);
  const release = 1 - Math.pow(0.001, dt * 0.72);
  for (let i = 0; i < BAND_COUNT; i++) {
    const rate = target[i] > bands[i] ? attack : release;
    bands[i] += (target[i] - bands[i]) * rate;
    if (bands[i] > peaks[i]) peaks[i] = bands[i];
    else peaks[i] = Math.max(bands[i], peaks[i] - dt * 0.25);
    reflection[i] += (bands[i] - reflection[i]) * dt * 2.5;
  }
  for (let i = 0; i < WAVE_COUNT; i++) {
    previousWave[i] = wave[i];
  }
}

function clear() {
  if (!ctx) return;
  ctx.clearRect(0, 0, width, height);
}

function drawBars() {
  const columns = clamp(Math.floor(width / 12), 16, 48);
  const rows = height - 8;
  const slot = width / columns;
  const values = resample(bands, columns);
  const peakValues = resample(peaks, columns);
  for (let i = 0; i < columns; i++) {
    const x = i * slot + Math.max(0, (slot - Math.max(2, slot * 0.62)) / 2);
    const barWidth = Math.max(2, slot * 0.62);
    const value = clamp(values[i] * 1.02, 0, 1);
    const units = value * rows;
    const full = Math.floor(units);
    for (let row = 0; row < full; row++) {
      const y = height - 7 - row;
      const heightT = row / Math.max(1, rows);
      ctx.fillStyle = gradient(0.18 + heightT * 0.55 + (i / columns) * 0.27);
      ctx.fillRect(x, y, barWidth, 1.1);
    }
    const peakY = height - 7 - clamp(Math.round(peakValues[i] * rows), 0, rows);
    if (peakValues[i] > 0.02) {
      ctx.fillStyle = palette.cream;
      ctx.globalAlpha = 0.8;
      ctx.fillRect(x, peakY, barWidth, 1.2);
      ctx.globalAlpha = 1;
    }
  }
  ctx.fillStyle = "rgba(78, 74, 102, 0.65)";
  for (let i = 0; i < columns; i++) {
    ctx.fillRect(i * slot + slot * 0.35, height - 2, 1, 1);
  }
}

function drawMirror() {
  const columns = clamp(Math.floor(width / 12), 16, 48);
  const axis = height / 2;
  const half = Math.min(axis - 5, height - axis - 5);
  const slot = width / columns;
  const values = resample(bands, columns);
  const lower = resample(reflection, columns);
  for (let i = 0; i < columns; i++) {
    const x = i * slot + Math.max(0, (slot - Math.max(2, slot * 0.62)) / 2);
    const barWidth = Math.max(2, slot * 0.62);
    const up = values[i] * half;
    const down = lower[i] * half * 0.82;
    for (let row = 0; row < up; row++) {
      const y = axis - row;
      ctx.fillStyle = gradient(0.18 + (row / Math.max(1, half)) * 0.6 + (i / columns) * 0.24);
      ctx.fillRect(x, y, barWidth, 1.1);
    }
    for (let row = 0; row < down; row++) {
      const y = axis + row + 1;
      ctx.fillStyle = mix(palette.panel, auroraGradient(i / columns), 0.62 - (row / Math.max(1, half)) * 0.3);
      ctx.fillRect(x, y, barWidth, 1.1);
    }
  }
  ctx.fillStyle = "rgba(78, 74, 102, 0.7)";
  for (let x = 0; x < width; x += 8) ctx.fillRect(x, axis, 1, 1);
}

function resample(values, count) {
  const out = new Float32Array(count);
  for (let i = 0; i < count; i++) {
    const position = (i * (values.length - 1)) / Math.max(1, count - 1);
    const left = Math.floor(position);
    const right = Math.min(left + 1, values.length - 1);
    out[i] = values[left] + (values[right] - values[left]) * (position - left);
  }
  return out;
}

function smoothCurve(values, radius) {
  const out = new Float32Array(values.length);
  for (let i = 0; i < values.length; i++) {
    let sum = 0;
    let count = 0;
    for (let offset = -radius; offset <= radius; offset++) {
      const index = i + offset;
      if (index < 0 || index >= values.length) continue;
      const weight = 1 - Math.abs(offset) / (radius + 1);
      sum += values[index] * weight;
      count++;
    }
    out[i] = count ? sum / count : 0;
  }
  return out;
}

function drawWave(points, color, thickness) {
  ctx.beginPath();
  for (let i = 0; i < points.length; i++) {
    const x = (i / (points.length - 1)) * width;
    const y = height / 2 - points[i] * (height * 0.41);
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.strokeStyle = color;
  ctx.lineWidth = thickness;
  ctx.lineJoin = "round";
  ctx.lineCap = "round";
  ctx.stroke();
}

function drawScope() {
  const columns = clamp(Math.floor(width / 9), 12, 40);
  const silhouette = resample(bands, columns);
  for (let i = 0; i < columns; i++) {
    const barHeight = silhouette[i] * height * 0.3;
    ctx.fillStyle = "rgba(123, 92, 255, 0.13)";
    ctx.fillRect((i / columns) * width, height - barHeight, Math.max(1, width / columns - 1), barHeight);
  }
  const current = smoothCurve(wave, 3);
  const previous = smoothCurve(previousWave, 3);
  drawWave(previous, "rgba(109, 139, 255, 0.25)", 1);
  drawWave(current, "rgba(201, 167, 255, 0.22)", 5);
  drawWave(current, palette.pink, 1.5);
  ctx.fillStyle = "rgba(78, 74, 102, 0.65)";
  for (let x = 0; x < width; x += 10) ctx.fillRect(x, height / 2, 1, 1);
}

function drawHalo() {
  const cx = width / 2;
  const cy = height / 2;
  const radius = Math.min(width, height) * 0.44;
  const inner = radius * 0.34;
  const rays = 96;
  const values = resample(bands, rays);
  let bass = 0;
  for (let i = 0; i < 8; i++) bass += bands[i];
  bass /= 8;
  ctx.lineWidth = 1;
  for (let i = 0; i < rays; i++) {
    const angle = -Math.PI / 2 + (i / rays) * Math.PI * 2;
    const length = inner + values[i] * (radius - inner);
    const x0 = cx + Math.cos(angle) * inner;
    const y0 = cy + Math.sin(angle) * inner;
    const x1 = cx + Math.cos(angle) * length;
    const y1 = cy + Math.sin(angle) * length;
    ctx.strokeStyle = mix(palette.violet, palette.pink, i / rays);
    ctx.beginPath();
    ctx.moveTo(x0, y0);
    ctx.lineTo(x1, y1);
    ctx.stroke();
    if (values[i] > 0.05) {
      ctx.fillStyle = palette.cream;
      ctx.fillRect(x1, y1, 1.2, 1.2);
    }
  }
  ctx.strokeStyle = "rgba(201, 167, 255, 0.25)";
  ctx.beginPath();
  ctx.arc(cx, cy, inner, 0, Math.PI * 2);
  ctx.stroke();
  ctx.strokeStyle = "rgba(35, 35, 58, 0.9)";
  ctx.beginPath();
  ctx.arc(cx, cy, radius, 0, Math.PI * 2);
  ctx.stroke();

  const core = inner * 0.56 + bass * inner * 0.3;
  const glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, core);
  glow.addColorStop(0, "rgba(255, 243, 221, 0.75)");
  glow.addColorStop(0.36, "rgba(255, 134, 194, 0.42)");
  glow.addColorStop(1, "rgba(123, 92, 255, 0)");
  ctx.fillStyle = glow;
  ctx.beginPath();
  ctx.arc(cx, cy, core, 0, Math.PI * 2);
  ctx.fill();

  const orbit = performance.now() * 0.00055;
  ctx.fillStyle = palette.cream;
  ctx.beginPath();
  ctx.arc(cx + Math.cos(orbit) * inner * 0.72, cy + Math.sin(orbit) * inner * 0.72, 1.6, 0, Math.PI * 2);
  ctx.fill();
}

function hash(value) {
  let h = value * 2654435761;
  h ^= h >>> 13;
  h *= 1597334677;
  h ^= h >>> 16;
  return (h >>> 0) / 4294967295;
}

function drawAurora() {
  for (let i = 0; i < Math.floor((width * height) / 1700); i++) {
    const x = hash(i * 3 + 1) * width;
    const y = hash(i * 7 + 2) * height * 0.72;
    const twinkle = 0.25 + 0.35 * (0.5 + 0.5 * Math.sin(performance.now() * 0.0012 + i));
    ctx.fillStyle = `rgba(233, 228, 255, ${twinkle})`;
    ctx.fillRect(x, y, 1, 1);
  }
  const columns = clamp(Math.floor(width / 3), 20, 200);
  const back = smoothCurve(resample(bands, columns), 9);
  const front = smoothCurve(resample(bands, columns), 4);
  fillAurora(back, 0.9, 0.14, 0.8, "back");
  fillAurora(front, 0.74, 0.48, 0.62, "front");
}

function fillAurora(values, heightScale, colorOffset, alpha, layer) {
  const columns = values.length;
  const step = width / columns;
  for (let i = 0; i < columns; i++) {
    const value = clamp(0.03 + 0.84 * Math.sqrt(values[i]) + 0.03 * Math.sin(i * 0.11 + performance.now() * 0.001), 0, 1);
    const top = height - value * height * heightScale;
    const x = i * step;
    const grad = ctx.createLinearGradient(0, top, 0, height);
    const topColor = auroraGradient(colorOffset + (i / columns) * 0.35);
    const bottomColor = mix(palette.panel, palette.violet, 0.3);
    grad.addColorStop(0, topColor);
    grad.addColorStop(1, bottomColor);
    ctx.globalAlpha = alpha;
    ctx.fillStyle = grad;
    ctx.fillRect(x, top, step + 0.5, height - top);
    ctx.globalAlpha = 1;
    ctx.fillStyle = layer === "front" ? "rgba(255, 196, 138, 0.55)" : "rgba(127, 231, 212, 0.38)";
    ctx.fillRect(x, top, step + 0.5, 1);
  }
}

function drawMatrix() {
  const columns = clamp(Math.floor(width / 13), 12, 52);
  const rows = Math.floor(height / 11);
  const slot = width / columns;
  const values = resample(bands, columns);
  const peakValues = resample(peaks, columns);
  const scan = Math.floor(performance.now() * 0.002) % Math.max(1, rows);
  for (let i = 0; i < columns; i++) {
    const x = i * slot + slot / 2;
    const lit = Math.round(values[i] * rows);
    const peak = Math.round(peakValues[i] * rows);
    for (let row = 0; row < rows; row++) {
      const y = height - 10 - row * ((height - 20) / rows);
      let color = "rgba(35, 35, 58, 0.55)";
      if (row === scan) color = "rgba(78, 74, 102, 0.7)";
      if (row < lit) color = gradient(0.18 + (row / rows) * 0.62);
      ctx.fillStyle = color;
      ctx.beginPath();
      ctx.arc(x, y, row < lit ? 1.55 : 0.9, 0, Math.PI * 2);
      ctx.fill();
    }
    if (peak > lit) {
      ctx.fillStyle = palette.cream;
      ctx.beginPath();
      ctx.arc(x, height - 10 - peak * ((height - 20) / rows), 1.4, 0, Math.PI * 2);
      ctx.fill();
    }
  }
}

function draw() {
  if (!ctx || !width || !height) return;
  clear();
  switch (mode) {
    case 0: drawBars(); break;
    case 1: drawMirror(); break;
    case 2: drawScope(); break;
    case 3: drawHalo(); break;
    case 4: drawAurora(); break;
    case 5: drawMatrix(); break;
  }
  updatePeak();
}

function updatePeak() {
  let best = 0;
  for (let i = 1; i < BAND_COUNT; i++) if (bands[i] > bands[best]) best = i;
  const frequency = F_MIN * Math.pow(F_MAX / F_MIN, (best + 0.5) / BAND_COUNT);
  if (bands[best] < 0.02) {
    peakLabel.textContent = "peak —";
    return;
  }
  peakLabel.textContent = frequency >= 1000
    ? `peak ${(frequency / 1000).toFixed(1)} kHz`
    : `peak ${Math.round(frequency)} Hz`;
}

function drawStatic() {
  if (!ctx || !width || !height) return;
  if (!bands.some((value) => value > 0)) {
    makeTarget(performance.now());
    for (let i = 0; i < BAND_COUNT; i++) {
      bands[i] = target[i];
      peaks[i] = target[i];
      reflection[i] = target[i] * 0.7;
    }
    makeWave(performance.now());
  }
  draw();
}

function setMode(next) {
  mode = (next + MODES.length) % MODES.length;
  modeLabel.textContent = `${MODES[mode]} · ${mode + 1}/6`;
  manualUntil = performance.now() + 18000;
  draw();
}

function loop(now) {
  if (!last) last = now;
  const dt = Math.min(0.05, (now - last) / 1000);
  last = now;
  if (!reducedMotion.matches) {
    smooth(dt);
    draw();
    requestAnimationFrame(loop);
  }
}

function startCycling() {
  if (cycleTimer) clearInterval(cycleTimer);
  cycleTimer = window.setInterval(() => {
    if (reducedMotion.matches) return;
    if (performance.now() < manualUntil) return;
    setMode(mode + 1);
    manualUntil = 0;
  }, 9500);
}

function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(text);
  }
  return new Promise((resolve, reject) => {
    const area = document.createElement("textarea");
    area.value = text;
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    try {
      document.execCommand("copy") ? resolve() : reject(new Error("copy failed"));
    } catch (error) {
      reject(error);
    } finally {
      area.remove();
    }
  });
}

document.querySelectorAll(".copy-button").forEach((button) => {
  button.addEventListener("click", () => {
    copyText(button.dataset.copy || "").then(() => {
      button.textContent = "copied";
      toast.textContent = "copied to clipboard";
      toast.classList.add("show");
      window.setTimeout(() => {
        button.textContent = "copy";
        toast.classList.remove("show");
      }, 1800);
    }).catch(() => {
      toast.textContent = "copy failed";
      toast.classList.add("show");
      window.setTimeout(() => toast.classList.remove("show"), 1800);
    });
  });
});

if (terminal) {
  terminal.addEventListener("keydown", (event) => {
    if (event.key === "h" || event.key === "ArrowLeft") {
      event.preventDefault();
      setMode(mode - 1);
    } else if (event.key === "l" || event.key === "ArrowRight") {
      event.preventDefault();
      setMode(mode + 1);
    } else if (event.key === "?") {
      event.preventDefault();
      if (dialog) dialog.showModal();
    }
  });
  canvas?.addEventListener("click", () => setMode(mode + 1));
}

if (dialog) {
  dialogClose?.addEventListener("click", () => dialog.close());
  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) dialog.close();
  });
}

const year = document.querySelector("#year");
if (year) year.textContent = String(new Date().getFullYear());

if (canvas) {
  const observer = new ResizeObserver(resize);
  observer.observe(canvas);
  resize();
  if (reducedMotion.matches) {
    drawStatic();
  } else {
    makeTarget(performance.now());
    makeWave(performance.now());
    for (let i = 0; i < BAND_COUNT; i++) {
      bands[i] = target[i] * 0.65;
      peaks[i] = bands[i];
      reflection[i] = bands[i] * 0.7;
    }
    requestAnimationFrame(loop);
    startCycling();
  }
  reducedMotion.addEventListener("change", () => {
    if (reducedMotion.matches) {
      if (cycleTimer) clearInterval(cycleTimer);
      drawStatic();
    } else {
      requestAnimationFrame(loop);
      startCycling();
    }
  });
}
