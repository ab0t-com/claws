# claws — brand imagery prompts (v3 — startup confident)

Calibrated between v1 (tactile photographic, Italian craft studio)
and v2 (Sanrio kawaii). Same 15 use cases, same reshape-checker
contract. **The theme survives — friendly, agent-team, sovereign,
crafted — but the execution grows up.**

If you only commission five: **01, 02, 03, 04, 09**. Same as v1 and v2.

---

## What we're going for (mood reference)

**Yes:**
- Linear's illustration system — geometric clarity, confident
  restraint, two-tone compositions
- Notion's mature illustration era (post-Alegria) — clean shapes,
  considered colour, occasional character
- Vercel's spot illustrations — refined, premium, deliberately
  flat
- Stripe's two-tone illustrations (the deeper-palette ones, not
  the gradient blob ones)
- Arc Browser's marketing — colourful but adult
- Pitch.com hero illustrations
- Cron (now Notion Calendar) launch art
- Replit ship-week hero panels
- Modern fintech illustration (Mercury, Wise)
- Mailchimp's mature era (the line-and-fill spots, not the
  whimsy era)

**No:**
- Sanrio / LINE Friends cuteness — we are pulling away from this
- Cheek blushes, sleeping-Z bubbles, tongue-out faces — no
- "Big head, tiny body" kawaii proportions — proportions become
  more balanced
- Goofy "wow" expressions on the mascot — neutral or quietly
  confident is the default
- Photorealism (kept from v2 — still flat vector / illustration)
- Heavy gradients, chrome, glow effects
- Corporate stock illustration "people in flat colour" (Alegria
  is still dead)
- Generic "abstract tech" — no orbiting nodes, no circuit
  pathways, no nodes-and-edges diagrams

The product is the same: a small team of AI agents on your own
server, tended by one person. v3 says **we ship serious
software** without sacrificing the warmth.

---

## The mascot — matured

We keep one mascot across most images. Same brand role as v2,
but executed with restraint.

**What changes from v2:**

| v2 (kawaii) | v3 (confident) |
|---|---|
| Round bean body, head ~60% of figure | More balanced 50/50 head-to-body ratio |
| Two cat-adjacent ear nubs | **No ear nubs** — pure round head |
| Big dot eyes (peppercorn-sized), wide-set | Smaller dot eyes (lentil-sized), naturally spaced |
| Cheek blush ovals always present | **No cheek blush, ever** |
| Default smile (upward curve) | **No default expression** — neutral. Smile only on celebration images (01 hero greeting, 02 OG, 07 GO button) |
| 4px contour line | 3px contour line — slightly more refined |
| Standing slightly squat | Standing more upright, posture confident |

**What stays the same:**

- Round head + simple body
- Two prominent paw-mittens — these are the brand
- Three tiny pad-circles on the underside of each paw
- Cream colour by default
- Cocoa-brown line work (not black)
- Hand-touched line quality (slight wobble, not vector-perfect)

**Personality dial:** v2's mascot read as *delighted*. v3's reads
as *competent and at ease*. Like the difference between a soft
toy and a thoughtful colleague who happens to be small and round.

---

## Visual signature (every image)

### Palette

Six colours plus one accent. Fewer than v2 by design — corporate
restraint comes from saying no to colours.

- **Cream** `#F8F1E4` — base backgrounds, mascot body
- **Forest** `#3A5A47` — anchor; serious, grounded, the
  "we mean it" colour
- **Navy** `#2C3E50` — second anchor; calm authority
- **Peach** `#F4A77E` — warm primary, more refined than v2's peach
- **Butter** `#F4D67A` — confident highlight, sun, paw-pads
- **Cocoa** `#4A3528` — line work and type (replaces black,
  same role as v2)

**Hero accent (use ONCE per image):** **Terra** `#D86545` — a
confident coral, less candy than v2's cherry-red. This is the
single deliberate eye-catch.

**Hard rule:** Use **2–3 colours per image** plus the cocoa line.
Never more than four total. Restraint is the v3 signature.

### Line work

