---
name: cinema-3d
description: Build cinematic 3D scenes in vertical (9:16) HyperFrames reels with Three.js, such as a globe, a planet or night sky, a 3D head or body cutaway, a battlefield or other miniature world, or a camera flying through a scene. Use it whenever a reel's story is told inside a 3D world, or a shot needs real depth and a moving camera. Holds the setup that renders reliably, the camera, light and text rules that worked, and the mistakes already made once. Read it after /hyperframes and before writing any Three.js.
---

# Cinema 3D

Learned from three finished 3D reels: a rainy battlefield, a glass head with the mouth cut away, and a night sky of planets that turns into a globe. The best of them passed 200K views. The generic contract (seek protocol, `data-duration`, `buildReady`, cut constants, the lit-mesh fill light, shader `pow` domains) is in `/hyperframes-animation` `adapters/three.md` and `/hyperframes` `references/types/3d.md`. This skill adds what those reels taught on top of that. `scene-template.html` next to this file is a working sub-composition (it passes `check`, and seeks out of order to the same frames). Start from it.

## The idea comes first

- One world per reel, and every shot is inside it. The editor threw out stock clips and AI clips cut between 3D shots. The only outside pictures he kept were real archive items (a painting, a manuscript page, NASA photos), placed in the world as a plane or sprite and graded to match.
- Every spoken fact gets its own picture: the camera is on the thing being named when the word is said. Wide shots are for finding your way and last 3 to 4 s at most. A stretch of pretty aerials under a line of fact got "I have no idea what you tried to say here".
- Make the abstract physical. The alphabet became places in a mouth, and the week became seven lights in the sky. A demonstration (a candle flame bending, rings of sound, breath through the nose) beats a diagram.
- Frame 1 is already moving, with the hook object on screen. No logo during the hook.
- Tell the story and stop. A "remember this for the exam" recap was cut. Low-poly figures were accepted as a "stylized miniature" once the grade and the motion were right, so spend effort on the look and the story before detailed models.

## Setup that renders

- **Library:** Three.js, one pinned release copied into the project so renders never depend on the network. Current releases split the module build into two files: copy `build/three.module.js` and `build/three.core.js` from the same release into `assets/lib/`. Get them with `npm pack three@0.181.2` in `work/` (it only downloads the package, it runs nothing), then `tar -xzf three-0.181.2.tgz package/build package/examples/jsm` and copy what you need. Then `import * as THREE from "./assets/lib/three.module.js"`. For addons (`GLTFLoader` and so on), copy them from `examples/jsm/` of the same release. Put `<script type="importmap">{"imports":{"three":"./assets/lib/three.module.js"}}</script>` in the root `index.html` `<head>`, because addons import the bare name `three`. If downloads are refused, stop and ask; don't work around the permission system.
- **Structure:** the 3D world is a sub-composition (`compositions/scene.html`) whose engine lives in `assets/scene.js`. Every beat time lives in one file, `assets/cues.js` (`export default { dur, hook, dive, ... }`), timed to the words of the voice. Each reel's voice was re-recorded 4 to 6 times, so with one cues file, re-timing means editing numbers instead of hunting through 1,000 lines. Keep camera keys there too.
- **Time:** `render(t)` is a pure function of `t`. A sub-composition drives it from a GSAP proxy tween, which gives local time: `tl.to(st, { t: C.dur, duration: C.dur, ease: "none", onUpdate: () => render(st.t) }, 0)`, registered as `window.__timelines["scene"]` after the async setup. A canvas placed directly in the root `index.html` can listen to `hf-seek` instead (the head reel did this). Register `window.__hf.buildReady.<key>` for font and data loading. Never use `requestAnimationFrame`, `Clock`, `performance.now()` or deltas from the previous frame.
- **Each sub-composition loads its own library.** A scene that relied on a `<script>` in the root failed `check`.
- **Renderer:** fixed `setSize(1080, 1920, false)`, `setPixelRatio(1)`, `preserveDrawingBuffer: true`. Use `antialias: false` with a `WebGLRenderTarget(W, H, { samples: 4 })` and one post pass, or `antialias: true` when there is no post pass. `THREE.ColorManagement.enabled = false` plus `outputColorSpace = LinearSRGBColorSpace` makes brand hex colours render exactly as written.
- **Determinism:** seed every random choice (mulberry32 `rng(seed)`, or `hash(i)`). Loops are modulo of `t`, for example rain height `H - ((hash(i) * H + t * speed) % H)`, and splash cycles come from `Math.floor(t / period + hash(i))`. Physics and crowds are precomputed or written as closed-form functions of `t`. A waving flag sets its vertices from `sin(x * k - t * w)` every frame.
- **Assets load before the first seek:** textures through `TextureLoader` (the runtime waits for it), and fonts and data through `buildReady`. Nothing may load at render time. A missing file shows up as an `http_error` in `check`: draw a fallback instead of requesting a file that doesn't exist.
- **Live map tiles never finish inside the renderer**, because it owns the page clock and the "loaded" signal never comes. Bake the map move to an MP4 first (`/hyperframes` `film.md`, "Maps and real places") and place it as a video clip, or draw land from Natural Earth or world-atlas data into a canvas texture on a sphere.

