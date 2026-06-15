# claws — brand imagery prompts

A working file for generating consistent imagery across the GitHub
repo, the landing page, social cards, and marketing. Fifteen prompts.
Every prompt is written for two audiences at once: a generative
image model (paste-ready paragraph at the bottom) and a human
illustrator/photographer (a layered brief with feeling notes).

If you only commission five images, do **01, 02, 03, 04, 09** —
that's the minimum that covers hero + OG share card + icon +
product story + the master wordmark.

---

## What claws actually is (so every brief stays honest)

A single-binary Go CLI that helps one person run a small team of
AI agents on their own server. Each agent is a persistent persona
that lives in a container, talks to messaging apps (Telegram,
WhatsApp, Discord, Slack), and survives reboots. Setup is one
command (`claws setup`). Most users are not engineers.

The product is not "a chatbot". It is **a workshop for tending a
team of agents** — small, calm, owned by the operator. The brand
needs to feel that way.

---

## Brand pillars (every image must serve at least two)

| Pillar | What the image should *show* |
|---|---|
| **Tactile** | Real materials. Warm wood, paper, brushed brass, cotton, ceramic. You should want to touch it. |
| **Sovereign** | Your hardware, your home, your decisions. Not "the cloud". Not corporate. |
| **Companionable** | The agents have personalities. They're a small team you know by name, not a faceless service. |
| **Crafted** | Made with care. Hand-cut edges. Considered typography. Nothing assembled from a template. |
| **Quietly powerful** | Doesn't shout. The capability is implied through composition and confidence, not through busy visuals. |

---

## Visual signature (shared across all 15)

These are the **constants** that make every image read as the
same brand. The brief for each individual image varies; these do
not.

### Palette

- **Honey / amber** `#D69A4C` — primary warmth, the late-afternoon
  golden tone. Used as a key-light hue.
- **Forest** `#2D4A3A` — deep grounding green. Wall paint, leather,
  shadow base.
- **Paper** `#F1E8D6` — cream off-white. Backgrounds, type plates,
  negative space.
- **Brass** `#8A6A3C` — patina'd metal, brackets, brand bug. Touch
  of richness.
- **Terra** `#C04A2B` — one vivid coral-rust accent. Used ONCE per
  image, never more. The single moment your eye lands on.
- **Ink** `#1A1614` — near-black with warmth. Type, deep shadow.

### Lighting

Soft directional natural light, golden-hour temperature
(~3500K). Long, gentle shadows. One window-shaped light source.
Slightly underexposed mid-tones — confident, not flat. No flash.
No ring lights. No HDR.

### Materials & texture vocabulary

Warm oak or walnut wood. Hand-thrown ceramic. Cream cotton paper
with fibrous edges. Brushed brass. Patina'd copper. Linen cloth.
Vellum. Letterpress impression. Soft graphite. Real machine
keycaps. Brass screws. A subtle film grain or paper grain overlay
on every final image (Kodak Portra 400 character — not "filter",
just texture).

### Typography direction (for any text in images)

- **Display:** a refined humanist serif with slightly idiosyncratic
  italics. Think *Recoleta*, *Söhne Breit*, or *GT Sectra*.
- **Mono:** a warm code typeface for any terminal/code surfaces.
  Think *Berkeley Mono*, *Söhne Mono*, or *JetBrains Mono*.
- **No sans-serif for body** unless the image is meant to feel
  technical. Default to the serif.

### What every image must avoid

