const button = document.querySelector(".pluck");
const canvas = document.querySelector(".resonance");
const status = document.querySelector(".status");
const ink = canvas.getContext("2d");
const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");

// Pontos da linha luminosa na gravura, a partir da corda que Melkor toca.
const path = [
  [1008, 524], [951, 508], [924, 475], [884, 500], [890, 400],
  [788, 471], [802, 334], [727, 403], [749, 348], [675, 378], [602, 419], [526, 445],
];

const chords = [
  { name: "ré menor", notes: [146.83, 174.61, 220, 293.66], color: "#ffc486" },
  { name: "ré maior", notes: [146.83, 185, 220, 293.66], color: "#fff2b9" },
  { name: "dó menor", notes: [130.81, 155.56, 196, 261.63], color: "#f38e65" },
];

let audio;
let frame;
let chordIndex = 0;

async function pluckChord(notes) {
  if (!window.AudioContext) return;
  audio ??= new AudioContext();
  if (audio.state !== "running") await audio.resume();

  notes.forEach((frequency, index) => {
    const start = audio.currentTime + index * 0.065;
    const string = audio.createOscillator();
    const gain = audio.createGain();

    string.type = "triangle";
    string.frequency.value = frequency;
    gain.gain.setValueAtTime(0.0001, start);
    gain.gain.exponentialRampToValueAtTime(0.032, start + 0.018);
    gain.gain.exponentialRampToValueAtTime(0.0001, start + 0.85);

    string.connect(gain).connect(audio.destination);
    string.start(start);
    string.stop(start + 0.86);
    string.onended = () => {
      string.disconnect();
      gain.disconnect();
    };
  });
}

function showResonance(color) {
  if (!ink) return;
  cancelAnimationFrame(frame);

  function draw(position, fade) {
    ink.clearRect(0, 0, canvas.width, canvas.height);
    ink.strokeStyle = color;
    ink.shadowColor = color;
    ink.shadowBlur = 17;
    ink.lineWidth = 3;
    ink.lineCap = "round";

    for (let index = 1; index < path.length; index += 1) {
      const distance = Math.abs(index - position);
      ink.globalAlpha = fade * (reducedMotion.matches ? 0.55 : Math.max(0, 1 - distance / 4));
      ink.beginPath();
      ink.moveTo(...path[index - 1]);
      ink.lineTo(...path[index]);
      ink.stroke();
    }
  }

  if (reducedMotion.matches) {
    draw(0, 1);
    return;
  }

  const started = performance.now();
  function animate(timestamp) {
    const progress = Math.min((timestamp - started) / 1150, 1);
    draw(progress * (path.length + 2), 1 - progress ** 2);
    if (progress < 1) frame = requestAnimationFrame(animate);
  }
  frame = requestAnimationFrame(animate);
}

button.addEventListener("click", () => {
  const chord = chords[chordIndex];
  chordIndex = (chordIndex + 1) % chords.length;
  status.textContent = `A canção muda para ${chord.name}.`;
  showResonance(chord.color);
  // A cena também funciona se o navegador restringir áudio.
  pluckChord(chord.notes).catch(() => {});
});
button.hidden = false;