## Camera language that worked

- **Two working models.** (a) A shot table: short shots of 1.3 to 5 s with hard cuts. Each shot has a 2- or 3-point path for the position and for the look-at target, a fov ramp, a focus distance and per-shot weather: `{ t: [a, b], p: [[...],[...]], l: [[...],[...]], fov: [42, 38], f: 4.4 }`, sampled with a quadratic Bezier. (b) One continuous camera through Catmull-Rom keys `[t, px, py, pz, tx, ty, tz, fov]`, crossing to the next world with a dive.
- **Never fully still.** Ease each segment as `eio(u) * 0.8 + u * 0.2`, so the camera doesn't stop dead at a key. Add a small drift on close shots: sine drift of about 0.01 units, or value noise of 0.02 to 0.05 units on hand-held close-ups, about a quarter of that on the look-at target.
- **Dive, don't orbit.** Push into the object as it is named. Change worlds by flying into something (the Sun, the mouth, the globe) with a short light flash of 0.2 s on the cut. Worlds are `Group`s switched by `visible` at a cut constant.
- **Vertical framing.** Three's fov is vertical, so at fov 40 a 9:16 frame is only about 23 degrees wide. A row of five items that fits a 16:9 frame gets cut off at both edges. Stack things vertically, or place a row from the camera so its ends land inside 120 to 888 px. Put the subject high in the middle of the frame with its label under it. A fov of 34 to 38 worked for close cutaways, 40 to 50 for landscapes and space.
- **Straight-down shots:** set `camera.up` to `(0, 0, -1)` so north stays at the top of the frame.
- **Things at infinity** (sky dome, stars) copy `camera.position` every frame.
- **Verify cuts** on the last frame before a cut and the first frame after it (`adapters/three.md`).

## Light, materials, look

