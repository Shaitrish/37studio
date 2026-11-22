---
name: motion-explainer
description: Short vertical motion-graphics explainers built from real pictures, maps and data, in a paper-collage and editorial-map look (archive photos, engravings, torn paper labels, maps that draw themselves). Use for any 60-100 s reel that explains why something is the way it is, a piece of history, a place, a border, a script or a number. It covers the story structure, the look, motion, timing captions to the voice, free-licence sources, pacing numbers and mistakes already made. Load /cinema-3d too when a scene needs real 3D.
---

# Motion explainer

Learned from one brand's reels, measured from their compositions, chat history and view counts. The best got 210K, 182K, 108K and 78K views. The weakest of the same brand got 1-3K. What separated them was mostly the first line and the story, not the topic or the polish. This skill holds the rules for story, look and timing. The HyperFrames skills already cover the composition contract, GSAP and captions, so read them for that.

## 1. The story comes first

Write the voice script before you pick a single picture. A row of nice scenes with facts read over them is a slideshow, and editors reject slideshows.

**The hook (line 1, 3-5 s) opens on one exact, concrete thing from the viewer's own world, and turns it over.**
- What won: a date and time everyone lived through ("21 November, 10:38. The city shook. It started 90 million years ago."), an everyday object ("This plus sign is a letter. You write it every day."), something learned as a child ("The alphabet you memorised is a map of your mouth."), a paradox about a person ("The man who drew the border had never been to the country."), a surprise about home ("There are whales in our sea.").
- What lost: a foreign name the audience doesn't know, "this is a known exam answer", a soft general question, a date over an empty chart, and anything about the brand itself.
- The numbers: the three winners lost only 22-28% of viewers in the first 3 seconds. Every reel that lost more than 40% stayed under 1,700 views. Fix the first line before you touch the body.
- Put "you" and "your" in it. Use tension, irony or a small laugh, never hype. Truth first: if a source says "encourages", don't write "fixed".

**One object travels through the whole reel.** One physical thing appears in the hook, changes as the story moves, and pays off at the end:
- the script reel: a single gold glyph that morphs through four eras into today's letter;
- the border reel: an ink line that is drawn in the hook, signed, kept secret, torn, burned, and is still the border at the end;
- the map reel: a gold paper cut-out of Greenland, pushed across the map by a real finger.
A new picture can come in for every beat, but the object ties them together. When you can't name the object, the story isn't ready.

**Show the surprise early and answer late.** State the strange fact in line 1-2. Ask "so how?" out loud by about 15-20 s, then answer at 65-75% of the reel with the one reveal shot the reel is remembered by (every point of the world sliding from one map projection to another; the glyph's small head mark stretching into today's top bar). The script reel even shows the end state at 4 s ("it's this letter"), then rolls a year counter back 2,000 years and walks forward.

**One chain, not a list.** Read the lines aloud. Each one must follow from the last, joined by plain words: but, so, because, then, yet, in the end. Cut side stories even when they're good, because a beat the viewer can't link to the last one loses them (an editor about one such beat: "I have no idea what you tried to say here").

**Start from what the viewer knows.** Answer the first question they'd ask before you go deeper ("what is a year?" before leap days; "who made this system?" before weekday names). Use the textbook term once, inline, when the picture shows it. A longer reel is fine if each line earns its place.

**Come back to the viewer's present.** The last fact lands back in their life: the letter on their phone keyboard, the line that is still today's border.

**End on a question people will argue about.** Pick a trap or a debate, ideally with four options on a card, and hold 2-2.8 s of thinking time with a countdown ring. Put the answer in a pinned comment ("how many of you fell for it?"). Don't end on a website, sign-up or logo card: promo posts and promo endings did worst. Links go in the post text.

**No revision section.** Recaps like "remember for the exam" or "three terms to know" were rejected every time. Exam facts go in the post text.

Shape that won (80-98 s, 13-17 voice lines):

| Part | Time | Job |
| --- | --- | --- |
| Hook | 0-5 s | The concrete paradox, with the object already moving at frame 0 |
| Turn | 5-20 s | Who, when, where; name the place the moment a fact needs it; ask the question |
| Chain | 20-60 s | Cause after cause, one claim and one picture per beat, a stake every 10-15 s |
| Reveal | 60-75 s | The answer, shown by the object; the shot people remember |
| Present | 75-85 s | Back to the viewer's life today |
| Question | last 8-10 s | Debate or trap card, thinking time, then the picture holds and ends |

## 2. The look