- Blue/purple sci-fi gradients.
- Generic AI robot heads, glowing brains, holographic UIs.
- Stock-photo "diverse team smiling at laptop".
- Phone-showing-app-showing-phone screen-in-screen recursion.
- Circuit boards, hexagon patterns, ones-and-zeros backgrounds.
- ChatGPT / OpenAI visual vocabulary (the green orb, the spiral).
- Lens flares, anamorphic streaks, bokeh balls "for vibes".
- 3D-render plastic look. We allow 3D *still-life* (#10) but it
  should read as photographed.

### Style suffix (append to every generator prompt)

> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme depth
> of field, subtle film grain, warm shadows, considered composition,
> brand aesthetic of a small high-craft Italian design studio.*

Keep that line consistent. It is the glue.

---

## Aspect ratio matrix (for the reshape checker)

The image gen models drift on aspect ratio. The reshape checker
should crop-fit to these exact targets after generation.

| Code | Ratio | Pixel target | Used by prompts |
|---|---|---|---|
| `hero` | 16:9 | 1920×1080 | 01, 04, 13 |
| `wide` | 21:9 | 2560×1080 | 05 |
| `og` | 1.91:1 | 1200×630 | 02 |
| `square` | 1:1 | 1024×1024 | 03, 08, 09, 10 |
| `portrait` | 4:5 | 1080×1350 | 11, 12, 15 |
| `phone` | 9:16 | 1080×1920 | 06 |
| `classic` | 4:3 | 1600×1200 | 07 |
| `banner` | 3:1 | 1500×500 | 14 |

The checker reads the `aspect:` field in each prompt's header. If
the generated file's actual ratio is within ±2% of target, accept.
Otherwise, crop with smart-content-aware (center bias) to hit
target exactly. Never stretch.

---

# The fifteen images

---

## 01. The First Light — Master Hero

**aspect:** `hero` (1920×1080)
**use:** Landing page hero, GitHub social preview, repo OG image fallback
**tier:** Primary — if you commission one image, commission this.

### Emotion

The feeling of arriving at your workshop early, before anyone
needs anything from you. Coffee not yet made. The light is just
beginning to reach the workbench. You are about to start
something that is yours. Calm, anticipatory, owned.

### Composition by layer

**Layer 1 — Backdrop:** A warm forest-green wall (`#2D4A3A`),
slightly textured plaster. Soft window light enters from the
upper left, casting a long pale rectangle across the wall and
down onto the bench. The window itself is out of frame.

**Layer 2 — Surface:** A warm-oak workbench fills the lower
third, grain running diagonally toward the upper right. Worn,
loved, with one small dark coffee ring near the front edge —
a real life signal.

**Layer 3 — Subjects (small fleet):** Arranged in a loose
diagonal across the bench, FIVE small ceramic objects, each
distinct, suggesting a small team of agents. They are not
robots — they are small hand-thrown vessels in different
colours of the palette: one honey-glazed, one forest-green, one
brass-banded, one paper-white, one with a single terra-cotta
dot near the rim (this is the eye-catch). Each has a small
white card tag at its base with a handwritten name in
fine-point pen — illegible at distance, but clearly *named*.

**Layer 4 — Foreground:** Out of focus, on the very edge of the
frame, a folded leather notebook with a brass pencil resting on
top. Soft, almost forgotten.

**Layer 5 — Atmosphere:** Visible motes of dust drifting in the
window light. Film grain. The faint warmth of the room.

### Lighting + palette

Single window light, upper-left, golden-hour temperature. Long
soft shadows that fall to the right and forward. Highlights on
the honey-glazed vessel and the brass pencil. The terra-cotta
dot is the single point of vivid colour.

### Material + texture

Plaster wall (matte, slight grain). Oak grain (visible, warm).
Ceramic (subtle satin reflections). Cream paper (fibrous edges).
Brass (slight patina). Leather (worn, soft).

### Avoid

No people. No screens. No text overlays at generation time
(we'll add the wordmark in post). No clutter — fewer than ten
objects total. No bright colours outside the palette.

### Generator prompt (paste-ready)

> A still-life photograph of a warm oak workbench against a deep
> forest-green plaster wall, soft golden-hour window light from
> the upper left casting long gentle shadows. Five small
> hand-thrown ceramic vessels arranged in a loose diagonal across
> the bench — one honey-glazed, one forest-green, one
> brass-banded, one paper-white, one with a single terra-cotta
> dot on its rim. Each vessel has a small handwritten name card
> at its base. A folded leather notebook with a brass pencil
> rests out of focus on the front edge of the frame. Dust motes
> visible in the window light. Composition reads as a small team
> of agents being tended by an unseen, careful operator.
> Photographic, calm, deeply tactile, no people, no screens.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 02. The Open Garden — OG / Social Card

**aspect:** `og` (1200×630)
**use:** Twitter/X share card, LinkedIn preview, generic OG fallback
**tier:** Primary

### Emotion

Welcoming, generous, slightly mysterious. The kind of OG card
that makes someone scrolling past stop for a half-second longer
than they meant to. "What is this." Calm confidence — no
exclamation marks needed.

### Composition by layer

**Layer 1 — Backdrop:** A cream paper background
(`#F1E8D6`), full-frame, with visible long-fibre paper texture.

**Layer 2 — Central object:** A single hand-printed
letterpress-style square card occupies the centre-right of the
frame, slightly off-axis (rotated about 2°). The card is the
honey-amber tone (`#D69A4C`), slightly off-square (because it's
been hand-cut), with deep letterpress impression of the wordmark
**claws** in the display serif. Below the wordmark, smaller,
the line: *a small team of agents, on your own server.*

**Layer 3 — Surround:** To the left of the card, the impression
of a small workshop tableau without showing it whole — the edge
of a ceramic mug, half a brass paperweight, the corner of a
leather notebook. Just enough that the eye fills in the rest.

**Layer 4 — Atmosphere:** A long soft shadow falls from the card
to the lower right, suggesting overhead window light. Subtle
paper grain. One terra-cotta thread lies across the lower-left
corner — the single accent.

### Lighting + palette

Diffuse overhead window light, slightly soft, not harsh. The
shadow on the cream paper is the longest in the composition —
implying time of day, implying calm.

### Material + texture

Mould-made cream paper with deckled edges. Letterpress
impression visible at print depth. Brass (cool brushed). Leather
(soft chestnut). Ceramic (eggshell satin).

### Avoid

No people. No screens. No URL or call-to-action text. No emoji.
The wordmark is the only text. The composition should hold up at
600 px wide thumbnail size — silhouettes must be readable.

### Generator prompt

> A still-life composition photographed straight down onto a
> cream paper background with visible long-fibre texture. Centred
> slightly right and rotated about 2°, a single hand-cut
> honey-amber letterpress card carries the deeply impressed
> wordmark "claws" in a refined humanist serif, with a smaller
> tagline below reading "a small team of agents, on your own
> server." To the left of the card, the partial edges of a
> ceramic mug, a brass paperweight, and a leather notebook
> suggest a small workshop without showing it whole. A long soft
> shadow falls from the card to the lower right, implying
> overhead window light. One terra-cotta thread lies across the
> lower-left corner — the only vivid colour in the frame.
> Composition must read as a confident, generous welcome at
> thumbnail size. No people, no screens, no extra text.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 03. The Mark — Icon / Avatar

**aspect:** `square` (1024×1024)
**use:** GitHub repo avatar, favicon source, app icon, NPM avatar
**tier:** Primary

### Emotion

A small object you would carry in your pocket. Inevitable, like
the icon was always there.

### Composition by layer

**Layer 1 — Field:** Solid honey-amber (`#D69A4C`), with a very
faint vignette toward the corners — almost imperceptible — so
the centre breathes.

**Layer 2 — The mark:** A single ink-impression mark in deep
ink (`#1A1614`), perfectly centred. The mark is a stylised
abstraction of a hand cupped slightly open — not a literal hand,
but the negative-space shape of a palm holding something small
and precious. The mark suggests both a "C" (for claws) AND the
gesture of holding. Drawn with the confidence of a brush stroke
— one motion, no hesitation, slight pressure variation at the
ends. About 60% of the canvas occupied by the mark, generous
margin all around.

**Layer 3 — Impression:** The mark is slightly debossed into the
honey field, with a one-pixel ink halo where the ink bled
slightly past the impression. Tactile.

**Layer 4 — Atmosphere:** A whisper of grain across the field —
the texture of a fine paper viewed through a loupe. Nothing
distracting.

### Lighting + palette

No directional light — it's a pure surface object, like a wax
seal viewed straight on. The vignette is the only lighting cue.

### Material + texture

Pressed honey-coloured wax or thick handmade paper.

### Avoid

No drop shadow. No "shine". No gradient inside the mark itself.
No literal claws. No emoji-style colours. Must read at 16×16 px
as still recognisable — silhouette is everything.

### Generator prompt

> An icon mark presented as a wax-seal impression centred on a
> solid honey-amber field. The mark is a single bold ink stroke
> in deep warm-black that abstracts a slightly-open cupped hand
> seen in profile — readable simultaneously as a "C" letterform
> and as the gesture of holding something small. Drawn with the
> confidence of one brush motion, slight pressure variation at
> the ends, no hesitation. The mark occupies about 60% of the
> canvas with generous breathing room. It is slightly debossed
> into the honey field with a faint ink halo where the ink bled
> a hair past the impression. A whisper of fine paper grain
> across the field, almost imperceptible vignette toward the
> corners. Must remain recognisable at 16-pixel size — the
> silhouette is the entire identity.
> Brand aesthetic of a small high-craft Italian design studio,
> wax seal / letterpress sensibility, tactile and inevitable.

---

## 04. The Operator's Hour — Product Story Hero

**aspect:** `hero` (1920×1080)
**use:** "What does this feel like to use" section on landing page
**tier:** Primary

### Emotion

The single best moment in the product: you've finished setup,
your agents are running, you can step back and just *watch them
work*. Pride without showing off. The feeling of having built
something small and good.

### Composition by layer

**Layer 1 — Room:** A warmly-lit home study. Forest-green wall
behind, oak desk, an open window to the right (out of frame) is
the light source — a low golden afternoon. Visible paper-grain
in the wall plaster.

**Layer 2 — Desk surface:** A real mechanical keyboard with warm
beige keycaps and one terra-cotta accent key (an unmarked
accent — escape, maybe). A ceramic mug of black coffee, half
full, steam still visible. A small brass desk lamp, off (we
don't need it; the window is enough).

**Layer 3 — Subject — operator's hands:** Bottom-centre of the
frame, the operator's hands in repose — NOT typing — resting
flat at the edge of the desk, palms down, wrists relaxed. We
see the hands from the operator's POV looking down. Sleeves
rolled to mid-forearm. One hand wears a simple brass-link
bracelet. Skin tones natural, not airbrushed.

**Layer 4 — Screen:** The laptop screen visible at the top of
the frame, angle suggesting we're looking just past it. Not the
focal point — intentionally slightly out of focus. What's on it
reads as a calm dashboard of small status indicators, mostly
green. The exact UI is illegible by design; we read the *colour*
of "things are working".

**Layer 5 — Atmosphere:** Long warm shadows across the desk.
Steam from the coffee catches the window light. Film grain.

### Lighting + palette

Strong directional window light from the right, low and warm.
The keyboard and mug are key-lit, the wall is in soft fill, the
laptop screen glows just enough to be alive without dominating.

### Material + texture

Oak. Ceramic. Brass. Cotton sleeve. Light catching on warm
beige keycaps with the slightest sheen. Coffee surface — matte,
not reflective.

### Avoid

No face. No detailed UI text (it should be illegibly *calm*, not
a UI screenshot). No "techy" peripherals — no RGB lights, no
gaming gear. The hands are not posed dramatically; they are
just resting.

### Generator prompt

> A warm interior photograph from the operator's first-person
> point of view, looking down past a partially-visible laptop
> screen onto an oak desk in a home study. Forest-green plaster
> wall behind. Strong low golden-hour window light from the right
> casting long warm shadows. On the desk: a mechanical keyboard
> with warm beige keycaps and a single terra-cotta accent key, a
> half-full ceramic mug of black coffee with visible steam, a
> small unlit brass desk lamp. The operator's hands rest flat at
> the front edge of the desk, palms down, wrists relaxed — they
> have just finished and are watching the work happen. Sleeves
> rolled to mid-forearm, a simple brass-link bracelet on one
> wrist, natural skin tones. The laptop screen at the top of the
> frame is intentionally slightly out of focus and shows a calm
> dashboard of small mostly-green status indicators — readable as
> "things are working" without the specific UI being legible.
> Film grain, motes in the light. The feeling is quiet pride in
> having built something small and good. No face visible.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 05. The Quiet Fleet — Multi-Agent Metaphor

**aspect:** `wide` (2560×1080)
**use:** Mid-page wide divider on the landing page, blog headers
**tier:** Secondary — the most cinematic piece

### Emotion

Each of your agents has its own life. Together they make a
small village that you run. The image should feel like an
overhead view of a model train layout someone has loved for
twenty years — small worlds, all in motion, all yours.

### Composition by layer

**Layer 1 — Surface:** Photographed from a high oblique angle
(about 25° down) across a long oak refectory table.
Forest-green wall behind, far end blurred. The grain of the
table runs left-to-right, leading the eye.

**Layer 2 — The fleet:** Five distinct tableaux spaced along
the length of the table, each about a foot apart. Each tableau
is a small grouping of objects suggesting one agent's "job":

- A typewriter ribbon spool, an open envelope, a brass clip
  (the messenger agent)
- A small abacus, a stack of receipts under a brass weight,
  a fountain pen (the accounting agent)
- A propagating jar of cuttings, a misting bottle, a hand
  trowel (the gardener agent)
- A small open notebook with handwritten notes, a magnifying
  loupe (the research agent)
- A radio, an antenna, a coil of warm copper wire (the
  broadcaster agent)

Each tableau small enough to fit in a coffee saucer; together
they form a procession across the table.

**Layer 3 — Labels:** Each tableau has a small cream-paper
nameplate in front of it. Names hand-lettered in dark ink. The
names are warm and personable — *ben*, *sarah*, *john*, *lead*,
*alpha-one*. (These match the agents shown in the README; let's
honour them.) Names legible only on closer inspection.

**Layer 4 — Light:** Side-lit from a window on the left, the
nearest tableau brightest, the far tableaux progressively softer
in the haze of golden light.

**Layer 5 — Atmosphere:** Visible dust in the light. Slight haze
between tableaux suggests depth without explicit blur.

### Lighting + palette

Long side-light from the left, late afternoon. Strong but soft.
Highlights on metal and ceramic across the procession. The
terra-cotta accent appears once — on the propagating-jar tableau
as a small painted clay marker stick.

### Material + texture

Oak, paper, brass, ceramic, copper, leather, cotton thread. Each
small tableau has at least one tactile metal element to catch
the light.

### Avoid

Don't make the objects toy-like or quaint. They are small, but
they are *real* objects — not miniatures. No literal robot
figures. No actual screens in any tableau.

### Generator prompt

> A wide cinematic photograph at a 25-degree high oblique angle
> across the length of a long oak refectory table, golden side
> light streaming from a window at the left, deep forest-green
> wall behind, far end of the table softening into golden haze.
> Five small still-life tableaux are spaced along the table, each
> about a foot apart, each evoking one role of a small team of
> agents: a typewriter ribbon spool with open envelope and brass
> clip; a small wooden abacus with stack of receipts under a
> brass weight and a fountain pen; a propagating jar of green
> cuttings with a small misting bottle and a hand trowel; a small
> open notebook with handwritten notes and a brass magnifying
> loupe; a radio receiver with an antenna and a coil of warm
> copper wire. Each tableau has a small cream-paper nameplate in
> front of it lettered in dark ink with a single name in lower
> case. The nearest tableau is sharply lit and the further
> tableaux fade progressively into warm haze, creating a
> procession effect. Visible dust motes in the light. The objects
> are real-scale, lovingly arranged, not toys. The mood is of a
> small team of agents at rest after a productive day.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 06. The Pocket — "Your Agent on Your Phone" Moment

**aspect:** `phone` (1080×1920)
**use:** Mobile hero, app-store-style portrait card, story format
**tier:** Secondary

### Emotion

The moment you realise your agent is *also* there, in your
pocket, ready when you message it. Not surveillance. Not always
listening. Just — when you reach for it, it's there.

### Composition by layer

**Layer 1 — Backdrop:** A patch of forest-green linen cloth
filling the frame, deep texture visible, slight warp and weft.

**Layer 2 — Pocket:** Across the upper third, the partial view
of a wool waistcoat pocket — the upper opening, the line of
stitching, the worn-soft edge of the fabric. Heritage-tailor
quality. Implies an owner with a sense of self.

**Layer 3 — The phone:** A simple, slightly older smartphone
emerging from the pocket at a relaxed angle, screen on but
showing only the cream-coloured chat thread from a single named
agent (e.g. *sarah*). The chat is illegible by design — we see
the *shape* of a friendly back-and-forth conversation, not the
words. Honey accent on the agent's avatar circle. The phone case
is brushed brass.

**Layer 4 — Hand:** The owner's hand visible from the wrist
down, lightly holding the phone — not gripping. The hand is in
the lower-left, the phone tilts up and to the right. Skin tone
natural, fingernails clean and short.

**Layer 5 — Atmosphere:** Soft warm light from above, paper
grain overlay, almost the texture of a sun-warmed afternoon
photo.

### Lighting + palette

Overhead diffuse warm light. The phone screen contributes a soft
secondary glow upward onto the hand and the pocket edge.

### Material + texture

Linen, wool, brushed brass, skin, cream pixels. The image must
feel touchable — you should sense the weight of the phone and
the give of the cloth.

### Avoid

No detailed UI text. No app icons visible. No emoji avatars. The
phone is not the brand of any real manufacturer (generic
silhouette). No social-media style framing.

### Generator prompt

> A vertical photograph: a hand emerging from the lower-left
> lightly holds a generic smartphone in a brushed-brass case at a
> relaxed upward-right angle. The upper third of the frame shows
> the open pocket of a heritage wool waistcoat in forest green
> with visible stitching. The background is forest-green linen
> cloth with deep visible weave. The phone screen is on, showing
> only a cream-coloured chat thread from a single named agent
> — the conversation reads as warm and friendly without any
> legible text, just the shape of cream message bubbles and a
> small honey-coloured circular avatar. Soft overhead warm
> daylight; the phone screen contributes a faint glow upward onto
> the hand and the pocket edge. The mood is of casual,
> companionable presence — not surveillance, not always-on, just
> available when reached for. Natural skin tones, no faces, no
> brand markings, no readable text.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 07. Say "GO" — Setup Moment

**aspect:** `classic` (1600×1200)
**use:** "Get started" section, install-CTA accompaniment
**tier:** Secondary

### Emotion

The moment between deciding to begin and the thing beginning.
Anticipation. A small ceremony. Like striking a match.

### Composition by layer

**Layer 1 — Backdrop:** A cream paper sheet,
slightly off-square, fills the lower two-thirds of the frame.
The wall behind (top third) is forest green plaster.

**Layer 2 — The instrument:** Centred on the paper, a single
brass key — large, old, ornate but not baroque. About 12 cm
long. Resting with its bow (the handle end) toward the lower
right and the bit (the working end) pointing upper-left. It is
not a literal "key for a lock" — it suggests both *the start* of
something and *something you own*.

**Layer 3 — The instruction:** Above the key, hand-lettered in
sepia ink in the display serif, two characters: **GO**. Just
GO. Letters are about 80% the height of the key's bow. Slight
ink variation suggests a real pen. Not centred on the page —
slightly offset to the right, like a margin note rather than a
title.

**Layer 4 — Marginalia:** In the lower-right corner of the
paper, smaller and lighter, a tiny ink sketch of the same
abstracted cupped-hand mark from the icon (#03). Just a little
brand signature.

**Layer 5 — Atmosphere:** A long soft shadow from the brass key
falls down-and-right, suggesting overhead window light. One
single terra-cotta-coloured ink drop sits below the GO — a
deliberate accident, the one wild element. Subtle paper grain.

### Lighting + palette

Soft overhead window light. The brass catches highlights along
its full length. The cream paper is gently warm.

### Material + texture

Mould-made cream paper, deckle-edged. Brass with age patina,
slight scratches catching the light. Ink with fibre absorbed —
not a printed look.

### Avoid

No literal lock. No countdown timers. No literal "start button"
visual. The mood is calm ceremony, not gamified onboarding.

### Generator prompt

> A still-life photograph viewed mostly from above: a single
> ornate brass skeleton key, about 12 centimetres long, rests
> diagonally on a cream deckle-edged sheet of mould-made paper —
> bow toward lower right, bit pointing upper left. The paper
> fills the lower two-thirds of the frame; a forest-green plaster
> wall fills the upper third. Above the key, hand-lettered in
> sepia ink in a refined humanist serif, two large letters: GO.
> Letters offset slightly right of centre, ink fibre visible, the
> mark of a real pen on paper. In the lower-right corner of the
> paper, a tiny ink sketch of a small abstracted cupped-hand
> mark — the brand signature. One single deliberate
> terra-cotta-coloured ink drop sits below the GO, the one wild
> accent. A long soft shadow from the brass key falls
> down-and-right, implying overhead window light. The brass
> catches gentle highlights along its length; the paper has a
> subtle warm grain. The mood is the calm ceremony of beginning
> — anticipation without urgency.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 08. The Wordmark Plate

**aspect:** `square` (1024×1024)
**use:** Press kit, README header, slide decks, type studies
**tier:** Brand asset

### Emotion

The official wordmark, presented like a museum object. Reverent,
calm, definitive.

### Composition by layer

**Layer 1 — Field:** Pure cream paper (`#F1E8D6`) filling the
frame, with fine long-fibre visible texture and one or two
microscopic bits of botanical inclusion in the pulp. Warm
neutral.

**Layer 2 — Letterpress impression:** Centred horizontally, set
in the lower half of the frame, the wordmark **claws** in deep
ink (`#1A1614`) in the refined humanist serif, all lowercase.
Letterpress impression depth clearly visible — you can see the
edges where the type bit into the paper.

**Layer 3 — Beneath:** Set in mono, in small caps, in a soft
grey-brown, two centimetres below the wordmark:
**A SMALL TEAM OF AGENTS, ON YOUR OWN SERVER.** Quietly
typographic, generous letterspacing, the kind of tagline that
would survive being misquoted because it is true.

**Layer 4 — Mark:** Above the wordmark, in the upper third,
centred, the cupped-hand brand mark from #03 — printed smaller,
about 80 px equivalent height, in the honey amber tone. Looks
like it has been pressed into the same sheet of paper at a
different time.

**Layer 5 — Atmosphere:** Long soft shadow falling
down-and-right across the entire sheet, implying a single
overhead light. Real paper grain.

### Lighting + palette

Overhead diffuse, but with directional bias from upper-right.
The letterpress impression catches micro-shadow on its left
edges.

### Material + texture

Heavy cream cotton paper, letterpress impression at depth.

### Avoid

No frame around the wordmark. No background pattern. No
illustration competing with the type. The wordmark is the only
subject.

### Generator prompt

> A square photograph of a museum-quality letterpress print on
> heavy cream cotton paper with visible long-fibre texture. The
> wordmark "claws" is set in the lower half in lowercase deep
> warm-black ink in a refined humanist serif, pressed deeply into
> the paper so the impression depth is clearly visible at the
> letter edges. Two centimetres below, in small caps in a soft
> grey-brown mono typeface with generous letterspacing, the
> tagline: A SMALL TEAM OF AGENTS, ON YOUR OWN SERVER. In the
> upper third, centred, a small honey-amber printed mark of an
> abstracted cupped hand — the brand bug — printed as if pressed
> in a separate pass. Overhead diffuse light with a directional
> bias from upper right catches micro-shadow on the left edges of
> the impressed type. The whole sheet has the calm dignity of a
> press kit object. No frame, no decoration, no background
> pattern — the type is the only subject.
> Brand aesthetic of a small high-craft Italian design studio,
> letterpress sensibility, reverent and tactile.

---

## 09. The Sealed Letter — Distribution Mark

**aspect:** `square` (1024×1024)
**use:** Release announcements, "v1.6.X shipped" social posts
**tier:** Brand asset / utility

### Emotion

Old-world correspondence. Each release feels like a real letter
sealed by hand and sent into the world. Care taken; sender
proud.

### Composition by layer

**Layer 1 — Backdrop:** A folded cream envelope photographed
straight-on, filling the frame. Slight age. Slightly
asymmetrical fold suggests handwork.

**Layer 2 — The seal:** Centred where the envelope's flap meets
its body, a wax seal in the honey-amber palette colour,
impressed with the cupped-hand brand mark from #03. The wax has
realistic dimensionality — slight shine, slight cracking at the
edge, droplet shape suggesting it was poured, not cast.

**Layer 3 — The address:** Above the seal, hand-lettered in
dark ink in the display serif, the addressee block looks like a
real letter would: a name (we'll mask with three short ink-blot
shapes since this is a *template* image), a city (similarly
masked), and at the bottom only a single visible
characters-line that reads: **v.____.____.____**. The blanks are
ink-lines like a fill-in form. This is the brand artefact for
versioned release posts; the version is composited in later.

**Layer 4 — Stamps:** In the upper right corner, a small honey
postage stamp showing the cupped-hand mark inside a fine border,
slightly tilted. Cancelled with a faint date-circle ink stamp.

**Layer 5 — Atmosphere:** Soft overhead daylight, gentle
shadow from the wax seal to the lower right, subtle paper grain,
one single terra-cotta thread on the lower edge.

### Lighting + palette

Diffuse overhead window light, the wax seal catching a quiet
highlight along its top edge.

### Material + texture

Cream envelope paper with fold creases. Honey wax with slight
sheen. Ink absorbed into paper. Postage-stamp perforations
realistic at edges.

### Avoid

No live-style "shipped!" overlays. No emoji. No actual real
addresses or names. The letter is a *template* — the version
goes in later.

### Generator prompt

> A square photograph of a folded cream envelope photographed
> straight-on, filling the frame, with a slight asymmetric fold
> suggesting handwork. A honey-amber wax seal is centred where
> the envelope's flap meets its body, freshly poured with
> realistic dimensional thickness, faint cracking at the edges,
> and a clean impression of an abstracted cupped-hand brand mark.
> Above the seal, an addressee block hand-lettered in dark ink in
> a refined humanist serif — the name and city lines are masked
> by three short ink-blot shapes each (this is a template; the
> details will be composited in later). The version line below
> reads "v.____.____.____", with the blanks rendered as ink-lines
> like a fill-in form. In the upper right, a small honey-amber
> postage stamp showing the same brand mark inside a fine border,
> slightly tilted, cancelled with a faint date-circle ink stamp.
> Soft overhead daylight, a gentle shadow falling from the wax
> seal to the lower right, subtle paper grain, one single
> terra-cotta thread lying on the lower edge of the envelope as
> the one accent. The mood is old-world correspondence —
> care-taken, sender-proud, calm ceremony.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 10. The Tile — Repeating Brand Pattern

**aspect:** `square` (1024×1024)
**use:** Tiled backgrounds, decorative dividers, footer fills, repo wallpaper
**tier:** Brand asset

### Emotion

A textile pattern your grandmother might have woven, except it
hides the cupped-hand mark in its rhythm. Calm, repeatable,
made-by-hand.

### Composition by layer

**Layer 1 — Base:** A weaving of fine cream cotton thread,
photographed top-down so the warp and weft are crisply visible.
Slightly underexposed for a calm mood.

**Layer 2 — Pattern:** Across the field, a sparse repeating
diamond grid embroidered in honey thread — each diamond about
40 px on a side equivalent, regularly spaced. The grid implies
order but does not dominate.

**Layer 3 — Hidden mark:** At regular intervals — say every
fourth diamond — instead of a diamond, the small cupped-hand
brand mark is embroidered. Same honey thread. Same scale. Reads
as decoration unless you look for it.

**Layer 4 — Accent:** Across one diagonal of the tile, a single
terra-cotta thread runs from one corner to another. Just one.
This is the heart-flicker.

**Layer 5 — Atmosphere:** The fibres are real; you can count
threads if you lean in. No filter look.

### Tiling note

Critical: the four edges of the tile must meet seamlessly so
this can be CSS-tiled. The embroidered pattern must align across
seams. The terra-cotta diagonal must also continue cleanly when
tiled — if it enters the lower-right edge at coordinate X, it
must exit the upper-left edge at coordinate X of the next tile.

### Lighting + palette

Flat overhead diffuse light. No directional shadows that would
break the tiling illusion.

### Material + texture

Hand-woven cotton, embroidered honey-coloured cotton thread, one
terra-cotta cotton thread.

### Avoid

No directional shadows. No vignette. No composition focal point
— this is a *field*, not an *image*.

### Generator prompt

> A seamless tileable photograph of hand-woven cream cotton
> textile photographed straight-down with flat overhead diffuse
> light and zero directional shadow. Visible warp and weft, fine
> long fibres, slightly underexposed. Embroidered across the
> field in honey-amber cotton thread, a sparse regular diamond
> grid — diamonds about forty pixels on a side, evenly spaced. At
> every fourth diamond position, instead of a diamond, an
> embroidered small abstracted cupped-hand brand mark in the same
> honey thread. A single terra-cotta cotton thread runs one
> diagonal of the image as the only vivid accent. The four edges
> of the tile must align seamlessly when CSS-tiled: the
> embroidered grid must continue across seams, and the
> terra-cotta diagonal must exit one corner at the exact
> coordinate it enters the opposite corner. No focal point, no
> directional shadow, no vignette — this is a textile field, not
> a composed image. Tactile, calm, made-by-hand.
> Brand aesthetic of a small high-craft Italian design studio,
> textile sensibility, intended for repeating use as a website
> background.

---

## 11. The Palette — Material Study

**aspect:** `portrait` (1080×1350)
**use:** Style guide page, "brand" landing-page section, press kit page
**tier:** Brand asset

### Emotion

A calm reference card. Like the pantone-chip page in a careful
designer's notebook. Definitive without being clinical.

### Composition by layer

**Layer 1 — Surface:** Cream paper background filling the frame,
visible texture.

**Layer 2 — Swatches:** Six rectangular material swatches
arranged in a 2×3 grid in the lower two-thirds of the frame —
honey, forest, paper, brass, terra, ink — each swatch about 280
px square, with generous spacing. Each swatch is a real
*material*, not a flat colour:

- **Honey** — a square of honey-coloured beeswax with a
  fingerprint impressed in one corner
- **Forest** — a square of forest-green wool felt with one frayed
  edge
- **Paper** — a square of cream paper torn slightly at one
  corner, revealing the under-layer
- **Brass** — a square of brushed brass with a single arc-shaped
  scratch
- **Terra** — a small terracotta tile with one corner glazed
- **Ink** — a square of ink-stained handmade paper, the ink dried
  with a clear meniscus edge

**Layer 3 — Labels:** Beneath each swatch, in mono small caps,
the colour name and the hex code printed in soft brown ink. e.g.
**HONEY · D69A4C**. Small, secondary — the swatches are the
heroes.

**Layer 4 — Header:** In the upper sixth of the frame,
hand-lettered in the display serif: *palette*. Lowercase,
modest, not centred — placed in a margin position.

**Layer 5 — Atmosphere:** Soft directional overhead light, paper
grain, one terra-cotta thread lying across the lower right
outside the swatch grid — the off-grid accent.

### Lighting + palette

Diffuse overhead, slight bias from upper right. Each swatch
catches its own appropriate light response — wax glistens, felt
absorbs, brass reflects, terracotta is matte.

### Material + texture

Six real materials, photographed not rendered.

### Avoid

No flat-colour rectangles. No CMYK / RGB callouts. No
overly-bright lighting — the swatches must feel honest.

### Generator prompt

> A portrait-orientation photograph of a brand palette reference
> card, presented as a careful designer's notebook page on cream
> paper with visible texture. In the lower two-thirds, six real
> material swatches arranged in a two-column three-row grid with
> generous spacing: a square of honey-coloured beeswax with a
> fingerprint impressed in one corner; a square of forest-green
> wool felt with one frayed edge; a square of cream paper torn
> slightly at one corner; a square of brushed brass with a single
> arc-shaped scratch; a small terracotta tile with one corner
> glazed; a square of handmade paper stained with deep warm-black
> ink with a clear dried meniscus edge. Beneath each swatch, in
> small caps soft-brown ink mono type, the colour name and hex
> code — HONEY · D69A4C, FOREST · 2D4A3A, PAPER · F1E8D6,
> BRASS · 8A6A3C, TERRA · C04A2B, INK · 1A1614. In the upper
> margin, hand-lettered in a refined humanist serif: *palette*.
> Soft directional overhead daylight from the upper right, each
> material responding appropriately — wax glistens, felt absorbs,
> brass reflects, terracotta is matte. One terra-cotta thread
> lies on the page outside the swatch grid, the off-grid accent.
> The mood is reference-grade, calm, honest — definitive without
> being clinical.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 12. The Workshop — Craft / Values

**aspect:** `portrait` (1080×1350)
**use:** "About" page on landing site, ethos section
**tier:** Tertiary

### Emotion

The room where this software is made. Honest labour. The
operator's hand cares about what it makes. Not romanticised
craftsmanship — actual cluttered work.

### Composition by layer

**Layer 1 — Backdrop:** A warm wood-panelled wall with one
hand-pinned cork-board, on which a few hand-drawn diagrams of
the agent fleet (the diagonal procession from #05) are pinned
with brass tacks.

**Layer 2 — Bench:** A workbench surface in the lower two-thirds
of the frame, oak, scarred from use.

**Layer 3 — Tools and parts:** Across the bench: an open laptop
turned slightly to the right (screen showing a calm
illegibly-green terminal — same energy as #04 but more
work-in-progress), a leather notebook open to a page of
handwritten architecture sketches, a brass T-square, two ceramic
mugs (one with cold coffee, one with pencils), a small pile of
brass-clipped paper cards each handwritten with an agent name, a
soldering iron resting on a brass stand (we don't actually
solder, but the *image* of capable hands matters), and one piece
of warm honey-coloured electronic component on the table — a
small brass-cased thing, mysterious and clearly hand-made.

**Layer 4 — Operator's hand:** From the right edge of the frame,
the operator's hand in motion — pen in fingers, mid-sketch on
the notebook page. Not posed; caught mid-thought.

**Layer 5 — Atmosphere:** Late-afternoon light from the left,
working clutter, faint warmth, subtle paper grain. Steam from
the pencil-mug suggests it was used recently for tea.

### Lighting + palette

Strong directional light from the left, low. Long shadows from
every object. The screen contributes a small green secondary
glow only on the laptop bezel — not enough to compete.

### Material + texture

Oak, leather, ceramic, brass, paper, cotton — the full
vocabulary. This is the image where the brand's materials live
together at once.

### Avoid

No "developer in plaid shirt" stereotype. No backlit
silhouette. No Edison bulbs. No succulents. The clutter is
*work*, not aesthetic.

### Generator prompt

> A portrait-orientation photograph from a slightly oblique angle
> across a heavily-used oak workbench in a wood-panelled
> workshop. Strong directional low afternoon light from the left
> casting long warm shadows. On the wall behind, a pinned
> cork-board displays a few hand-drawn architectural diagrams of
> a small fleet of agents arranged in a diagonal procession, held
> with brass tacks. On the bench: an open laptop turned slightly
> right showing a calm illegibly-green terminal interface; a
> leather notebook open to a page of hand-sketched architecture
> diagrams; a brass T-square; two ceramic mugs, one with cold
> coffee and one stuffed with pencils, faint steam rising from
> the pencil-mug; a small pile of brass-clipped cream cards each
> handwritten with a single agent name; a soldering iron resting
> on a brass stand; one mysterious small honey-amber brass-cased
> electronic component sitting in centre frame, clearly
> hand-made. From the right edge, the operator's hand enters
> mid-motion holding a pen, caught mid-sketch on the notebook
> page — not posed, working. The mood is honest cluttered labour,
> not romanticised craft. Subtle film grain.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 13. The Quiet Server — Sovereignty

**aspect:** `hero` (1920×1080)
**use:** "Runs on your hardware" section, security/privacy claims
**tier:** Tertiary

### Emotion

The opposite of a cloud datacentre photograph. A small,
dignified piece of hardware in a domestic setting. Calm,
inhabited, *yours*. Not a server farm; one warm machine.

### Composition by layer

**Layer 1 — Room:** A corner of a domestic room — bookshelf to
the left holding old books and one or two ceramic objects from
elsewhere in the brand, forest-green plaster wall, a small
wooden side table.

**Layer 2 — Subject — the server:** Centred on the side table, a
small server in a warm wooden enclosure (we have made the
chassis ourselves; this is not an off-the-shelf rack unit).
About the size of a hardback novel standing on end. Brass
ventilation grille on the front. One small honey-amber LED
glowing softly. A leather pull-handle on the top. Looks like an
object you might inherit, not buy.

**Layer 3 — Cables:** Two thin cotton-braided cables descend
from the back of the enclosure, falling gracefully behind the
table. Cream-coloured braid with one terra-cotta thread woven
into them — the one accent.

**Layer 4 — Companion objects:** Beside the server on the
table: a small ceramic bowl of brass paperclips, a folded piece
of cream paper with handwritten setup notes, a worn leather
luggage tag tied to the server's pull-handle with twine — the
tag is hand-lettered with a single name in the display serif
(we'll say *workshop* or similar).

**Layer 5 — Atmosphere:** Late-evening warm interior light from
a lamp out of frame to the right. Long soft shadows. Faint
visible wisp of warm steam from a cup on the lower-left edge of
the frame.

### Lighting + palette

Single warm lamp, low and to the right. The room reads as
inhabited, lived-in. The honey-amber LED is the smallest light
source but draws the eye.

### Material + texture

Walnut wood chassis, brass grille, cotton-braided cable, leather
tag, ceramic bowl, paper, plaster.

### Avoid

No rack mount. No server room. No black metal cases. No racks
of cables. No cooling fan whine implied. The server should look
like furniture you could explain to a stranger.

### Generator prompt

> A wide horizontal interior photograph: a small custom server
> built into a hand-made walnut chassis about the size of a
> standing hardback novel, with a brass ventilation grille on its
> front face and one small softly-glowing honey-amber LED, rests
> on a small wooden side table in a domestic room. A leather
> luggage tag tied to the server's top handle with twine is
> hand-lettered with a single name in a refined humanist serif. A
> bookshelf with old books and one or two ceramic objects sits to
> the left, a forest-green plaster wall fills the background. Two
> thin cotton-braided cables in cream with one terra-cotta thread
> woven through descend from the back of the enclosure and fall
> behind the table. Beside the server on the table: a ceramic
> bowl of brass paperclips, a folded cream-paper sheet of
> hand-written setup notes. Late-evening warm lamp light from the
> right out of frame, long soft shadows, a wisp of steam from a
> cup at the lower-left edge of the frame. The whole composition
> reads as a beloved domestic object that happens to run a small
> team of agents — sovereignty as inheritance, not enterprise.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 14. The Open Door — Community / OSS

**aspect:** `banner` (1500×500)
**use:** GitHub repo banner, CONTRIBUTING.md header, community page
**tier:** Tertiary

### Emotion

The door of a small studio that is open during working hours.
You are welcome to come in. No-one will perform welcome — the
door is just open.

### Composition by layer

**Layer 1 — Architecture:** A horizontal panoramic view across
the threshold of a small studio. From left to right: about a
third of the frame is the warm wooden interior of a workshop
seen through an open door; the middle third is the open
doorway itself; the right third is the cobbled street outside.

**Layer 2 — The door:** The door is warm wood, painted forest
green on the street side, paper-coloured on the interior side.
It is held open by a heavy brass weight on the floor. A small
brass plaque mounted at eye-height on the doorpost reads
**claws** in the display serif and below it, in mono small caps:
**OPEN-SOURCE STUDIO · ENTER WITHOUT KNOCKING**.

**Layer 3 — The interior glimpse:** Through the open door, on
the left third of the image, the suggestion of a small
workshop — a single workbench, the back of someone in a cotton
shirt working at it (we see only the back of one shoulder, no
face). A honey-amber pendant lamp glows above the bench.

**Layer 4 — The street:** On the right third, the cobbled
street fades into gentle afternoon light. A small chalkboard
sign leans against the wall outside, hand-lettered: **today:
the team are working on better setup**. Casual, honest, a
real shop.

**Layer 5 — Atmosphere:** Late-afternoon directional light from
the right, washing across the threshold. The brass plaque
catches highlight. One terra-cotta flower in a small ceramic pot
sits beside the doorway — the accent.

### Lighting + palette

Afternoon golden light from the right (street side), the
interior of the studio illuminated by its own warm pendant
lamp — two warm light sources at slightly different temperatures
meet at the threshold.

### Material + texture

Wood, brass, cobblestone, ceramic, cotton, chalk.

### Avoid

No corporate "We're hiring" energy. No marketing copy. No
"diverse team smiling at camera". The shop is open; the work is
what's important.

### Generator prompt

> A long horizontal panoramic photograph spanning the threshold
> of a small open-source software studio: roughly the left third
> shows the warm wooden interior of a workshop seen through an
> open door, the middle third is the open doorway, the right
> third is a sunlit cobbled street outside. The wooden door is
> painted forest green on the street side and cream on the
> interior side, held open by a heavy brass floor weight. A small
> brass plaque mounted at eye-height on the doorpost reads
> "claws" in a refined humanist serif with a smaller mono small
> caps line below reading "OPEN-SOURCE STUDIO · ENTER WITHOUT
> KNOCKING." Through the doorway, the suggestion of a single
> workbench with the back of one figure in a cotton shirt
> working — only one shoulder visible, no face — under a glowing
> honey-amber pendant lamp. On the street side, a chalkboard sign
> leans against the wall hand-lettered "today: the team are
> working on better setup." A small ceramic pot beside the
> doorway holds a single terra-cotta flower as the one vivid
> accent. Late-afternoon golden directional light from the right
> washes across the threshold; the interior is warmed by its own
> pendant lamp. The image reads as a welcoming open shop, no
> performance of welcome — just the door open and the work
> visible.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

## 15. The Evening Watch — Companionship

**aspect:** `portrait` (1080×1350)
**use:** "A friend that does the work" emotional anchor, footer image, blog headers
**tier:** Tertiary

### Emotion

It's late. The work is done for today. Your team of agents is
still gently humming. You are not anxious about it. You sit
with a book. The room is warm. The image should feel like the
end of a good day.

### Composition by layer

**Layer 1 — Room:** A reading nook at evening. Forest-green
walls, warm lamp light, an armchair upholstered in honey-coloured
linen. A patterned cream rug (subtly carrying the tile pattern
from #10).

**Layer 2 — Subject — owner's lap & book:** In the lower
foreground, from the owner's first-person perspective, an open
book rests across crossed legs. The book is hardcover, bound in
deep forest cloth. We see the page edges and the owner's right
hand turning a page with the kind of paper-affection only real
readers have.

**Layer 3 — The companion shelf:** Just behind and to the right
of the armchair, a small dedicated shelf of three or four warmly
glowing little objects — each is a small ceramic vessel like in
#01, each has one tiny honey-amber LED inside that glows softly
through translucent glaze. These are the agents at rest. They
are not screens; they are presence. Each has its small name card
at its base, hand-lettered.

**Layer 4 — Window:** Off to the right, a window with the
soft-violet of late twilight just visible behind warm sheer
curtains. Outside is cold; inside is warm.

**Layer 5 — Atmosphere:** A single brass lamp on a side table
casts the dominant light. A ceramic cup of tea on the side
table beside the lamp catches a quiet highlight. The terra-cotta
accent is one bookmark ribbon hanging from the closed end of the
book — the only vivid colour.

### Lighting + palette

Warm interior tungsten-equivalent lamp light. Mood is amber and
forest with the cool twilight just visible at the window as
contrast.

### Material + texture

Linen, wool rug, hardcover cloth binding, ceramic, paper,
brass, sheer cotton curtain.

### Avoid

No phone in the image. No laptop. The agents are present as
*presence*, not as devices. No fireplace cliché. No wine glass.
The book is real reading, not a prop.

### Generator prompt

> A vertical first-person photograph from a reader's perspective
> in an evening reading nook. The owner sits in an armchair
> upholstered in honey-coloured linen; we see the owner's lap and
> crossed legs in the lower foreground, with an open hardcover
> book bound in deep forest cloth resting across them. The right
> hand mid-page-turn, paper-affection evident. A single bookmark
> ribbon in terra-cotta hangs from the back of the book — the
> only vivid accent. Forest-green walls, a patterned cream rug on
> the floor. Behind and to the right of the armchair, a small
> dedicated shelf holds three or four small hand-thrown ceramic
> vessels of the brand palette, each with a soft honey-amber LED
> glowing through translucent glaze — these are the operator's
> agents at rest, presence without screens. Each vessel has a
> small hand-lettered name card at its base. Off to the right, a
> window shows soft cold twilight behind warm sheer cotton
> curtains. A single brass lamp on a side table casts the
> dominant warm light; a ceramic cup of tea beside the lamp
> catches a quiet highlight. The mood is the end of a good day —
> the team is gently humming, the work is done, the room is
> warm. No phone, no laptop, no screens.
> *Shot on Hasselblad medium format, Kodak Portra 400 film stock,
> natural window light at golden hour, shallow but not extreme
> depth of field, subtle film grain, warm shadows, considered
> composition, brand aesthetic of a small high-craft Italian
> design studio.*

---

# Notes for the reshape checker

When the image arrives back from the generator:

1. **Read this file**, find the prompt by its index (01–15), look up
   the `aspect:` value.
2. **Look up the target pixel dimensions** in the aspect ratio
   matrix near the top of this file.
3. **Measure the generated image.** If the actual ratio is within
   ±2% of target, accept as-is and only resize to target px (using
   high-quality Lanczos).
4. **If the ratio is off**, smart-crop with **centre bias** to hit
   the target ratio, then resize. Never stretch (we lose the brand
   feel immediately).
5. **For tile #10**, the seamless-edge property MUST be verified.
   The checker should rotate and tile the result 2×2 in memory and
   look for visible seams before accepting. If seam visible, reject
   and re-generate.
6. **For wordmark #08 and seal #09**, run an OCR pass to confirm
   the wordmark "claws" or the tagline letters are spelled
   correctly. Image gen models corrupt typography constantly; this
   guard is essential.
7. **Reject if any of the avoid-list constants is present** — e.g.
   if a face is detected in 01/04/06/12/15 (we explicitly do not
   want faces in those), reject and re-generate.

The output of the checker on accept should write:

```
images/01-the-first-light.jpg
images/02-the-open-garden.jpg
...
images/15-the-evening-watch.jpg
```

Keep names in lower-case-kebab matching the title here.

---

# What I'd recommend if you only commission a subset

**Five images** (minimum viable brand):
01 (hero), 02 (OG), 03 (icon), 08 (wordmark), 09 (seal).

**Eight images** (full launch pack):
add 04 (operator's hour), 05 (fleet), 11 (palette).

**All fifteen** (complete brand library):
the remaining seven round out community, sovereignty, and
emotional anchors for blog content and ongoing marketing.

If a generator struggles with the still-life realism (some
models are weak at hand-thrown ceramics), commission #01 and
#04 to a real photographer instead of a generator and lean on
the generators for the more graphic pieces (#02, #03, #08, #09,
#10). Mixing real photography with generated illustration is
fine if the **style suffix** is honoured everywhere — that's
what holds the brand together.