- **Night, space and glass:** a near-black navy ground (#01030a to #03060c). Glows are additive sprites with a canvas radial-gradient texture, glass and atmospheres a fresnel rim shader (`pow(1.0 - clamp(abs(dot(n, v)), 0.0, 1.0), 2.0)`), and stars seeded `Points` with a slight twinkle in the shader from `uTime`. Floating dust as bokeh points gives depth: size and blur grow with distance from the focus plane.
- **Daylight worlds:** a `HemisphereLight` fill, a warm `DirectionalLight` key with soft shadows, and the shadow camera fitted to each shot's subject (bias about -0.0006, normalBias about 0.03). Use `FogExp2` in the sky's horizon colour, and a sky-dome shader with a gradient and a warm band. Reflections come from `PMREMGenerator.fromScene()` over a tiny scene (the sky plus one bright strip), so bronze and water shine with no HDRI file. Short-lived `PointLight`s handle muzzle flashes and embers.
- **Grade in one post pass:** an ACES-style tone curve, about 18% desaturation, yellow-greens pushed toward olive and teal, contrast around 0.3, warm highlights and cool shadows, a vignette, and grain seeded per 1/24 s (`floor(t * 24)`). Procedural worlds come out milky and lime green by default. That look was rejected until it was graded darker and deeper.
- **Everything moves a little:** clouds turn on their dome, grass bends from a wind uniform, smoke drifts, banners wave, the globe turns slowly and the camera drifts. The `dead_stretch` warning in `check` can't see inside a canvas, so prove the motion with snapshots instead.
- **Real photos of round things** (planets, the Moon): turn black into alpha in a canvas at load (alpha from the brightest channel) and show them as sprites. For a real place, use an equirectangular texture on a sphere, with day and night maps if the story needs them.
- **Flashes keep the picture.** A flash that turned the whole frame flat beige was rejected. Mix 30 to 40% toward a warm tint over 0.2 s instead.
- **No model available?** Build it from primitives, merged geometries with vertex colours, or a signed-distance shape sampled onto a sphere. That is how the glass head of the most-viewed reel was made.

## Text over 3D on 9:16

- **Three layers.** (1) Statements: a few big words as HTML clips over the canvas, at 80 to 104 px with a 44 px line under them, rising in with a slight blur. (2) Labels pinned to objects, as canvas-text sprites in the world or as DOM elements placed with `v.project(camera)`. (3) Captions that must stay put while the world turns, anchored to the camera: `cam.position + forward * d + up * k`.
- **Projected DOM labels:** `x = (v.x * 0.5 + 0.5) * 1080`, `y = (-v.y * 0.5 + 0.5) * 1920`, in composition pixels even when the canvas renders smaller. Clamp `x` to the safe band, hide the label when `v.z > 1` (behind the camera), and give it a dark text shadow and a short leader line down to the object.
- **Canvas text** handles any script, because the browser shapes it. First `await document.fonts.load('700 120px "Family"', '<sample in that script>')`, so the right subset loads. Draw a glow pass (shadowBlur) and then a pale core. Redraw every label canvas after the fonts arrive: a canvas never redraws itself. For depth of field, cross-fade a sharp and a blurred copy of the sprite by distance from the focus.
- **Safe zone:** words stay inside the zone in `/hyperframes` `references/types/vertical.md`, which `check` enforces. When a world-space label drifts past the edge as the scene turns, anchor it to the camera instead.
- **Contrast:** text that sits on a bright sky fails. Tilt the camera so the words sit over the dark ground, or move the words.
- **On-screen words match the voice.** Each word or letter appears at the moment it is spoken, from the transcript's word times. Small labels wait until the big ones have gone, rather than colliding with them.
- **Brand and credits:** the logo is small and faint (about 40% opacity) in a top corner, and arrives after the hook. CC BY assets get a small, faint credit line while they're on screen.

## Performance and rendering

- Measured on the editor's machine: 54 s with shadows, 16-tap depth of field and 5,000 grass blades took 33 min. 92 s of additive glass and lines took 20 min. 65 s of sprites and photos took 13 min. Check `render --fps 15 --quality draft` first, then render the full film once.
- Heavy scene: render the canvas at 75% (810×1440) and stretch it with CSS. After grain the difference is invisible, and it draws almost half as many pixels. Don't use the `--resolution` 4K presets with a fixed-size canvas: the 3D gets upscaled.
- Use `InstancedMesh` for crowds, grass, rain and splashes, and merge static props into one geometry with vertex colours. Hide groups that aren't in the current shot, and update instance matrices only for visible systems. Build canvases and textures once at setup, and reuse vectors instead of allocating new ones every frame.
- Ignore "GPU stall due to ReadPixels" warnings in `check`: they come from frame capture.
- When a snapshot looks wrong, snapshot that one moment again before fixing it. Stale captures happen.

## Assets with free licences

- Prefer procedural: seeded noise textures, canvas-drawn silhouettes (tree lines, boats), and geometry built in code. Nothing to license, and nothing to download at render time.
- Models, textures and HDRIs: Poly Haven (CC0), ambientCG (CC0), Kenney and Quaternius (CC0, low-poly and rigged characters). Other glTF models (three.js examples, Khronos samples, Sketchfab) each carry their own licence: read it and credit CC BY.
- Space and Earth: NASA imagery (public domain, but not NASA logos), Solar System Scope textures (CC BY 4.0, credit them), and Natural Earth or world-atlas data (public domain) for land and borders.
- Archive pictures: Wikimedia Commons public-domain paintings and pages. Check each file's licence through the Commons page and caption copies honestly (for example "17th-century copy").
- Copy everything into `assets/`. Never hotlink.

## Mistakes already made, and the fix

1. Stock and AI clips cut between 3D shots: rejected. Fix: stay in one world, and bring real archive items into it as graded planes.
2. Pretty wide shots under a spoken fact: "no idea what you tried to say". Fix: one clear picture per fact, close on the named thing.
3. Milky, lime-green procedural colour. Fix: a post grade (tone curve, olive and teal greens, deep shadows, warm highlights, vignette, grain).
4. Shots that felt static. Fix: drift on the camera, plus wind, clouds, smoke and banners in every shot.
5. Words over bright sky. Fix: tilt the camera down so the text sits on dark ground.
6. Labels drifted off the right edge while the system turned. Fix: anchor them to the camera, or clamp the projected `x`.
7. Rows of letters cut at the left and right edges. Fix: account for the narrow 9:16 width, and stack or re-centre from the camera.
8. Letters on screen didn't match the letters being spoken. Fix: cue each item to its word time in `cues.js`.
9. A flash cut became a flat beige frame. Fix: a partial, 0.2 s warm mix that keeps the image.
10. Canvas labels drawn in the fallback font. Fix: await `document.fonts.load` with sample text, then redraw the label canvases.
11. The scene file used a library loaded only by the root. Fix: each sub-composition imports its own Three.js.
12. A live map never finished rendering. Fix: bake it to video first.
13. 404s for textures that were never made. Fix: draw fallbacks without requesting a file.
14. The voice was sped up to fit the timing: "too fast". Fix: keep the voice at natural pace and let the reel run longer. The editor prefers a longer reel that teaches over a rushed one.
15. Re-timing after every new voice take was slow. Fix: keep every beat in one cues file, timed from the transcript's word times or the pauses in the take.