- **Real pictures carry the facts:** archive photos, engravings, old maps, gazette and newspaper pages, inscriptions, NASA and satellite imagery. They're real things owned by people who are proud of them. Draw or generate a picture only when no real one exists, label it as an illustration, and never generate a portrait of a real person, a fake document or a map whose accuracy matters.
- **Flat colour fields** behind the cut-outs: cream, sea teal, warm yellow, navy, coral, rust. Use one per scene, add a light halftone dot, and multiply paper grain over everything.
- **Paper pieces:** a white cut edge, a soft shadow that grows when the piece lifts, and a tilt of ±0.5-2°.
- **Torn paper strips as labels:** 2-5 words on cream or rust, with ragged edges (a `clip-path` polygon with jittered points) and a slight rotation. One accent colour (rust or red) marks the line that matters most.
- **Cut-out type** for big words and numbers: weight 800-900 with a thick white or navy edge. Write every word in HTML text with a proper font for the language (Noto covers most scripts), never in an image.
- **Photos stay clean:** give them a torn white rim and a strip of tape. Heavy halftone made a real photo look distorted, so keep halftone for small cut-outs. Old engravings go in as they are. Portraits can take an engraved or sepia treatment.
- **Marker strokes draw themselves:** circles, arrows, underlines and ellipses round a date on a real page, with a little wobble and a little change in width.
- **Maps are real data, painted:** a terrain or colour wash for land, ink lines for rivers and borders, place names on paper strips. Real projections only. Anything said as a measurement must be computed in code, not eased by eye (move a country by distance and bearing on the sphere, then project it, so it grows or shrinks truthfully).
- **The palette:** cream paper, dark ink brown or navy, one accent, and gold for the travelling object. Use the brand's design.md colours where they fit, but keep the paper.
- **Type sizes at 1080×1920:** strip text 64-104 px, big numbers 124-210 px, secondary lines 40-60 px, credit line 18-20 px.
- **Pitch it to adults.** No mascots, no storybook look. A 3D object (a glass head, a textured globe) is welcome where it explains better (see /cinema-3d).

## 3. Motion

- **Something moves in every scene, and the movement means something:** a line being drawn, a route travelling, a counter rolling, a glyph morphing, a page being signed, the camera diving from a world map to a district to one field. A still photo with a slow zoom is an accent, never a whole scene. Two reels built from stills and cards were rejected as "slides". A flat infographic of pies, grids and on-screen sums was rejected too: show physical things (coins, paper, a real globe) instead.
- **Paper moves in steps, 12 a second,** like stop-motion: `const step = (t, f = 12) => Math.floor(t * f + 1e-4) / f`. The camera and the marker stay smooth, and wipes step at 24.
- **Keep a slow camera push on every scene** (about 1-2% scale over 3 s) so nothing sits dead.
- **Pop in with a little overshoot,** and only one loud thing at a time. Measured easings: a strip appears in 0.22-0.3 s (opacity 0→1, y +26→0, scale 1.05→1, `power3.out`), and a slam takes 0.16 s (scale 1.5→1, `power4.in`).
- **Transitions:** a torn sheet of the next colour slides up, an ink wipe, a burn or a tear on the beat word. Better still, match-cut on the travelling object.
- **Draw from timeline time:** a canvas or WebGL layer draws each frame from `t` alone, never from frame counts or `Date`. Draw the flat background outside the camera transform, or its edge shows when the camera moves.

## 4. Voice, captions and sound

**Voice.**
- Record the whole script as **one plain take** with no speed tags and no "…" pauses. Separate files per line reset the tone every few seconds, and tags change the voice's style partway through.
- Then transcribe it with word timings (`37studio transcribe <file> --quality best`; the faster model skips chunks of synthetic speech in some languages). Cut pauses in as silence at word boundaries with ffmpeg: 0.1-0.4 s inside a thought, 0.5-0.9 s before a new idea.
- Speed: natural, or at most 1.1×. Reels at 1.25-1.3× were called "too fast" three times.
- Fade the last 80-130 ms of a synthetic voice clip, because it can end on a click.
- Check every name, number and year in the transcript before you place it, then tell the editor exactly which timestamps to listen to. Voices misread years, final vowels and bare letters, so write words the voice says correctly and show the hard symbol on screen.

**Words on screen are keyword strips, not subtitles.**
- Each beat gets 1-3 strips that repeat the key words of the line as it's spoken: a name, a date, a number, the twist.
- A strip appears 0.05-0.25 s before its word, found by matching the word in the transcript (match the whole word and pick the right occurrence, because a substring search hits longer words).
- At most one strip in the top band (y 150-320) and one in the lower band (y 1150-1500). Keep the picture in between, the bottom 20% and the right 12% clear, and faces uncovered. Hide a strip before the next one takes its band. Overlapping text was the most common screen complaint.
- The hook's words are on screen at frame 0. Viewers decide with the sound off.
- When the brand asks for full subtitles, put them in the lower band and move the keyword strips to the top band.
- Credit every sourced picture while it is on screen ("Photo: Name, CC BY-SA 4.0"), 18-20 px at the bottom edge of the picture area. Put the full list in the post text and in work/credits.md.