- **All contours in cocoa** (`#4A3528`), never black.
- Line weight ≈ 3 px equivalent (down from v2's 4 px) — more
  refined, slightly more "designed".
- Consistent thickness — no calligraphic taper.
- Interior details at 1.5 px — half the contour weight.
- Slight imperfection in the line still — a few-pixel wobble so
  it reads as drawn, not vector-perfect. But the wobble is more
  controlled than v2.

### Shading

- **No gradients on objects.** Flat fills only.
- **Allowed:** one gentle background gradient — cream to peach,
  or forest to navy at the top of a frame. Single stop, soft.
- **Allowed:** subtle 80%-opacity drop shadow on the mascot and
  on key objects. Offset 4–6 px, soft 8 px feather. Not the
  hard cocoa offset shape from v2. More like a Linear-style
  whisper of a shadow.
- **Forbidden:** gloss, bevels, inner glow, light rays beyond a
  simple sun rendition.

### Typography

- **Display:** a confident humanist sans with mild personality —
  Inter Tight Bold, Söhne Halbfett, GT America, or Recoleta when
  serif warmth fits. **Sentence case by default**, not lowercase
  everywhere (v2 was all-lowercase; v3 uses appropriate case).
- **Mono (used for technical surfaces — version line, code-look
  details):** clean technical mono — Berkeley Mono, JetBrains
  Mono, IBM Plex Mono.
- **Type colour:** cocoa (`#4A3528`) by default. Forest or navy
  acceptable when the composition calls for it.
- **No italics in body, no all-caps screaming, no condensed.**
- **Tagline lockup** uses the display sans in **Medium** weight,
  not bold. Generous letterspacing on the tagline.

### Background motifs — restrained

Pick **at most one** per image (down from v2's "1–2"):

- A single subtle gradient sky (cream → peach, navy → forest)
- A geometric grid of cream dots (8 px dots, generous spacing)
  — barely visible texture, not a foreground element
- One stylised sun (flat circle, no rays, just shape)
- One stylised cloud (a single soft peach or cream cloud, NOT
  multiple)
- Generous **negative space** — preferred over any motif

**No:** scattered sparkles, no twinkles, no floating hearts, no
4-point stars. Those were v2 signatures. v3 trusts the
composition.

### Style suffix (append to every generator prompt)

> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

That suffix is the glue.

---

## Aspect ratio matrix (for the reshape checker)

Identical to v1 and v2. The checker reuses its logic; only
content changes.

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

Reshape rules: ±2% tolerance, smart-crop with centre bias,
Lanczos resize, never stretch. See v1 file for the full checker
contract.

---

# The fifteen images

---

## 01. Open Workshop — Master Hero

**aspect:** `hero` (1920×1080)
**use:** Landing page hero, GitHub social preview, repo OG image fallback
**tier:** Primary

### Mood

A considered tableau, viewed at eye-level. The mascot stands at
the threshold of its workshop, calm and ready. The team is
present but in the background, settled into their stations. The
viewer should feel: *this is a place where serious work gets
done by someone who cares about the craft*.

### Composition by layer

**Layer 1 — Backdrop:** A single soft cream-to-peach vertical
gradient across the upper two-thirds. **No clouds, no sun, no
sparkles.** Just space.

**Layer 2 — Floor/desk plane:** A flat-fill forest-green
horizontal band fills the lower third. The transition between
peach gradient and forest band is the horizon — a single 3 px
cocoa line.

**Layer 3 — The workshop architecture:** Centre-left of frame,
a simple architectural suggestion of an open studio — three
clean cream rectangles indicating a back wall, a workbench
surface, and a doorway, with strong geometric refinement. Two
small navy hanging pendant shapes above the workbench (just the
silhouette, no detail). The whole architecture occupies about
40% of the canvas width.

**Layer 4 — The mascot:** Centre-stage on the forest floor,
slightly off-centre toward the right of the architecture. The
cream mascot stands upright, both paws relaxed at its sides.
Neutral expression — small dot eyes, no smile, no blush. One
paw subtly raised in a small, dignified acknowledgement (not a
wave). It conveys *welcome* without performing welcome.

**Layer 5 — The team — quiet presence:** Inside the workshop, on
the workbench, three smaller mascot silhouettes face away from
the viewer, occupied with their stations. Each in a different
palette colour: one peach, one butter, one navy. They are
slightly desaturated by being in the architectural midground.
Each has a small cream paper card on the bench in front of it —
no readable name needed; the card is the gesture.

**Layer 6 — Accent:** A single terra-cotta coral object on the
workbench between the team members — a small flat-fill round
shape (a ceramic pot, abstractly). The one vivid moment of the
composition.

**Layer 7 — Atmosphere:** A single subtle 80%-opacity soft drop
shadow under the main mascot. No other shadows.

### Avoid

No humans. No detailed laptops or screens. No clouds or
sparkles. No labels or text in image. The architecture is
suggested by geometry, not detailed.

### Generator prompt

> A flat vector horizontal hero illustration with refined startup
> aesthetic. Upper two-thirds: a soft cream-to-peach vertical
> gradient with generous empty space — no clouds, no sun, no
> sparkles. Lower third: a flat forest-green floor band, the
> horizon a single 3-pixel cocoa-brown line. Centre-left of
> frame, an architectural suggestion of an open studio in three
> clean cream rectangles — back wall, workbench surface, doorway
> — with strong geometric refinement. Two small navy pendant-lamp
> silhouettes hang above the workbench. The architecture occupies
> about 40% of canvas width. Centre-stage on the forest floor,
> slightly to the right of the architecture, the cream mascot
> stands upright: round head with no ear nubs, two small lentil-
> sized dot eyes in cocoa, neutral expression with no smile and
> no cheek blush, two prominent paw-mittens. One paw subtly
> raised in a small dignified acknowledgement — welcoming without
> performing welcome. Inside the workshop on the workbench, three
> smaller mascot silhouettes face away from the viewer at their
> stations — one peach, one butter, one navy — each with a small
> cream paper card in front of it. A single terra-cotta coral
> round ceramic shape sits on the workbench between the team
> members as the one accent. A subtle 80%-opacity soft drop
> shadow under the main mascot only. The mood is competent and
> at ease — a place where serious work gets done by someone who
> cares.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 02. The Lockup — OG / Social Card

**aspect:** `og` (1200×630)
**use:** Twitter/X share card, LinkedIn preview, generic OG fallback
**tier:** Primary

### Mood

A clean product-launch lockup. Confident, considered, exactly
the kind of OG card a serious tool's announcement deserves.

### Composition by layer

**Layer 1 — Field:** A cream background (`#F8F1E4`), absolutely
flat — no gradient, no motifs. Pure space.

**Layer 2 — Left two-thirds — wordmark lockup:**
- Small brand bug top-left (the paw-mark symbol from #03 at
  about 64 px equivalent, cocoa outline only) — a quiet brand
  flag.
- Wordmark **claws** centred horizontally within this two-thirds
  zone, in the display sans at heavy weight, cocoa colour,
  sentence-case-friendly lowercase, about 180 px equivalent
  height.
- Tagline below the wordmark, in the display sans at medium
  weight, in cocoa, sentence case: **A small team of agents, on
  your own server.** Generous letterspacing.

**Layer 3 — Right third — mascot quiet presence:**
- The cream mascot stands on a 3 px cocoa baseline, occupying
  the right third. Upright, both paws relaxed. Small dot eyes.
  No smile, no blush. Looking slightly toward the wordmark on
  the left — not directly at viewer, but acknowledging the
  brand.
- A subtle 80%-opacity drop shadow under the mascot.

**Layer 4 — Accent:** A single small terra-cotta coral round
shape sits behind the mascot at its feet — like a small ceramic
pot or a marker. The one vivid moment.

### Avoid

No URL. No CTA button. No emoji. No tagline punctuation
beyond the comma. The mascot is dignified, not waving.

### Generator prompt

> A flat vector OG social card with refined startup aesthetic on
> a pure cream background, absolutely flat with no gradient and
> no motifs. In the left two-thirds: a small paw-mark brand bug
> in cocoa outline only at the top-left corner about 64 pixels
> in size; below, the wordmark "claws" centred horizontally
> within this zone in a heavy-weight humanist display sans in
> cocoa-brown, lowercase, about 180 pixels in equivalent height;
> beneath the wordmark, the tagline in a medium-weight humanist
> sans in cocoa with generous letterspacing, sentence case: "A
> small team of agents, on your own server." In the right third:
> the cream mascot stands on a 3-pixel cocoa baseline — round
> head with no ear nubs, two small lentil-sized dot eyes in
> cocoa, neutral expression with no smile and no cheek blush,
> two prominent paw-mittens, upright posture. The mascot looks
> slightly toward the wordmark on the left rather than directly
> at the viewer — acknowledging the brand without performing.
> A subtle 80%-opacity drop shadow under the mascot. A single
> small terra-cotta coral round shape sits behind the mascot at
> its feet as the only accent. No URL, no CTA, no emoji. Mood is
> confident product-launch announcement.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 03. The Mark — Icon / Avatar

**aspect:** `square` (1024×1024)
**use:** GitHub repo avatar, favicon source, app icon
**tier:** Primary

### Mood

A single confident mark. Inevitable. Reads at any size.

### Composition by layer

**Layer 1 — Field:** A solid forest-green (`#3A5A47`) field
filling the entire square. No border, no vignette.

**Layer 2 — The mark:** Centred, a single cream-coloured
geometric paw-mitten shape — abstracted, simplified, slightly
more geometric than the mascot's paws (it is the brand
*symbol*, not a literal drawing of the mascot's paw). Three
small forest-green pad-circles in the underside. The whole mark
occupies about 50% of the canvas, generous margin all around.

**Layer 3 — Refinement:** The paw shape is constructed from
clean rounded geometry — visible deliberate curves, not
hand-drawn wobble. This is the ONE image in v3 that gets
geometric precision. The rest of the brand keeps the slight
wobble; the icon is the locked symbol.

**Layer 4 — Optional accent:** A single 3 px cocoa outline
around the paw shape — a stamped impression effect. Optional;
ship two variants (with and without the outline) so the brand
team can pick.

### Avoid

No drop shadow. No gradient. No literal "claws". No emoji
colours. No background pattern. Must read at 16×16 px —
silhouette is the entire identity.

### Generator prompt

> A flat vector app icon: a solid forest-green field
> (#3A5A47) fills the entire square canvas with no border and
> no vignette. Centred, a single cream-coloured geometric
> paw-mitten symbol — abstracted, simplified, more geometric
> than a literal drawing — constructed from clean deliberate
> rounded curves rather than hand-drawn wobble. Three small
> forest-green circular pad shapes on the underside of the paw.
> The mark occupies about 50% of the canvas with generous margin
> on all sides. A 3-pixel cocoa-brown outline traces the paw
> shape, suggesting a stamped impression. No background pattern,
> no drop shadow, no gradient, no halo. Silhouette must remain
> recognisable at 16-pixel size.
> Brand aesthetic of a contemporary American startup mark — Linear,
> Notion, Vercel, Stripe — confident, geometric, inevitable.

---

## 04. The Desk — Operator's Hour

**aspect:** `hero` (1920×1080)
**use:** "What does this feel like to use" landing-page section
**tier:** Primary

### Mood

A considered workspace. The operator has set things in motion;
the team is at work. The scene has the still confidence of a
Vermeer interior translated to flat vector.

### Composition by layer

**Layer 1 — Backdrop:** A flat forest-green wall fills the
upper 60%. No texture, no pattern.

**Layer 2 — Desk plane:** A flat cream surface fills the lower
40%, divided from the wall by a single 3 px cocoa horizon line.

**Layer 3 — The operator surrogate:** Centre-left on the desk,
a simple flat-fill peach ceramic mug with a single navy stripe
around its middle (a deliberate small design touch). Two short
straight cocoa-line steam wisps rise from the mug — but only
two, deliberate, not feathery.

**Layer 4 — The laptop:** Centre-right, an illustrated laptop
in flat cream with a forest-green screen. On the screen, a
single small butter-yellow dot indicates "everything is
running". The laptop is geometrically clean — a rounded
rectangle for the screen, a rounded trapezoid for the base.

**Layer 5 — The mascot:** Standing on the desk beside the
laptop, centre frame, the cream mascot in upright neutral
posture. Both paws at its sides. Eyes on the laptop screen —
the mascot is watching its team work, not facing the viewer.
Subtle drop shadow under its feet.

**Layer 6 — The team — abstract presence:** On the desk to
the right of the laptop, three small geometric markers — one
peach circle, one navy rounded rectangle, one butter triangle.
These abstract the team without being literal mascot
characters. Each on its own small cream baseline tile.

**Layer 7 — Accent:** A single terra-cotta coral paperclip
shape on the desk near the mug — the one vivid mark.

### Avoid

No human hands. No detailed UI on the laptop. No scattered
desk objects beyond what's listed. No clouds, no sparkles.
The composition is sparse and considered.

### Generator prompt

> A flat vector horizontal hero illustration of a refined
> workspace: a flat forest-green wall fills the upper 60%, a flat
> cream desk surface fills the lower 40%, divided by a single
> 3-pixel cocoa-brown horizon line — no texture, no pattern.
> Centre-left on the desk, a flat-fill peach ceramic mug with a
> single navy stripe around its middle, two short straight cocoa
> steam wisps rising deliberately above it. Centre-right, an
> illustrated laptop with a flat cream body and a forest-green
> screen showing a single small butter-yellow dot — everything is
> running. The laptop is geometrically clean: rounded rectangle
> screen, rounded trapezoid base. Standing on the desk beside the
> laptop in centre frame, the cream mascot in upright neutral
> posture — round head with no ear nubs, small lentil-sized dot
> eyes in cocoa, no smile, no cheek blush, two prominent
> paw-mittens. Both paws rest at its sides; eyes directed at the
> laptop screen rather than the viewer, watching the team work.
> Subtle 80%-opacity drop shadow under the mascot. On the desk to
> the right of the laptop, three small geometric markers
> abstracting the team: a peach circle, a navy rounded rectangle,
> a butter triangle, each on its own small cream baseline tile. A
> single terra-cotta coral paperclip shape on the desk near the
> mug as the one vivid accent. Mood is competent stillness —
> Vermeer translated to vector.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 05. The Roster — Team Lineup

**aspect:** `wide` (2560×1080)
**use:** Mid-page wide divider, blog headers, "your fleet" section
**tier:** Secondary

### Mood

A roster. Each agent has a role, a colour, a baseline. The
arrangement reads as deliberate — closer to a Linear feature
grid than a v2 class photo. The mascots are present but
restrained.

### Composition by layer

**Layer 1 — Backdrop:** A cream background. A faint pattern
across the upper third only: a 24×8 grid of tiny 4 px cream
dots on a barely-lighter cream — almost imperceptible texture.

**Layer 2 — Baseline:** A single 3 px cocoa horizontal line
crosses the lower 30% of the frame — the roster baseline.

**Layer 3 — Five mascots in a row:** Spaced evenly along the
baseline, five mascot characters in different palette colours.
Order left-to-right: peach, butter, forest, navy, cream
(leader, slightly larger at 110% scale, at the right end).
Each in upright neutral posture, both paws at sides, no
smile, no blush. Each looks straight ahead.

**Layer 4 — Role markers:** Above each mascot, floating about
60 px above the head, a single small geometric symbol in cocoa
outline indicating the agent's role:
- Peach: a small envelope shape (messenger)
- Butter: a small plant in pot (gardener / maintainer)
- Forest: a small ledger book (record-keeper)
- Navy: a small antenna (broadcaster)
- Cream (leader): a small ring-of-three-dots (orchestrator)

**Layer 5 — Nameplates:** Below each mascot, a small cream
nameplate with a single name in the display sans at small
size, cocoa colour. Names are short, lowercase, friendly but
not cute. e.g. *sarah*, *ben*, *lead*, *ren*, *alpha*.

**Layer 6 — Accent:** A single terra-cotta coral dot floats
between two of the mascots' role markers — the one accent,
quietly placed.

**Layer 7 — Atmosphere:** Subtle drop shadow under each mascot
on the baseline.

### Avoid

No more than five. No additional decoration. No sparkles, no
hearts. The roster reads as a feature grid, not a children's
book.

### Generator prompt

> A flat vector wide-format roster illustration: a cream
> background with a barely-perceptible 24-by-8 grid of tiny
> 4-pixel cream dots across the upper third only as
> almost-imperceptible texture. A single 3-pixel cocoa horizontal
> baseline crosses the lower 30% of the frame. Five mascot
> characters stand evenly spaced along the baseline in different
> palette colours, left-to-right: peach, butter, forest-green,
> navy, and the cream leader at the right end at 110% scale. Each
> mascot in upright neutral posture — round head with no ear
> nubs, small dot eyes in cocoa, no smile, no cheek blush, two
> prominent paw-mittens — both paws at sides, looking straight
> ahead. Above each mascot, floating about 60 pixels above the
> head, a single small cocoa-outline geometric role symbol: a
> small envelope, a small potted plant, a small ledger book, a
> small antenna, a small ring-of-three-dots respectively. Below
> each mascot, a small cream nameplate with a single short
> lowercase name in a humanist display sans in cocoa-brown. A
> single terra-cotta coral dot floats between two of the role
> markers as the only accent. Subtle 80%-opacity drop shadow
> under each mascot on the baseline. Mood is a deliberate feature
> grid, not a class photo.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 06. The Phone — Pocket Presence

**aspect:** `phone` (1080×1920)
**use:** Mobile hero, story format, vertical landing-page card
**tier:** Secondary

### Mood

A premium product shot of the agent in a chat thread. Reads
like the App Store hero for a refined messaging app. The
mascot is small and present, not popping out of frame.

### Composition by layer

**Layer 1 — Backdrop:** A soft navy-to-forest-green vertical
gradient. Deep, calming, premium.

**Layer 2 — The phone:** Centred, a clean flat-fill cream
smartphone with rounded corners and a thin 3 px cocoa
screen-edge outline. The phone fills the central 60% of the
canvas vertically.

**Layer 3 — Inside the phone screen — chat:** The screen
shows a refined chat interface:
- Top bar: a small cream-on-cream header with the mascot's
  paw-mark icon (from #03) and a name in the display sans:
  *sarah*. Tagline beneath in mono small: *agent · online*.
- Chat area: three peach speech bubbles on the right (user)
  and three cream speech bubbles with 3 px cocoa outlines on
  the left (agent). Bubbles illegible — abstract rounded
  rectangles with tiny cocoa wave-lines standing in for words.
  Sized realistically, not cartoonish.
- Bottom: a clean input field rendered in pale cream with a
  small terra-cotta send button at the right — the one vivid
  accent.

**Layer 4 — The mascot inside the bottom bubble:** At the very
bottom of the upper agent bubble, a tiny cream mascot face
icon (just the round head and dot eyes from #03, no paws, no
ear nubs) sits as the agent avatar. Neutral expression.

**Layer 5 — No mascot popping out:** Unlike v2, the mascot
does NOT break the phone frame. The phone is the subject. The
mascot's presence is felt through the chat avatar and the
mark.

**Layer 6 — Hand:** At the very bottom of the canvas, a
minimal cream-coloured silhouette of a hand holds the phone —
just the gesture, no fingers detailed, no skin tone.

### Avoid

No real phone brand. No detailed UI text. No emoji. No human
face. No mascot breaking the phone frame. The whole image
should read like a premium product render at thumbnail size.

### Generator prompt

> A flat vector vertical phone product illustration: a soft
> navy-to-forest-green vertical gradient background, deep and
> premium. Centred, a clean flat-fill cream smartphone with
> rounded corners and a 3-pixel cocoa screen-edge outline,
> filling the central 60% of the canvas vertically. The screen
> shows a refined chat interface: at the top, a small cream-on-
> cream header with a small paw-mark icon and the name "sarah"
> in a humanist display sans in cocoa-brown, with a smaller mono
> "agent · online" tagline beneath. In the chat area, three peach
> speech bubbles on the right and three cream speech bubbles with
> 3-pixel cocoa outlines on the left — all bubbles illegible
> abstract rounded rectangles with tiny cocoa wave-lines standing
> in for words, sized realistically. At the bottom of the screen,
> a clean pale-cream input field with a small terra-cotta-coral
> send button at the right — the one accent. Inside the bottom
> agent bubble, a tiny mascot face icon (round head, two small
> dot eyes, no paws, no ear nubs, neutral) sits as the agent
> avatar. The mascot does NOT break the phone frame. At the very
> bottom of the canvas, a minimal cream silhouette of a hand
> holds the phone — just the gesture, no detail. Mood is a
> premium App Store hero shot.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 07. The Button — Say "Begin"

**aspect:** `classic` (1600×1200)
**use:** "Get started" section, install-CTA accompaniment
**tier:** Secondary — this is the one image v3 lets be a touch warmer

### Mood

A confident CTA composition. Still warm, still inviting, but
the energy is "let's begin" rather than "yay press it!". The
button is the hero; the mascot is supporting.

### Composition by layer

**Layer 1 — Backdrop:** A flat cream background. No gradient,
no clouds, no sparkles. Pure space.

**Layer 2 — The button:** Centre frame, a large rounded
rectangle button in terra-cotta coral (`#D86545` — the accent
color allowed to be hero this once). About 45% canvas width.
A 3 px cocoa border. **No "depth" rectangle behind it** — we
drop v2's chunky-button depth shape for a flatter, more
sophisticated Linear-style button. A subtle 80%-opacity drop
shadow beneath the button instead.

**Layer 3 — Button label:** Centred on the button face, in
the display sans at heavy weight in cream, sentence case:
**Begin**. (One word. *Begin*, not *go*. v3 is more confident.)

**Layer 4 — The mascot:** Slightly to the right of the button,
standing on the same baseline, the cream mascot in upright
posture, one paw resting on the side of the button — a
companionable touch, like a colleague standing next to you
ready to start. Neutral expression. **Optional small smile**
permitted here (this is the celebration image) — a single
short upward curve, no blush.

**Layer 5 — Confetti hint — minimal:** Above the button, three
small geometric shapes drift gently — a peach circle, a butter
triangle, a navy small square. ONLY THREE. Not v2's confetti
spray. Three deliberate marks.

**Layer 6 — Atmosphere:** Subtle drop shadow under the
mascot's feet. The button itself has a subtle drop shadow.

### Avoid

No "click here" arrow. No motion lines. No sparkles. No
"chunky" button depth effect. The image must read as
confident, not exuberant.

### Generator prompt

> A flat vector horizontal CTA illustration on a flat cream
> background with no gradient and no decoration. Centre frame, a
> large rounded rectangle button in terra-cotta coral about 45%
> canvas width with a 3-pixel cocoa-brown border and a subtle
> 80%-opacity drop shadow beneath — NO chunky depth rectangle, a
> flatter Linear-style button. Centred on the button face in
> heavy-weight humanist display sans in cream, sentence case:
> "Begin". Slightly to the right of the button on the same
> baseline, the cream mascot in upright posture — round head with
> no ear nubs, small lentil-sized dot eyes in cocoa, an optional
> small short upward smile curve (celebration moment allows it),
> no cheek blush, two prominent paw-mittens. One paw rests
> companionably on the side of the button. Above the button,
> three small geometric shapes drift gently — a peach circle, a
> butter triangle, a navy small square. Three only, deliberate,
> no confetti spray. Subtle drop shadow under the mascot's feet.
> Mood is confident invitation — "let's begin" rather than
> exuberant celebration.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 08. The Wordmark — Type Lockup

**aspect:** `square` (1024×1024)
**use:** Press kit, README header, slide decks, type studies
**tier:** Brand asset

### Mood

The wordmark, presented with confidence and restraint. Reads
like the title slide of a thoughtful product announcement.

### Composition by layer

**Layer 1 — Field:** Cream background, flat. **No motifs.** A
single 3 px cocoa horizontal line crosses the canvas at 25%
from the top — a divider, anchoring the composition.

**Layer 2 — Above the line — brand bug:** The paw-mark from
#03 sits centred above the line at 50% scale, cocoa outline
only.

**Layer 3 — Below the line — wordmark stack:**
- Wordmark **claws** centred at about 35% from top, in the
  display sans at heavy weight, cocoa colour, lowercase, about
  280 px equivalent height.
- Tagline below in display sans medium weight, cocoa, sentence
  case: **A small team of agents, on your own server.**
  Generous letterspacing.

**Layer 4 — Below the tagline — accent line:** A single 60 px
horizontal terra-cotta coral line, 4 px thick, centred. The
one vivid mark. Like the colophon of a thoughtful book.

**Layer 5 — Bottom — version mark (optional):** Below the
accent line at small size in mono cocoa, generous letterspacing:
*claws · brand v1.0*. Quiet, like a press-kit page footer.

### Avoid

No mascot character (the brand bug stands in). No background
motifs. No decoration. The type is the subject.

### Generator prompt

> A flat vector square wordmark lockup on a cream background,
> absolutely flat with no motifs. A single 3-pixel cocoa-brown
> horizontal line crosses the canvas at 25% from the top — a
> divider. Above the line, a paw-mark brand bug at 50% scale in
> cocoa outline only, centred. Below the line, the wordmark
> "claws" centred at about 35% from top in a heavy-weight
> humanist display sans in cocoa-brown, lowercase, about 280
> pixels in equivalent height. Below the wordmark, the tagline in
> medium-weight humanist display sans in cocoa with generous
> letterspacing, sentence case: "A small team of agents, on your
> own server." Below the tagline, a single 60-pixel horizontal
> terra-cotta coral line at 4 pixels thick, centred, as the only
> accent — like the colophon of a thoughtful book. At the bottom
> in small mono cocoa with generous letterspacing: "claws · brand
> v1.0". No mascot character, no background motifs, no
> decoration. The type is the subject. Mood is the title slide
> of a thoughtful product announcement.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint.*

---

## 09. The Release — Versioned Announcement

**aspect:** `square` (1024×1024)
**use:** Version release posts, "v1.6.X shipped" social cards
**tier:** Brand asset / utility

### Mood

A confident release-day card. Like the hero from a Vercel ship
post — clean, considered, one thing at a time.

### Composition by layer

**Layer 1 — Field:** A flat forest-green field fills the upper
60% of the canvas. A flat cream field fills the lower 40%. A
single 3 px cocoa horizontal line divides them.

**Layer 2 — Upper field — wordmark + version:**
- Centred at the top, the wordmark **claws** in display sans
  heavy weight, cream colour (legible on the forest field),
  lowercase, about 120 px equivalent.
- Below the wordmark, large, centred, in display sans heavy
  weight, cream: **v.____.____.____**. Fill-in blanks for the
  version number — this is a template.
- Below that, in mono small caps cream: **CHANGELOG INSIDE**.

**Layer 3 — Lower field — mascot delivers:** The cream mascot
stands centred on the cream field, upright. In both paws, the
mascot holds a small flat-fill peach envelope at chest height
— clean, considered, no over-emphasis. The envelope has a
small terra-cotta coral round wax seal centred on it (the one
accent). On the envelope face, in tiny display sans cocoa:
*for you*.

**Layer 4 — Atmosphere:** Subtle drop shadows under the mascot
and under the envelope's bottom edge. No sparkles, no
delivery-wings.

### Avoid

No specific version number filled in — blanks are part of the
template. No "shipped!" or "release!" or "new!" text. No
delivery-motion lines. No emoji. The mascot is dignified,
delivering with care.

### Generator prompt

> A flat vector square release-day card: a flat forest-green
> field fills the upper 60% of the canvas, a flat cream field
> fills the lower 40%, divided by a single 3-pixel cocoa-brown
> horizontal line. In the upper forest field: centred at the top,
> the wordmark "claws" in heavy-weight humanist display sans in
> cream colour, lowercase, about 120 pixels equivalent height;
> below the wordmark, large centred in display sans heavy weight
> in cream: "v.____.____.____" with blanks for the version (this
> is a reusable template); below that, in mono small caps in
> cream: "CHANGELOG INSIDE". In the lower cream field: the cream
> mascot stands centred in upright posture — round head with no
> ear nubs, small dot eyes in cocoa, no smile, no cheek blush,
> two prominent paw-mittens. The mascot holds a small flat-fill
> peach envelope at chest height between both paws, clean and
> considered. The envelope has a small terra-cotta coral round
> wax seal centred on its face (the only accent), with tiny
> display sans cocoa text reading "for you". Subtle 80%-opacity
> drop shadows under the mascot and beneath the envelope's
> bottom edge. No sparkles, no wings, no emoji. Mood is
> confident release-day announcement, like a Vercel ship post
> hero.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 10. The Field — Brand Pattern

**aspect:** `square` (1024×1024)
**use:** Tiled backgrounds, decorative dividers, footer fills
**tier:** Brand asset

### Mood

A geometric pattern. Quiet. Confident. The kind of pattern you
would put on the inside cover of a serious notebook.

### Composition by layer

**Layer 1 — Base:** Solid cream field.

**Layer 2 — Pattern motif:** A regular 6-column × 6-row grid
of small paw-mark icons (from #03, simplified to silhouette
only) in **cocoa outline only** (not filled). Each mark about
100 px equivalent. Generous spacing. **NO rotation variation**
— v3 prefers ordered grids over hand-stamped feel.

**Layer 3 — Accent diagonal:** A single 3 px terra-cotta coral
line crosses the field on a clean diagonal, top-left to
bottom-right. Continues seamlessly across tile edges. The one
accent.

**Layer 4 — Seamless tiling:** The grid must align across
edges. The diagonal accent line must align across edges. This
is a CSS tile.

### Avoid

No paw-print "footprint" style. No checkerboard alternation
(that was v2). No hidden Easter-egg mascot. No sparkles. The
pattern is calm and geometric.

### Generator prompt

> A seamless flat vector tile pattern on a solid cream field: a
> regular 6-column by 6-row grid of small paw-mark icons —
> simplified geometric silhouettes of a paw-mitten with three
> pad-circles — in 3-pixel cocoa-brown outline only (not filled).
> Each mark about 100 pixels in equivalent size with generous
> spacing. No rotation variation; the grid is ordered and aligned.
> A single 3-pixel terra-cotta coral diagonal line crosses the
> field from top-left to bottom-right, continuing seamlessly
> across tile edges (when CSS-tiled, the line must exit one
> corner at the same coordinate it enters the opposite corner).
> The grid must also align seamlessly across edges. No focal
> point, no directional shadow, no vignette, no sparkles, no
> hidden Easter eggs. Mood is the inside cover of a serious
> notebook.
> *Flat vector illustration in a restrained warm palette of
> cream and cocoa-brown line work of consistent ~3px weight,
> with a single terra-cotta-coral accent diagonal. No gradients,
> no textures, no realism, no sparkles. Refined geometry, mature
> illustration style of a contemporary American startup, intended
> for repeating use as a website background tile.*

---

## 11. The Reference — Palette Card

**aspect:** `portrait` (1080×1350)
**use:** Style guide, brand reference page, press kit
**tier:** Brand asset

### Mood

A clean colour reference. Like a Pantone chip but mature, like
a brand manual page from a thoughtful design system.

### Composition by layer

**Layer 1 — Field:** Cream background, flat.

**Layer 2 — Header:** Top of the frame, in display sans heavy
weight cocoa, sentence case: **Palette**. Left-aligned with
generous margin. Below the title, in mono small caps cocoa
with generous letterspacing: **CLAWS · BRAND v1.0**.

**Layer 3 — Divider:** A single 3 px cocoa horizontal line
beneath the title block.

**Layer 4 — Swatches:** Below the divider, six rectangular
flat-fill swatches arranged in a 2-column × 3-row grid in the
middle of the frame. Each swatch is a clean rectangle (NOT a
circle — circles were v2), about 380 × 240 px equivalent, with
a 1.5 px cocoa border. Order:

- **Cream** / **Forest** (top row)
- **Navy** / **Peach** (middle row)
- **Butter** / **Cocoa** (bottom row)

**Layer 5 — Labels:** Beside each swatch (to its right), in
display sans medium cocoa: the colour name. Below the name, in
mono cocoa: the hex code. e.g. **Forest** / `#3A5A47`.

**Layer 6 — Accent swatch (special):** Below the 2×3 grid,
isolated, the **Terra** accent swatch presented at slightly
smaller size with a label noting **ACCENT — USE ONCE PER
COMPOSITION**.

**Layer 7 — Footer:** Bottom of the frame, in mono small cocoa
with generous letterspacing: **PAGE 01 / 12 — BRAND**.

### Avoid

No mascot. No circles. No sparkles. No decoration. The
palette is the subject.

### Generator prompt

> A flat vector portrait-orientation brand palette reference
> page: cream background. Top of frame, the title "Palette" in
> heavy-weight humanist display sans in cocoa-brown, sentence
> case, left-aligned with generous margin. Below the title, in
> mono small caps in cocoa with generous letterspacing: "CLAWS ·
> BRAND v1.0". A single 3-pixel cocoa horizontal divider line
> beneath the title block. Six rectangular flat-fill colour
> swatches arranged in a 2-column 3-row grid in the middle of the
> frame, each swatch about 380 by 240 pixels with a 1.5-pixel
> cocoa border: cream and forest-green in the top row, navy and
> peach in the middle row, butter and cocoa in the bottom row.
> Beside each swatch on its right, the colour name in
> medium-weight display sans in cocoa with the hex code in mono
> beneath. Below the 2x3 grid, isolated, a smaller terra-cotta
> coral accent swatch labelled "ACCENT — USE ONCE PER
> COMPOSITION" in mono small caps. At the bottom of the frame, in
> mono small cocoa with generous letterspacing: "PAGE 01 / 12 —
> BRAND". No mascot, no circles, no decoration. Mood is a
> thoughtful design-system brand manual page.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles. Generous whitespace, refined
> geometry, mature design-system aesthetic.*

---

## 12. The Studio — Craft Mood

**aspect:** `portrait` (1080×1350)
**use:** "About" page, ethos / values section
**tier:** Tertiary

### Mood

The studio where the work gets made. Considered, occupied,
quiet. Closer to the back-page photo in a thoughtful magazine
than v2's "workshop with apron".

### Composition by layer

**Layer 1 — Backdrop:** A flat forest-green wall fills the
upper 65%. **No windows, no sun, no clouds.** Pure wall.

**Layer 2 — Floor:** A flat cream floor fills the lower 35%.

**Layer 3 — The desk:** Centre-frame, a clean rectangular
peach desk on two thin navy legs. Geometrically refined.

**Layer 4 — On the desk:** Three objects only.
- Centre: a small flat-fill cream notebook, closed.
- Left: a flat-fill navy fountain pen, laid horizontally.
- Right: a small ceramic mug, butter colour, with a single
  navy stripe (same mug as #04 — consistent prop).

**Layer 5 — Above the desk:** On the wall behind the desk, a
single small framed cream rectangle (a hung print). Inside
the frame, the paw-mark symbol from #03 at small scale,
cocoa outline only.

**Layer 6 — The mascot:** Standing to the right of the desk,
the cream mascot in upright neutral posture. One paw resting
on the desk surface — a gesture of ownership. Looking
straight ahead toward the viewer with calm presence. No
smile, no blush.

**Layer 7 — Accent:** A single terra-cotta coral bookmark
ribbon sticks out of the closed notebook — the one accent.

**Layer 8 — Atmosphere:** Subtle drop shadows under the
desk, the mascot, and the framed print.

### Avoid

No apron on the mascot. No tools (no wrench, no paintbrush).
No marker board, no to-do lists. The studio is sparse and
considered. Three objects on the desk, one print on the
wall, one mascot, one accent.

### Generator prompt

> A flat vector portrait illustration of a refined studio
> interior: a flat forest-green wall fills the upper 65% with no
> windows, no sun, no clouds — pure wall. A flat cream floor
> fills the lower 35%. Centre-frame, a clean rectangular peach
> desk on two thin navy legs, geometrically refined. On the desk,
> three objects only: a small flat-fill cream closed notebook in
> the centre, a flat-fill navy fountain pen laid horizontally on
> the left, a small butter-coloured ceramic mug with a single
> navy stripe on the right. On the wall behind the desk, a single
> small framed cream rectangle containing the paw-mark symbol at
> small scale in cocoa outline only. Standing to the right of the
> desk, the cream mascot in upright neutral posture — round head
> with no ear nubs, small lentil-sized dot eyes in cocoa, no
> smile, no cheek blush, two prominent paw-mittens. One paw rests
> on the desk surface in a gesture of ownership; the mascot looks
> straight ahead at the viewer with calm presence. A single
> terra-cotta coral bookmark ribbon sticks out of the closed
> notebook as the one accent. Subtle 80%-opacity drop shadows
> under the desk, the mascot, and the framed print. Mood is the
> back-page photo in a thoughtful magazine — sparse and
> considered.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 13. The Appliance — Your Server

**aspect:** `hero` (1920×1080)
**use:** "Runs on your hardware" section, privacy / sovereignty section
**tier:** Tertiary

### Mood

A premium home server appliance rendered as a small object.
Not a cottage with a cute window (v2). Not a hand-made wooden
chassis (v1). A confident industrial object — like the
Synology / Plex / Hello Sign / Eero generation of home
appliances rendered in v3 illustration language.

### Composition by layer

**Layer 1 — Backdrop:** A flat navy field fills the upper 65%.
A subtle barely-visible 24×8 cream dot grid in the upper
third only.

**Layer 2 — Surface:** A flat cream surface fills the lower
35%.

**Layer 3 — The appliance:** Centred on the surface, a small
illustrated server in isometric projection — about 25% of
canvas width. Construction:
- Body: flat-fill peach rounded rectangle (a sympathetic
  warm tone, not industrial grey)
- Front face: a clean cream rectangle with three small
  cocoa-line ventilation slots arranged horizontally
- One small butter-yellow indicator dot in the lower-right of
  the front face — "everything is running"
- A small navy heat-sink fin pattern on the top (three thin
  lines)
- A subtle drop shadow beneath

**Layer 4 — The mascot:** Standing to the left of the
appliance on the cream surface, the cream mascot in upright
neutral posture. About 70% the height of the appliance. One
paw resting on the side of the appliance — owner's gesture.
Looking straight ahead, calm presence.

**Layer 5 — A small cable:** A single thin 3 px cocoa cable
runs from the back of the appliance off the right edge of the
frame — implies the server is connected to the home network
without drawing the network.

**Layer 6 — Accent:** A small terra-cotta coral round shape
sits on the cream surface in the lower-right corner — a
ceramic pot abstractly, or just a marker. The one accent.

### Avoid

No cottage / house metaphor. No yard with playing mascots. No
chimney. No "home server" sign. No racks. This is an
appliance — like a thoughtful smart-home device.

### Generator prompt

> A flat vector horizontal illustration of a premium home server
> appliance: a flat navy field fills the upper 65% with a subtle
> barely-visible 24-by-8 cream dot grid in the upper third only.
> A flat cream surface fills the lower 35%. Centred on the cream
> surface, a small illustrated server appliance in isometric
> projection about 25% of canvas width: a flat-fill peach rounded
> rectangle body, a clean cream front face with three small
> cocoa-line horizontal ventilation slots and one small
> butter-yellow indicator dot in the lower-right of the front
> face, and a navy heat-sink fin pattern of three thin lines on
> top. A subtle 80%-opacity drop shadow beneath the appliance.
> Standing to the left of the appliance on the cream surface, the
> cream mascot in upright neutral posture — round head with no
> ear nubs, small dot eyes, no smile, no cheek blush, two
> paw-mittens — at about 70% the height of the appliance. One
> paw rests on the side of the appliance in an owner's gesture;
> the mascot looks straight ahead with calm presence. A single
> thin 3-pixel cocoa cable runs from the back of the appliance
> off the right edge of the frame, implying network connection.
> A small terra-cotta coral round shape in the lower-right corner
> of the cream surface as the one accent. NOT a cottage, NOT a
> house — a thoughtful appliance like a Synology or Plex
> generation home device, rendered in flat vector. Mood is
> confident sovereignty as premium ownership.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 14. The Threshold — Community Banner

**aspect:** `banner` (1500×500)
**use:** GitHub repo banner, CONTRIBUTING.md header, community page
**tier:** Tertiary

### Mood

A confident community banner. The brand at scale — not a
storefront with awning (v2). More like the header of a Linear
community page or a Vercel community section.

### Composition by layer

**Layer 1 — Backdrop:** A flat cream field across the full
banner. **No clouds, no sun, no flagpoles.**

**Layer 2 — Wordmark left:** Far left, vertically centred, the
wordmark **claws** in display sans heavy weight cocoa,
lowercase, about 100 px equivalent height. Below the wordmark,
in mono small caps cocoa with generous letterspacing:
**OPEN-SOURCE · COMMUNITY**.

**Layer 3 — A row of mascots — centred:** In the centre of
the banner, a row of THREE mascots stand on a single 3 px
cocoa baseline, evenly spaced — peach, cream, forest. Each in
upright neutral posture. The cream one (centre) holds a small
cream paper card with a 3 px cocoa border — abstractly
representing a pull request. The other two stand at ease.

**Layer 4 — Right — pennant lockup:**
Far right, vertically centred, a small lockup:
- A small flat-fill terra-cotta coral rectangle (a flag
  shape) — the accent — about 80 px tall.
- Beside it, in display sans medium cocoa: **Contribute →**.
  Sentence case, one arrow.

**Layer 5 — Atmosphere:** Subtle drop shadows under the three
mascots and under the wordmark.

### Avoid

No storefront. No awning. No "welcome!" pennants. No mascots
walking with gift boxes. No exclamation marks. The community
energy is calm welcome, not parade.

### Generator prompt

> A flat vector horizontal banner illustration on a flat cream
> field with no clouds, no sun, no decoration. Far left, the
> wordmark "claws" vertically centred in heavy-weight humanist
> display sans in cocoa-brown, lowercase, about 100 pixels in
> equivalent height. Below the wordmark, in mono small caps in
> cocoa with generous letterspacing: "OPEN-SOURCE · COMMUNITY".
> In the centre of the banner, three mascot characters stand
> evenly spaced on a single 3-pixel cocoa baseline in upright
> neutral posture — peach, cream, and forest-green coloured. The
> centre cream mascot holds a small cream paper card with a
> 3-pixel cocoa border abstractly representing a pull request;
> the other two stand at ease. Each mascot has a round head with
> no ear nubs, small dot eyes, no smile, no cheek blush, two
> paw-mittens. Far right, vertically centred, a lockup: a small
> flat-fill terra-cotta coral rectangle about 80 pixels tall (the
> only vivid accent) beside the text "Contribute →" in
> medium-weight display sans in cocoa, sentence case. Subtle
> 80%-opacity drop shadows under the three mascots and under the
> wordmark. Mood is calm community welcome — Linear / Vercel
> community page energy.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

## 15. The Evening — Quiet Companionship

**aspect:** `portrait` (1080×1350)
**use:** "A friend that does the work" emotional anchor, blog footer images
**tier:** Tertiary

### Mood

End of the day. The studio is quiet. One lamp is on. The
mascot sits with a book. The team is settled. Sophisticated
warmth — not bedtime story (v2), not Vermeer interior (v1).
Adult evening at home.

### Composition by layer

**Layer 1 — Backdrop:** A flat deep navy field fills the
upper 70%. **No moon, no stars, no sparkles, no scattered
dots.** Pure depth.

**Layer 2 — Floor:** A flat forest-green floor fills the
lower 30%.

**Layer 3 — The armchair:** Centre-right of the frame, a
clean flat-fill peach armchair — geometrically refined. About
40% of canvas height.

**Layer 4 — The mascot reading:** Sitting in the armchair,
the cream mascot in a seated posture (legs out, body settled
into the chair). Both paws hold a small flat-fill cream open
book at chest height. Dot eyes directed at the book — the
mascot is reading, not looking at the viewer. Neutral
expression. The book has a small terra-cotta coral bookmark
ribbon hanging from the bottom — the one accent.

**Layer 5 — The lamp:** Centre-left of the frame, on a small
navy side table, a single illustrated brass-coloured lamp
(flat butter colour) with a cream lampshade. The lamp emits
a soft cream-coloured circular glow that gently lights the
floor and the lower edge of the armchair — implied with a
30%-opacity flat fill, NOT a gradient.

**Layer 6 — The team at rest:** On the floor in front of the
armchair, three small mascot figures sit in a small cluster —
peach, butter, navy — each curled in a resting posture. Eyes
closed (small upward-curve arcs, but NOT with Z bubbles —
v2's Z bubbles were too cute). They are just resting.

**Layer 7 — Atmosphere:** Subtle drop shadows under the
armchair, the side table, and the resting mascots. The lamp
glow does NOT cast harsh shadows — it's a soft warm
indication.

### Avoid

No bedtime, no nightgowns, no Z bubbles, no crescent moon
with sleepy face. No bedside book with "goodnight" energy.
The mood is adult evening reading — calm warmth.

### Generator prompt

> A flat vector portrait illustration of a quiet evening
> interior: a flat deep navy field fills the upper 70% with no
> moon, no stars, no sparkles — pure depth. A flat forest-green
> floor fills the lower 30%. Centre-right of the frame, a clean
> flat-fill peach armchair, geometrically refined, about 40% of
> canvas height. Sitting in the armchair, the cream mascot in
> seated posture with legs out and body settled into the chair —
> round head with no ear nubs, small dot eyes directed at a book
> (not at the viewer), no smile, no cheek blush, two paw-mittens
> holding a small flat-fill cream open book at chest height. The
> book has a small terra-cotta coral bookmark ribbon hanging from
> the bottom as the only accent. Centre-left of the frame, on a
> small navy side table, a single illustrated lamp with a flat
> butter-coloured brass body and a cream lampshade. The lamp
> emits a soft cream-coloured circular glow rendered as 30%-
> opacity flat fill (NOT a gradient) that gently lights the floor
> and the lower edge of the armchair. On the floor in front of
> the armchair, three small mascot figures sit in a small
> resting cluster — peach, butter, navy — each curled with eyes
> closed (small upward-curve arcs, NO Z-bubbles). Subtle
> 80%-opacity drop shadows under the armchair, the side table,
> and the resting mascots. Mood is adult evening at home — calm
> warmth, sophisticated, not bedtime.
> *Flat vector illustration in a restrained warm palette of
> cream, forest green, navy, peach, butter, and cocoa-brown line
> work of consistent ~3px weight, with a single terra-cotta-coral
> accent used once. No gradients on objects, no textures, no
> realism, no scattered sparkles, no kawaii cuteness markers (no
> cheek blush, no big eyes, no Sanrio expressions). Generous
> whitespace, refined geometry, mature illustration style of a
> contemporary American startup (Linear / Notion / Vercel /
> Stripe / Pitch). Confident, friendly, warm but grown-up,
> deliberate restraint. A subtle 80%-opacity soft drop shadow
> under the mascot and key objects (offset 4-6px, soft 8px
> feather) — Linear-style whisper.*

---

# Notes for the reshape checker (v3-specific)

Aspect ratio handling: ±2% tolerance, smart-crop with centre
bias, Lanczos resize, never stretch. Same as v1 and v2.

**v3-specific guards:**

- **Cuteness markers — auto-reject.** If the generator output
  contains any of: visible cheek blush ovals, Z-bubble sleep
  indicators, "big eyes" (eyes >2% of canvas area in any single
  mascot), wide tongue-out smiles, "kawaii" sparkles or
  twinkles → reject and re-generate. v3 must hold the line on
  these.
- **Palette count check.** No more than four colours per image
  (plus cocoa line work and one terra-cotta accent). Detect by
  k-means clustering on the output; reject if >5 distinct fills
  appear.
- **Accent area cap.** Terra-cotta coral must occupy ≤3% of the
  canvas (exception: image 07, the Begin button — up to 22%
  allowed; image 09, the upper forest field is large but
  forest, not terra). Detect by hue mask in the
  `#D86545`-adjacent range.
- **Mascot consistency.** Across all images that feature the
  mascot (01, 02, 04, 05, 07, 09, 12, 13, 14, 15), run a
  perceptual hash on the mascot head and compare to the
  established mean after the first three are approved. Reject
  if any one diverges substantially.
- **No-ear-nub check.** v3 mascot has NO ear nubs. If the
  generator adds them, reject — the silhouette will be
  inconsistent with #03 (the icon).
- **OCR guard on 08, 09, 11, 14.** Confirm typography is spelled
  correctly.
- **Tile guard for 10.** Verify seamless edge alignment + the
  diagonal accent line continues across edges.

Output naming:

```
images/v3/01-open-workshop.jpg
images/v3/02-the-lockup.jpg
...
images/v3/15-the-evening.jpg
```

The `/v3/` directory separates this set from v1 and v2 so
direction switches are a CSS path change.

---

# Subset recommendations

**Five images** (minimum viable brand):
01 (hero), 02 (OG), 03 (icon), 08 (wordmark), 09 (release).

**Eight images** (full launch pack):
add 04 (desk), 05 (roster), 11 (palette).

**All fifteen** (complete library):
the remaining seven cover sovereignty, community, and
emotional anchors for blog and marketing.

**Pin the mascot first.** Same advice as v2 but more urgent —
v3's restraint depends on the mascot being EXACTLY the same
across 10+ images. Commission #03 first, lock the mascot's
proportions, eye spacing, paw geometry, and posture. Use the
approved asset as a character-reference for every other prompt
when the generator supports it.

**Mixing the three brand directions:** v1, v2, and v3 should
NOT mix on the same page. Pick one per surface. If your
audience is mixed — say, both technical and consumer-friendly
contexts — consider using v3 for the marketing site and v2 for
in-product onboarding illustrations, but never both within a
single page or email.

**Recommended choice for an American startup launch:** v3 for
landing page + OG cards + GitHub repo. v2 for blog post
illustrations IF you want a softer tone occasionally. v1 for
press kit / brand archive only.