**Sound** (one sound cue about every 2-3 s: the map reel had 48 in 92 s).
- Every sound belongs to something on screen and stops at the next scene cut.
- Measured levels: voice 1.0, music bed a steady 0.58-0.6 with no sidechain ducking (ducking pumps the music up in every gap), soft stamps and paper 0.2-0.3, stamps and slams 0.36-0.42, marker 0.08. Master true-peak ceiling -1.5 dB.
- Pen, marker and paper sounds sit where the ear is sharpest: low-pass them at 3.8-6 kHz.
- Music: 4-5 sections whose mood changes at the story's turns (riddle, urgency, plain sadness, warm reveal, playful question), cut to the timeline. Use short plucked or struck notes that leave room for the voice, and no long drones, pads, risers or sirens. One long take of the score is better than stitched loops, because gaps at the end were noticed.

## 5. Sources (free licence, credit every one)

See [sources.md](sources.md) for where to find pictures, maps, data and footage, and which licences are safe. In short: Wikimedia Commons (check each file), NASA, USGS, Copernicus Sentinel, Natural Earth, OpenStreetMap, museum open-access collections, and Pexels and Pixabay for footage. Never use NonCommercial or NoDerivatives files for a brand, and never use copyrighted clips.

## 6. Building it in 37Studio

- Edit comp/ by hand, one sub-composition per beat plus one picture layer. Never generate comp/ from a script (it wipes the editor's edits), so do maths such as projections and areas in JS inside the composition. Put small map data (GeoJSON, simplified) in comp/assets.
- If the brand has a collage template (for example "Vox collage reel"), start from it: its engine, paper, grain and fonts are made to be reused. Change the story, data and pictures.
- Before building, write the beat table in work/plan.md: time, voice line, picture, travelling object state, strip text, sound.
- Look for the beat in the registry first (`catalog --query "map route"`, "counter", "torn paper"), then restyle it to paper.
- Before ending a turn, take snapshots at every beat at phone size and check that text never overlaps, the right script renders and the credits are readable.

## 7. Mistakes already made

| Mistake | Fix |
| --- | --- |
| Voice sped up 1.25-1.3× to fit a length | Natural pace; let the reel run 80-100 s |
| One TTS file per line, or speed tags and "…" in one take: the voice sounds like different people | One plain take; add pauses afterwards as silence at word timestamps |
| Voice says a bare letter, year or abbreviation wrong | Rewrite it as words the voice says correctly; show the symbol on screen; transcribe and check before placing |
| Stills and cards with zooms ("slides") | Motion that carries meaning in every scene; footage or real animation, stills as accents |
| Flat 2D infographic (pie, grid, sums) | Physical objects, real textures and maps; 3D where it explains better |
| A foreign name or an empty chart as the hook | Stakes the viewer feels, in local terms (a loss compared with the national budget), then the names |
| A "remember for the exam" recap near the end | Cut it; one story from hook to question; exam facts in the post text |
| Website or sign-up card at the end | End on the question; links in the post text |
| Strips overlapping each other or the subject | One per band, hard-hidden before the next; keep the y 300-1150 picture zone and faces clear |
| A beat whose point nobody got | One claim per beat that you can say in one line; if the picture doesn't prove it, cut the beat |
| Skipping the basics the viewer lacks | Answer their first natural question before the clever part |
| Heavy halftone on a real photo | Halftone only on small cut-outs; photos clean with a torn rim and tape |
| Map rings crossing 180° draw a band across the map | Unwrap longitudes before projecting |
| Dropping points from map rings opens gaps along straight borders | Keep every point, or simplify with a topology-aware tool |
| A word search for a short word matched a longer word | Match whole words and pick the right occurrence |
| Raw pen sound hurt the ear (24 dB over the voice at 3-6 kHz) | Low-pass and compress it; play it only while the pen is on screen |
| Music ducked by a compressor keyed to the voice | One steady bed level under the whole narration |
| A sound runs on into the next scene | End every cue at the next cut |
| A helper (`$$`) used but not defined: the render stalls | Read `check` output; define every helper in each sub-composition |
| Calling it done from the preview | Check frames from the actual render, zoomed in, and run `37studio check` on the file |
