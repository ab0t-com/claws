# claws — brand imagery prompts (v2 — kawaii startup)

A complete pivot from v1's tactile photographic direction. Same 15
use cases, same reshape-checker contract, but the look is **Japanese
kawaii character design meets American startup product-launch
polish** — Sanrio meets Notion meets Linear. Flat colours, clean
contours, generous whitespace, a friendly mascot at the heart of
every shot.

If you only commission five: **01, 02, 03, 04, 09**. Same as v1.

---

## What we're going for (mood reference)

**Yes:**
- Sanrio / San-X character system (Cinnamoroll, Pochacco, Pusheen
  era kawaii)
- LINE Friends mascot energy
- Notion's illustration system — flat vectors, witty, friendly
- Slack's early launch illustrations
- Linear's geometric clarity
- Figma's chunky friendly spot illustrations
- Mailchimp's whimsy
- Japanese product launch posters of the 1980s–90s — clean
  typesetting + a darling mascot

**No:**
- Photorealism. Anywhere. Ever.
- Anime in the "big sparkly eyes" sense — we want simple dot eyes
- Manga panel composition
- Detailed textures (cloth weave, wood grain, brushed metal —
  forbidden)
- Heavy gradients (a gentle one-stop gradient on a background sky
  is OK, but no chrome / glassy / neon-glow)
- Sci-fi UI overlays
- Corporate stock illustration "people in flat colour" (Alegria
  style is over)
- The tactile photographic direction from v1 — different file,
  different brand

---

## The mascot

The brand has **one mascot** that appears in most images. Treat it
like a Sanrio property: same proportions, same details, every time.
Variations are *colour* and *accessory* only.

**Body plan:**
- A round head (roughly 60% of total figure) — almost a circle, very
  slightly squat.
- A small rounded body directly attached (no neck), like a soft
  bean. About 40% the head's height.
- Two soft paw-mittens stick out from the body — **these are the
  brand**. The paws are visible, prominent, never hidden. Each paw
  is a soft mitten shape with three tiny pad-circles on the
  underside (visible when palms-up).
- Two tiny stub legs at the base, just enough to show it's
  standing — toes optional.
- Two small ear nubs on top of the head — soft triangles, rounded
  corners. Not pointed. Cat-adjacent but not committed.

**Face:**
- Two simple **dot eyes** in warm dark cocoa (`#5A3E2B`), spaced
  generously, slightly larger than you'd expect — about the size of
  small peppercorns.
- A tiny **smile** — a single short upward curve, optional (some
  expressions are neutral).
- Two soft **blush ovals** on the cheeks in pale pink
  (`#FF9FA8`) — small, low opacity, just a hint of warmth.
- No nose. No mouth other than the smile curve.
- No eyebrows. Eyes do all the emotion work.

**Default colour:** cream (`#FFF8EC`) body with pale-honey
(`#FFE699`) paw-pads. Some images recolour the mascot — see notes
per prompt.

**Personality:** Quietly delighted. Not goofy, not aggressive,
not sad. The default expression is "happy to be here".

**Scale rule:** When the mascot appears in a composition with
furniture, treat it as palm-sized — about 12 cm tall. It's a small
desk companion, not a person.

---

## Visual signature (every image)

### Palette

A six-colour soft pastel system. Use 2–3 per image; never more
than four. **Pick a hero colour per image** and let the rest play
support.

- **Cream** `#FFF8EC` — base backgrounds, mascot body
- **Peach** `#FFC4A3` — warm primary, sunny mood
- **Mint** `#A8E6CF` — cool secondary, fresh mood
- **Lavender** `#D8BFFF` — accent, calm mood
- **Butter** `#FFE699` — cheerful highlight, mascot paw-pads
- **Sky** `#BEE6FA` — soft blue, sky/water/calm
- **Blush** `#FF9FA8` — cheek dots, occasional small accent
- **Cocoa** `#5A3E2B` — line work and type (instead of black)

**Hero accent (use ONCE per image):** a small bright element in
**Sunset** `#FF7A45` or **Cherry** `#E84A6F` — the eye-catch. Like
the terra-cotta in v1, this is the deliberate spark.

### Line work

- **All contours in cocoa** (`#5A3E2B`), never black.
- Line weight ≈ 4 px equivalent (relative to canvas). Consistent
  thickness — no calligraphic taper, no variable width.
- Small interior details: 2 px equivalent — half the contour weight.
- Lines slightly *imperfect* — a wobble of a few pixels, like the
  ink-pen sketch is human-drawn, not vector-perfect. Avoid the
  "Illustrator pen tool" plasticky look.

### Shading

- **No gradients on objects.** Flat fills.
- **Allowed:** one gentle background gradient on sky/wall (e.g.
  peach-to-cream from top to bottom).
- **Allowed:** a single flat darker-tone shape used as "shadow"
  — e.g. a 90%-opacity oval on the floor under the mascot, no
  blur, no feathering. Like a Pochacco-era shadow.
- **No drop shadows on text. No glow. No bevels. No highlights.**

### Typography

- **Display:** a rounded humanist sans — Quicksand, Nunito, or
  the rounded geometric Japanese sensibility of Tazugane Gothic
  Round. Friendly, warm, slightly playful but professional.
- **Mono (used sparingly, for the version-line on release art):**
  rounded mono — Berkeley Mono Round, or Comic Mono.
- **Type colour:** cocoa (`#5A3E2B`) by default.
- **No italics, no all-caps screaming, no condensed.**

### Background patterns / motifs

Pick from these. Use 1–2 per image:

- Soft puffy clouds (peach, mint, or sky)
- Small dotted sparkles (4-point twinkles, cocoa, ≤8 per image)
- Polka dots (cream on a pastel field, generously spaced)
- Tiny stars (5-point, hollow, cocoa)
- Tiny hearts (cocoa outline only)
- Soft sun (just a circle + 4–6 stubby rays)
- Wavy horizon line (ground meeting sky)
- Tile pattern of mini paw-mittens (see #10)

### Style suffix (append to every generator prompt)

> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients on
> objects, no textures, no realism, generous whitespace, rounded
> friendly shapes, kawaii character sensibility (Sanrio / LINE
> Friends), contemporary American startup product-launch polish
> (Notion / Linear / Slack illustration system), warm cheerful
> mood, gentle and inviting, no harsh black (cocoa-brown instead),
> no detailed textures, no heavy shading.*

Keep that line consistent across all generator prompts. It is the
glue.

---

## Aspect ratio matrix (for the reshape checker)

Same matrix as v1 so the checker can reuse its logic.

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

Reshape rules: ±2% tolerance, smart-crop with centre bias, never
stretch. See the v1 file for the full checker contract — v2 uses
the same logic, only the *content* is different.

---

# The fifteen images

---

## 01. Hello, Workshop — Master Hero

**aspect:** `hero` (1920×1080)
**use:** Landing page hero, GitHub social preview, repo OG image fallback
**tier:** Primary

### Mood

The mascot is at their workbench, calmly setting up the small team
for the day. The viewer arrives and is *invited in*. Bright,
optimistic, "good morning let's do this".

### Composition by layer

**Layer 1 — Sky/wall:** Soft peach-to-cream vertical gradient
filling the upper two-thirds. A few puffy mint clouds drift
across the upper portion. One tiny sunset-orange sparkle in the
upper right — the eye-catch.

**Layer 2 — Ground/desk:** A horizontal warm-butter band fills
the lower third — a simple flat-fill desk surface, no wood
grain. A single thin cocoa line separates desk from wall.

**Layer 3 — Subject:** The cream mascot stands centre-left on
the desk, slightly turned to face viewer-right. One paw raised
in a small wave. Smile present. Cheek blush.

**Layer 4 — The small team:** To the mascot's right, four
miniature mascot variants stand in a friendly row, each in a
different palette colour (peach, mint, lavender, sky), each
holding a tiny tool — a paintbrush, a watering can, a tiny
envelope, a tiny radio. Each has its own dot eyes and tiny
smile. Each has a small cream nameplate at its feet.

**Layer 5 — Foreground delight:** A few tiny cocoa-outline
sparkles float between the main mascot and the team. One small
cherry-red paper plane lies on the desk in front of the
mascot — the single bright accent.

### Lighting + palette

No directional light — flat illumination. Hero colours: peach,
cream, butter. Each team mascot uses one other palette colour.
One cherry-red accent.

### Avoid

No human characters. No realistic depth. No background detail
beyond clouds. No text overlays at generation time. No more
than 5 team mascots (don't overcrowd).

### Generator prompt

> A flat vector illustration in soft pastel palette: a friendly
> cream-coloured round mascot character with two small ear nubs,
> dot eyes in warm cocoa-brown, a tiny smile, soft pink cheek
> blush, and two prominent paw-mittens stands centre-left on a
> warm butter-yellow desk surface, one paw raised in a small
> hello wave. To the mascot's right, four miniature versions of
> the same character in peach, mint, lavender and soft sky-blue
> stand in a friendly row, each holding a tiny tool — a
> paintbrush, a watering can, a tiny envelope, a tiny radio — and
> each with a small cream nameplate at its feet. The background
> is a soft peach-to-cream vertical gradient with a few puffy
> mint clouds drifting across the upper portion. A few tiny
> cocoa-outline sparkles float between the main mascot and the
> team. A single small cherry-red paper plane lies on the desk in
> front of the main mascot as the only vivid accent. Clean
> cocoa-brown line work of consistent weight, no realism, no
> textures.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 02. The Welcome — OG / Social Card

**aspect:** `og` (1200×630)
**use:** Twitter/X share card, LinkedIn preview, generic OG fallback
**tier:** Primary

### Mood

A friendly product-launch banner. The kind of OG card that makes
someone smile and click. Confident but warm.

### Composition by layer

**Layer 1 — Field:** Cream background (`#FFF8EC`), no gradient,
flat. Sparse cocoa-outline sparkles scattered (no more than 6).

**Layer 2 — Left third — wordmark lockup:**
- Top: tiny mascot face icon (just the head from the icon
  treatment in #03), about 80 px equivalent, centred over the
  wordmark.
- Centre: the wordmark **claws** in the rounded display sans,
  cocoa colour, all lowercase, large (about 200 px
  equivalent height).
- Below wordmark: tagline in mono small caps cocoa colour:
  **A SMALL TEAM OF AGENTS, ON YOUR OWN SERVER.**
- Aligned left within the left third.

**Layer 3 — Right two-thirds — mascot moment:**
- The full mascot character stands holding up a small cream sign
  that reads **hi!** in the display sans — soft, two-letter, the
  social greeting.
- The mascot's free paw waves.
- Behind the mascot, a soft butter circle sun with 4 stubby rays
  fills the centre of the right two-thirds. Mascot stands in
  front of it.
- Two small mint clouds drift in.

**Layer 4 — Accent:** A single cherry-red tiny heart shape
floats above the mascot's left ear — the single vivid mark.

### Avoid

No URL. No call-to-action button. No emoji. The cream sign is
the only message; the wordmark is the only branding.

### Generator prompt

> A flat vector illustration banner for a product-launch social
> card: cream background with sparse cocoa-outline sparkles. On
> the left third, a small mascot head icon sits above a large
> lowercase wordmark "claws" in a rounded humanist sans typeface
> in cocoa-brown, with a smaller mono small-caps tagline below
> reading "A SMALL TEAM OF AGENTS, ON YOUR OWN SERVER." All
> left-aligned. On the right two-thirds, the full cream-coloured
> mascot character — round head with small ear nubs, dot eyes
> in cocoa, tiny smile, pink cheek blush, two prominent
> paw-mittens — stands in front of a soft butter-yellow sun
> circle with four stubby rays. The mascot holds up a small cream
> sign with the word "hi!" in lowercase, and the free paw waves.
> Two small mint puffy clouds drift in. A single cherry-red tiny
> heart shape floats above the mascot's left ear as the only
> vivid accent. Clean cocoa-brown line work, no realism, no
> gradients on objects.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 03. The Face — Icon / Avatar

**aspect:** `square` (1024×1024)
**use:** GitHub repo avatar, favicon source, app icon
**tier:** Primary

### Mood

The mascot's face in a circle. Inevitable, recognisable, repostable.

### Composition by layer

**Layer 1 — Field:** A solid pastel-peach circle fills about
80% of the square canvas, centred. A 4 px cocoa border around
the circle. The rest of the square is cream.

**Layer 2 — The face:** Inside the circle, the mascot's head
fills about 70% of the circle's area. Just the head — no body,
no paws. Two small ear nubs at the top. Two dot eyes spaced
generously, slightly wide-set. A tiny upward-curve smile.
Two soft pink cheek blush ovals.

**Layer 3 — One paw moment:** A single paw-mitten enters the
bottom of the circle, just visible — like the mascot is peeking
up over the edge of the frame. This signals "claws" without
being literal.

**Layer 4 — Nothing else:** No background pattern inside the
circle. No sparkles. No outline halo. Negative space does the
work.

### Avoid

No text. No drop shadow. No multiple colours competing — peach
+ cream + cocoa lines is the whole palette here. Must remain
recognisable at 16×16 px — silhouette is the brand.

### Generator prompt

> A flat vector app-icon illustration: a solid pastel-peach
> circle fills about 80% of the square canvas, centred, with a
> 4-pixel cocoa-brown border. The rest of the square is cream.
> Inside the circle, the mascot's head — round, cream-coloured,
> two small rounded ear nubs at the top, two dot eyes in cocoa
> spaced generously, a tiny upward-curve smile, two soft pink
> cheek blush ovals — fills about 70% of the circle's area.
> A single cream paw-mitten with pale-butter pads enters the
> bottom edge of the circle as if the mascot is peeking up over
> the frame. No background pattern, no sparkles, no halo, no
> text. Silhouette must remain recognisable at 16-pixel size.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 04. Sip & Watch — Operator's Hour

**aspect:** `hero` (1920×1080)
**use:** "What does this feel like to use" landing-page section
**tier:** Primary

### Mood

You set things up; your little team is doing the work. You sit
back with a drink. Mascots happy at their stations. The vibe of
"this is going well".

### Composition by layer

**Layer 1 — Backdrop:** Soft mint-to-cream vertical gradient
sky. Two puffy peach clouds drift across the upper third. A
single butter-yellow sun shape (just a circle + 4 stubby rays)
in the upper right.

**Layer 2 — Ground line:** A flat horizon line at about 60%
height. Below the horizon, a warm cream flat-fill desk surface.

**Layer 3 — Operator surrogate:** Centre-left, a simple
illustrated coffee mug (cream cup, lavender hot drink, two tiny
cocoa-line steam wisps rising) sits on the desk. The mug stands
in for the operator — we don't show a person.

**Layer 4 — Mascot team — the agents at work:** Spread across
the right two-thirds of the desk:
- The main cream mascot sits in front of a tiny pastel-peach
  laptop, paws tapping the keyboard, cheerful expression.
- A mint mascot waters a small potted plant.
- A peach mascot carries a tiny envelope.
- A lavender mascot reads a tiny open book.
- A sky-blue mascot strums a tiny ukulele.
Each at a different station, each with a tiny smile.

**Layer 5 — Delight:** A few cocoa-outline sparkles float
above the team. One cherry-red small heart floats above the
coffee mug — the operator is feeling fond of their team.

### Avoid

No human figures. No detailed desk objects. No realistic
laptop UI — the laptop is a flat-fill rectangle with a single
cocoa line for the screen edge. Don't overcrowd — leave breathing
room between mascots.

### Generator prompt

> A flat vector horizontal illustration: a soft mint-to-cream
> vertical gradient sky with two puffy peach clouds and a small
> butter-yellow sun (circle plus four stubby rays) in the upper
> right. A flat horizon line at about 60% height; below, a warm
> cream desk surface. Centre-left, an illustrated coffee mug —
> cream cup, lavender drink, two tiny cocoa-line steam wisps —
> stands in for the operator (no person shown). Spread across the
> right two-thirds of the desk, five mascot characters at work:
> the main cream mascot taps a tiny pastel-peach laptop with a
> cheerful expression; a mint-coloured mascot waters a small
> potted plant; a peach mascot carries a tiny envelope; a
> lavender mascot reads a tiny open book; a sky-blue mascot
> strums a tiny ukulele. Each has dot eyes in cocoa, a tiny
> smile, pink cheek blush, and two paw-mittens. A few
> cocoa-outline sparkles float above the team. One small
> cherry-red heart floats above the coffee mug as the only vivid
> accent. The mood is "the operator's team is happily working".
> Clean cocoa-brown line work, no realism, no textures.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 05. Meet the Team — Multi-Agent Lineup

**aspect:** `wide` (2560×1080)
**use:** Mid-page wide divider, blog headers, "meet your team" section
**tier:** Secondary

### Mood

A class photo of your agent team. Each one a delight, each
distinct, each lined up grinning at the camera. The kind of
image that makes a viewer say "I want one".

### Composition by layer

**Layer 1 — Background:** Soft sky-blue gradient fading to
cream toward the bottom. A wavy butter horizon line about 65%
down — like a soft grassy field. Above the horizon, a few small
cocoa-outline stars and one cherry-red tiny heart — the accent.

**Layer 2 — The lineup:** Five mascots stand in a row across
the middle of the frame, evenly spaced, each one in a different
hero colour. Order left-to-right: peach, butter, mint, sky,
lavender. The cream version stands at the far right as the
"leader" (slightly larger — 110% scale).

**Layer 3 — Accessories:** Each mascot wears one small
identifying accessory:
- Peach: a tiny envelope satchel
- Butter: a paintbrush behind one ear
- Mint: a small potted-plant on top of head
- Sky: a tiny radio antenna headband
- Lavender: a small book held in paws
- Cream (leader): a tiny cream conductor's baton

**Layer 4 — Nameplates:** Each mascot stands on a small cream
nameplate, hand-lettered in cocoa with a single short
lowercase name — *sarah*, *john*, *ben*, *lead*, *alpha*, and
the cream one labelled *you* (because the team leader is the
operator's avatar, conceptually).

**Layer 5 — Sparkle:** A scattering of cocoa-outline sparkles
above the lineup. One cherry-red small heart hovers above the
group as the single vivid accent.

### Avoid

Don't make any mascot bigger than 110% of the standard. Don't
add a sixth — five is the number. Don't add text other than the
nameplates.

### Generator prompt

> A flat vector wide-format class-photo illustration: a soft
> sky-blue-to-cream gradient background with a wavy butter
> horizon line at about 65% height. A few small cocoa-outline
> stars dot the upper sky. Five mascot characters stand in an
> evenly-spaced row across the middle: peach, butter,
> mint, sky-blue, and lavender. The cream-coloured leader stands
> at the far right, slightly larger at 110% scale. Each has the
> standard mascot design — round head, ear nubs, dot eyes, tiny
> smile, pink cheek blush, two paw-mittens. Each wears one small
> identifying accessory: the peach one a tiny envelope satchel,
> the butter one a paintbrush behind one ear, the mint one a
> small potted-plant on top of its head, the sky-blue one a tiny
> radio antenna headband, the lavender one a small book held in
> paws, the cream leader a tiny cream conductor's baton. Each
> stands on a small cream nameplate hand-lettered in cocoa with
> a lowercase name. A scattering of cocoa-outline sparkles above
> the lineup and one small cherry-red heart hovering above the
> group as the only vivid accent. Mood is a delighted team class
> photo. Clean cocoa-brown line work, no realism.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 06. Pocket Buddy — Phone Moment

**aspect:** `phone` (1080×1920)
**use:** Mobile hero, story format, vertical landing-page card
**tier:** Secondary

### Mood

The mascot is in your phone. When you message them, they're
right there. Companionship in a pocket.

### Composition by layer

**Layer 1 — Background:** Lavender-to-cream vertical gradient.
A few small cocoa-outline sparkles and one small cherry-red
heart float in the upper third — the accent.

**Layer 2 — The phone:** Centre frame, a friendly flat-fill
illustrated smartphone with rounded corners, cream body, a
cocoa screen-edge outline. The phone fills the central 60% of
the canvas vertically.

**Layer 3 — Inside the phone screen:** The screen shows a
chat interface — peach speech bubbles on the right (from "you")
and cream speech bubbles on the left (from the mascot). The
bubbles are illegible — no real text, just round bubble shapes
with tiny cocoa-line wavy lines suggesting words. At the top of
the screen sits a small mascot head avatar with a name in tiny
cocoa display sans: *sarah*.

**Layer 4 — The mascot peeking out:** The cream mascot's head
and one paw pop OUT of the top of the phone screen — breaking
the frame, three-dimensionally — waving hello directly at the
viewer. Mascot tiny smile. The phone's screen border curls back
slightly at the corners where the mascot emerges.

**Layer 5 — Hand suggestion:** At the very bottom of the
frame, a soft minimal cream-coloured outline of a hand holds
the phone — just enough silhouette to suggest it's being held.
No fingers detailed, no skin tone, just the gesture.

### Avoid

No realistic phone brand. No detailed UI text. No actual
emoji. No human face. Keep the chat completely abstract — the
*shape* of conversation, not the words.

### Generator prompt

> A flat vector vertical illustration: a lavender-to-cream
> vertical gradient background with a few small cocoa-outline
> sparkles and one small cherry-red heart in the upper third. A
> friendly flat-fill cream smartphone with rounded corners and a
> cocoa screen-edge outline sits centred, filling the central
> 60% of the canvas vertically. The phone's screen shows an
> abstract chat interface — peach speech bubbles on the right
> and cream speech bubbles on the left, all bubbles illegible
> (just round bubble shapes with tiny cocoa-line wavy lines
> standing in for words). At the top of the screen, a small
> mascot head avatar with the name "sarah" in tiny cocoa
> display sans. The cream mascot character pops out of the top
> of the phone screen — head and one paw breaking the frame
> three-dimensionally, the other paw waving hello directly at
> the viewer, tiny smile, pink cheek blush. The phone's screen
> border curls back slightly where the mascot emerges. At the
> very bottom of the frame, a soft cream-coloured silhouette of
> a hand holds the phone — no detail, just the gesture. Mood is
> companionable pocket presence. Clean cocoa-brown line work, no
> realism.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 07. Press the Button — Say GO

**aspect:** `classic` (1600×1200)
**use:** "Get started" section, install-CTA accompaniment
**tier:** Secondary

### Mood

The "now we begin" moment, in cute cartoon form. A big
inviting button. The mascot is about to press it. You will
also press it. Joy.

### Composition by layer

**Layer 1 — Backdrop:** Cream background. A scattering of
cocoa-outline tiny stars and sparkles. Two soft mint clouds in
the upper corners.

**Layer 2 — The button:** Centre frame, a large rounded
rectangle button in cherry-red (`#FF7A45`-`#E84A6F`-ish — the
accent colour, but used as the hero this one time). The button
is about 50% canvas width, with a 4 px cocoa border and a 12 px
cocoa offset "depth" rectangle behind it (giving the chunky
playful button-with-shadow effect popular in product launch
illustrations). On the button face, in the rounded display sans
in cream colour, all lowercase: **go**.

**Layer 3 — The mascot:** The cream mascot stands directly in
front of the button, on the lower-centre of the frame, both
paws raised at the height of the button. About to press with
both paws. Big delighted smile. Cheek blush. Tiny stars in the
mascot's dot eyes (small cream-coloured star reflections — the
ONE moment we let eyes have a sparkle).

**Layer 4 — Anticipation marks:** A few cocoa-outline motion
lines radiate from the mascot's paws toward the button — like
manga "energy lines" but stubby and friendly, not aggressive.
A few tiny cocoa-outline stars above the mascot.

**Layer 5 — Confetti hint:** Around the upper edges of the
button, a sprinkling of tiny peach, mint, lavender, butter
confetti rectangles — like the confetti is already starting.

### Avoid

The button is the ONE vivid red moment in the brand —
elsewhere the cherry-red is small accents only. Here it gets
to be the hero. Don't add a literal "click here" arrow. Don't
overdo confetti.

### Generator prompt

> A flat vector illustration: cream background scattered with
> cocoa-outline tiny stars and sparkles and two soft mint clouds
> in the upper corners. Centred, a large chunky cherry-red
> rounded-rectangle button about 50% of the canvas width, with a
> 4-pixel cocoa border and a 12-pixel cocoa offset depth
> rectangle behind it (giving a chunky playful raised-button
> appearance). The button face reads "go" in a rounded humanist
> sans typeface in cream colour, all lowercase. The cream mascot
> stands directly in front of the button on the lower centre of
> the frame, both paws raised to press it, a delighted smile, two
> tiny cream-coloured star reflections in the dot eyes (the one
> sparkle moment we allow). Stubby friendly cocoa-outline motion
> lines radiate from the mascot's paws toward the button. A few
> tiny cocoa-outline stars above the mascot. A sprinkling of
> tiny peach, mint, lavender, and butter rectangular confetti
> pieces hint around the upper edges of the button. Mood is
> joyful anticipation of beginning.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 08. The Wordmark — Type Lockup

**aspect:** `square` (1024×1024)
**use:** Press kit, README header, slide decks, type studies
**tier:** Brand asset

### Mood

The official wordmark in its lockup. Clean, friendly, presented
without ceremony — like a logo on a sticker sheet.

### Composition by layer

**Layer 1 — Field:** Cream background, flat. Sparse: just a
single soft butter sun (circle + 4 stubby rays) in the upper
right corner about 80 px diameter, and three or four tiny
cocoa-outline sparkles scattered.

**Layer 2 — The wordmark:** Centred horizontally, set in the
lower half of the frame: the wordmark **claws** in the rounded
humanist display sans in cocoa colour. All lowercase. Large —
about 60% of canvas width. Slightly soft, slightly hand-touched
letterforms (not vector-perfect — small wobble).

**Layer 3 — The mascot beside the type:** To the left of the
wordmark, a single mascot face peeking up — only the head and
one paw visible, as if the mascot is leaning against the *c* of
the wordmark. Cream body, dot eyes, tiny smile, paw resting on
top of the *c*.

**Layer 4 — Tagline:** Two centimetres below the wordmark, in
mono small caps in cocoa: **A SMALL TEAM OF AGENTS, ON YOUR OWN
SERVER.** Generous letterspacing. Centred under the wordmark.

**Layer 5 — Bug:** In the upper-left corner, a tiny stamped
brand bug — just the mascot's two paw-mittens making a soft
heart shape (the brand mark concept in v2). About 80 px
equivalent. Cocoa outline only.

### Avoid

No frame around the wordmark. No background decoration. The
type, the mascot peek, and the tagline are the whole
composition.

### Generator prompt

> A flat vector wordmark lockup illustration: cream background
> with a single soft butter-yellow sun shape (circle plus four
> stubby rays) in the upper right corner about 80 pixels in
> diameter, and three or four tiny cocoa-outline sparkles
> scattered. In the upper left corner, a tiny stamped brand bug
> consisting of the mascot's two paw-mittens forming a soft heart
> shape, cocoa outline only. Centred horizontally in the lower
> half of the frame: the wordmark "claws" in a rounded humanist
> display sans typeface in cocoa-brown, all lowercase, about 60%
> of canvas width, with slightly soft hand-touched letterforms
> (not vector-perfect). To the left of the wordmark, the cream
> mascot's head and one paw peek up as if leaning against the
> letter "c" with the paw resting on top of it. Two centimetres
> below the wordmark, in a rounded mono small-caps typeface in
> cocoa, generous letterspacing, centred under the wordmark: "A
> SMALL TEAM OF AGENTS, ON YOUR OWN SERVER." No frame, no
> background decoration. Mood is a friendly sticker-sheet
> wordmark presentation.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 09. Special Delivery — Release Announcement

**aspect:** `square` (1024×1024)
**use:** Version release posts, "v1.6.X shipped" social cards
**tier:** Brand asset / utility

### Mood

A cute mascot delivering today's release. Each new version is
an envelope the mascot is excited to hand over.

### Composition by layer

**Layer 1 — Backdrop:** Soft mint background. Scattered
cocoa-outline sparkles and one cherry-red small heart in the
upper right.

**Layer 2 — The mascot:** Centre frame, the cream mascot
stands holding a large peach envelope up to the viewer with
both paws. Big delighted smile. Cheek blush. Two stub legs
visible below the envelope.

**Layer 3 — The envelope:** The envelope nearly the same size
as the mascot's head — slightly oversized for cute proportion.
Peach body, cream flap closed with a small heart-shaped sticker
seal (the brand-bug paw-mittens-making-a-heart from #08). On
the front of the envelope, hand-lettered in cocoa display
sans, three lines:
- Line 1 (small, top): TO YOU
- Line 2 (large, centred): **v.____.____.____**  — fill-in
  blanks for the version number; this is a template
- Line 3 (small, bottom): WITH LOVE FROM CLAWS

**Layer 4 — Postage stamp corner:** Upper-right corner of the
envelope shows a small stamp — a butter square with the mascot
head icon and a tiny stamped circle date cancel mark.

**Layer 5 — Delight:** A few cocoa-outline sparkles around
the envelope. Two tiny cream wing shapes on either side of the
envelope — implying it's flying / being delivered.

### Avoid

No specific version number filled in — the blanks are part of
the template. No "shipped!" overlay text. No emoji. The mascot
is the only character.

### Generator prompt

> A flat vector square illustration on a soft mint background
> with scattered cocoa-outline sparkles and one cherry-red small
> heart in the upper right. Centred, the cream mascot character —
> round head, ear nubs, dot eyes, tiny delighted smile, pink
> cheek blush — stands holding up a large peach envelope with
> both paw-mittens. The envelope is roughly the same size as the
> mascot's head, with a cream flap closed by a small
> heart-shaped sticker seal made of two paw-mittens forming a
> heart shape. On the front of the envelope, three lines of
> hand-lettered cocoa display sans text: a small top line
> reading "TO YOU"; a large centred line reading
> "v.____.____.____" with blank fill-in spaces for the version
> number (this is a reusable template); a small bottom line
> reading "WITH LOVE FROM CLAWS". A small butter-yellow postage
> stamp in the upper-right corner of the envelope shows a tiny
> mascot head icon and a stamped circle date cancel mark. A few
> cocoa-outline sparkles and two tiny cream wing shapes on
> either side of the envelope suggest a special delivery in
> motion. The mood is cheerful release-day announcement. Clean
> cocoa-brown line work.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 10. Paw Print Field — Repeating Pattern

**aspect:** `square` (1024×1024)
**use:** Tiled backgrounds, decorative dividers, footer fills, wrapping paper
**tier:** Brand asset

### Mood

A pattern wallpaper for the brand. The kind of repeating motif
you'd put on the inside of a notebook cover, a Slack channel
background, a sticker.

### Composition by layer

**Layer 1 — Base:** Solid pastel-peach field across the entire
canvas.

**Layer 2 — Pattern motif:** Across the field, a regular
4-column × 4-row grid of motifs. Two motifs alternate in a
checkerboard pattern:
- The mascot's small paw-mitten print (just the mitten with
  three tiny pad-circles), in cream, cocoa outline
- A small cocoa-outline 5-point star

Each motif about 120 px equivalent, evenly spaced, centred in
its grid cell. Slight rotation variation: each motif rotated
randomly by ±15° to feel hand-stamped not vector-perfect.

**Layer 3 — Accent:** In one cell — the second column, third
row from top — replace the standard motif with a tiny full
cream mascot head (just the face from icon #03 at the same
~120 px scale). One mascot per tile, hidden in the pattern as
a delight Easter egg.

**Layer 4 — Edge alignment:** The pattern grid must align
seamlessly with itself when CSS-tiled. The motifs in column 4
must visually continue into column 1 of the next tile, and row
4 into row 1. No motif crosses the seam.

### Tiling note

CRITICAL: this is a seamless tile. Edges must align. No
directional shadow, no vignette, no off-centre composition.

### Avoid

No central focal point. No directional shadow. No mascot
larger than the standard motif scale — the Easter egg is the
same scale as the surrounding paws.

### Generator prompt

> A seamless flat vector tile pattern on a solid pastel-peach
> field: a regular four-column by four-row grid of two
> alternating motifs in a checkerboard arrangement — the
> mascot's small paw-mitten print in cream with cocoa outline
> and three tiny pad-circles, alternating with a small
> cocoa-outline five-point star. Each motif is about 120 pixels
> in equivalent size and centred in its grid cell, with a slight
> random rotation of plus-or-minus 15 degrees to feel
> hand-stamped rather than vector-perfect. In one specific cell
> — the second column, third row from the top — replace the
> standard motif with a tiny full mascot head face (round head,
> ear nubs, dot eyes, tiny smile, pink cheek blush) at the same
> 120-pixel scale, hidden in the pattern as a delight. The grid
> must align seamlessly with itself when CSS-tiled: motifs in
> column four must visually continue into column one of the next
> tile, and row four into row one. No directional shadow, no
> vignette, no focal point — this is a field, not a composition.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 11. The Palette Card — Colour Reference

**aspect:** `portrait` (1080×1350)
**use:** Style guide page, brand reference page, press kit
**tier:** Brand asset

### Mood

A cheerful colour-chip card. Like a Sanrio character introducing
the palette for their world.

### Composition by layer

**Layer 1 — Field:** Cream background. A few cocoa-outline tiny
stars scattered. A single butter sun shape upper right.

**Layer 2 — Header:** Top of frame, hand-lettered in the
display sans, cocoa colour: *the palette*. Lowercase, friendly.

**Layer 3 — The swatches:** Below the header, eight rounded
flat-fill colour circles arranged in a 2×4 grid in the lower
two-thirds of the frame. Each circle about 200 px diameter,
generous spacing between. Order top-to-bottom, left-to-right:
- Cream, Peach
- Mint, Lavender
- Butter, Sky
- Blush, Cocoa

Each circle a flat solid fill. 4 px cocoa border around each.

**Layer 4 — Labels:** Below each circle, in rounded mono small
caps cocoa colour: the colour name + hex code on two lines, e.g.
**CREAM / #FFF8EC**. Generous letterspacing.

**Layer 5 — Mascot moment:** In the upper-right area near the
sun, a small cream mascot leans into frame holding a tiny
paintbrush dipped in cherry-red — the one vivid accent —
gesturing toward the palette. Cheerful smile. As if the mascot
is the palette's introducer.

### Avoid

No flat colour rectangles — the circles are non-negotiable.
No CMYK callouts. Don't make the mascot too large; it's a
supporting presence here, the palette is the subject.

### Generator prompt

> A flat vector portrait-orientation brand palette reference
> card: cream background scattered with cocoa-outline tiny
> stars and a single soft butter-yellow sun shape (circle plus
> four stubby rays) in the upper right. Top of frame,
> hand-lettered in a rounded humanist display sans in
> cocoa-brown, lowercase: "the palette." Below the header, eight
> rounded flat-fill colour circles arranged in a 2-column by
> 4-row grid in the lower two-thirds, each circle about 200
> pixels in diameter with a 4-pixel cocoa border, generous
> spacing. Colours top-to-bottom, left-to-right: cream, peach,
> mint, lavender, butter, sky-blue, blush, cocoa. Below each
> circle in rounded mono small-caps cocoa colour, the colour name
> and hex code on two lines (e.g. CREAM / #FFF8EC) with generous
> letterspacing. In the upper-right area near the sun, a small
> cream mascot character — round head, ear nubs, dot eyes, tiny
> smile, pink cheek blush — leans into the frame holding a tiny
> paintbrush dipped in cherry-red, gesturing toward the palette
> as its cheerful introducer. The cherry-red is the only vivid
> accent. No flat rectangles, no CMYK callouts.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 12. The Workshop — Craft Mood

**aspect:** `portrait` (1080×1350)
**use:** "About" page, ethos/values section
**tier:** Tertiary

### Mood

The mascot is at work. The workshop is a happy place. Tools
are simple shapes; everything has a friendly face energy.

### Composition by layer

**Layer 1 — Backdrop:** Soft mint wall. Two square cocoa-line
"windows" near the top, each containing a tiny butter sun
shape — friendly windows into a sunny day.

**Layer 2 — Workbench:** A flat-fill peach workbench fills the
lower half. Two simple stub legs visible.

**Layer 3 — The mascot at work:** The cream mascot stands
behind the workbench, both paws on a small project on the
desk. The project is a tiny rounded laptop shape with a small
green dot on its screen (the gentle "all good" signal). The
mascot wears a small cream apron tied at the back, and has a
small pencil tucked behind one ear nub.

**Layer 4 — Tools on the bench:** Arranged simply, no clutter:
- A tiny brass-coloured wrench (just shapes, not detailed)
- A small notebook with a tiny pencil
- A ceramic mug of butter-yellow tea, one tiny steam wisp
- A small potted plant (mint leaves, peach pot)
- A tiny paw-print stamp (the brand bug as a literal stamp
  object)

**Layer 5 — Marker board:** Above and to the left of the
mascot, a cream-coloured rounded-rectangle marker board with
three small handwritten cocoa lines that read (illegibly, just
wavy lines + a few stars) — implying a to-do list. The top item
has a small cocoa checkmark beside it.

**Layer 6 — Accent:** One small cherry-red heart sticker on
the marker board — the one accent.

### Avoid

No realistic tool detail. No detailed UI on the laptop —
just the shape and one green dot. No clutter; this is calm
work.

### Generator prompt

> A flat vector portrait illustration of a friendly workshop: a
> soft mint wall background with two square cocoa-outline
> windows near the top, each containing a tiny butter-yellow sun
> shape. A flat-fill peach workbench with two stub legs fills the
> lower half. Behind the workbench, the cream mascot — round
> head, ear nubs, dot eyes, tiny smile, pink cheek blush, two
> paw-mittens — stands wearing a small cream apron tied at the
> back, with a tiny pencil tucked behind one ear nub. Both paws
> rest on a small project on the desk: a tiny rounded laptop
> shape with a single small green dot on its screen. Arranged
> simply on the bench: a small brass-coloured wrench shape, a
> small notebook with a tiny pencil, a ceramic mug of
> butter-yellow tea with one tiny steam wisp, a small potted
> mint plant in a peach pot, and a tiny paw-print stamp object.
> Above and to the left of the mascot, a cream rounded-rectangle
> marker board with three small cocoa wavy lines suggesting a
> to-do list (illegible by design); the top item has a small
> cocoa checkmark beside it and a small cherry-red heart sticker
> in the corner as the only vivid accent. Mood is calm cheerful
> work.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 13. The Little House — Your Server

**aspect:** `hero` (1920×1080)
**use:** "Runs on your hardware" section, privacy/sovereignty section
**tier:** Tertiary

### Mood

Your server is a cosy little house where your team lives. Not
a datacentre, not a cloud. A small home you can point to.

### Composition by layer

**Layer 1 — Sky:** Soft sky-blue-to-cream gradient. Two puffy
peach clouds. A single butter sun shape upper-left with stubby
rays.

**Layer 2 — Ground:** A wavy mint horizon line about 70% down.
Below, a flat mint ground.

**Layer 3 — The little house (the server):** Centred on the
ground, a small house-shaped illustration about 35% of canvas
width:
- Cream walls
- Peach pitched roof
- One cocoa-outlined round window in the centre — through
  which we see the cream mascot waving
- A cocoa-outline rounded door on the right with a tiny brass
  doorknob
- A small cocoa-line chimney with one tiny butter heart
  emerging instead of smoke (the warmth coming out)
- A small wooden sign on a stake in front of the house,
  cocoa-line, hand-lettered: *home server*

**Layer 4 — Yard companions:** Three tiny mascot variants
(peach, mint, sky) play in the yard around the house — one
gardens a tiny plant, one carries an envelope, one waves at
the viewer.

**Layer 5 — Accent:** A single cherry-red small heart floats
above the chimney's heart-smoke — the one vivid accent.

### Avoid

No realistic server hardware. No rack. No racks. No literal
data-centre imagery. No barbed wire, no fences. The server
must look like a house.

### Generator prompt

> A flat vector horizontal illustration: a soft sky-blue-to-cream
> gradient sky with two puffy peach clouds and a single
> butter-yellow sun shape with stubby rays in the upper left.
> A wavy mint horizon line at about 70% height with flat mint
> ground below. Centred on the ground, a friendly small
> house-shaped illustration about 35% of canvas width: cream
> walls, peach pitched roof, one cocoa-outlined round window in
> the centre through which we see the cream mascot waving, a
> cocoa-outline rounded door on the right with a tiny
> brass-coloured doorknob, a small cocoa-line chimney from
> which one tiny butter heart emerges instead of smoke. A small
> wooden sign on a stake in front of the house is hand-lettered
> in cocoa: "home server." Three tiny mascot variants — peach,
> mint, and sky-blue — play in the yard around the house: one
> gardens a tiny plant, one carries an envelope, one waves at
> the viewer. A single cherry-red small heart floats above the
> chimney's heart-smoke as the only vivid accent. Mood is "your
> server is a cosy little home." Clean cocoa-brown line work.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 14. The Open Shop — Community Banner

**aspect:** `banner` (1500×500)
**use:** GitHub repo banner, CONTRIBUTING.md header, community page
**tier:** Tertiary

### Mood

A storefront welcome banner. The mascot is standing out front
of a small shop, waving. Visitors welcome.

### Composition by layer

**Layer 1 — Backdrop:** Soft butter-yellow sky gradient to
cream toward the bottom. A few tiny cocoa-line clouds.

**Layer 2 — Ground:** Flat peach ground band along the bottom
20%.

**Layer 3 — The shop:** Across the centre of the banner, a
small flat-fill shop facade. Cream walls, peach awning with
cocoa stripes. A friendly storefront window with the wordmark
**claws** painted on it in display sans cocoa. A cocoa-outline
"OPEN" sign hangs in the window, hand-lettered.

**Layer 4 — The mascot greeter:** In front of the shop, the
cream mascot stands centred, paws raised in a big welcoming
double-wave. Big smile. Cheek blush. Wears a small cream apron.

**Layer 5 — Banner:** Above the shop, strung between two tiny
cocoa-line flagpoles, a cream pennant banner reads in display
sans cocoa: **welcome — pull requests welcome too!**

**Layer 6 — Friends arriving:** To the right of the shop, two
small mascot variants (mint, lavender) walk toward the shop
carrying tiny cream gift boxes — implying community
contribution.

**Layer 7 — Accent:** A single cherry-red heart floats above
the shop sign — the one vivid accent.

### Avoid

No corporate "hiring" energy. No URL. No bullet points. No
faces other than mascots. The banner is welcoming, not
selling.

### Generator prompt

> A flat vector horizontal banner illustration: soft butter-yellow
> sky gradient to cream toward the bottom with a few tiny
> cocoa-line clouds. A flat peach ground band along the bottom
> 20%. Across the centre, a small flat-fill shop facade — cream
> walls, peach awning with cocoa stripes, a friendly storefront
> window with the wordmark "claws" painted on it in a rounded
> humanist display sans in cocoa. A cocoa-outline "OPEN" sign
> hangs in the window, hand-lettered. In front of the shop, the
> cream mascot character stands centred with both paws raised in
> a welcoming double-wave, big smile, pink cheek blush, wearing a
> small cream apron. Above the shop, strung between two tiny
> cocoa flagpoles, a cream pennant banner reads in display sans
> cocoa: "welcome — pull requests welcome too!" To the right of
> the shop, two small mascot variants in mint and lavender walk
> toward the shop carrying tiny cream gift boxes, implying
> community contribution. A single cherry-red heart floats above
> the shop sign as the only vivid accent. Mood is friendly open
> storefront welcome.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

## 15. Sleepy Time — Evening Companionship

**aspect:** `portrait` (1080×1350)
**use:** "A friend that does the work" emotional anchor, blog footer images
**tier:** Tertiary

### Mood

It's late. The mascots are settling in for the night. Your team
is happy and so are you. Soft, quiet, kind.

### Composition by layer

**Layer 1 — Backdrop:** Deep lavender night sky gradient to
cream-pink at the bottom. A friendly crescent moon in the
upper-right — flat-fill cream, cocoa outline, with a tiny
sleeping face (closed-eye curves, no mouth). A scattering of
small cocoa-outline stars across the sky.

**Layer 2 — Ground:** A flat mint band along the bottom 15%.

**Layer 3 — The bed:** Centre frame, a small flat-fill peach
bed with a cream-pink pillow. The bed is mascot-scale — about
30% of canvas width.

**Layer 4 — The main mascot tucked in:** The cream mascot
lies in the bed, paws folded on top of the blanket. Eyes
closed (small upward-curve arcs instead of dots — sleep mode).
Tiny smile. A small cream bubble floats above its head with a
single cocoa-outlined "z" inside.

**Layer 5 — The team's tiny beds:** Around the main bed,
arranged in a soft semicircle, four tiny smaller beds — peach,
mint, lavender, sky — each with a sleeping mini-mascot in the
matching colour. All sleep-eyes, all tiny smiles, all little
"z" bubbles.

**Layer 6 — Bedside accent:** Beside the main bed, a small
brass-coloured lamp turned off, with a tiny cherry-red
bookmark sticking out of a closed cream book — the one vivid
accent. A ceramic mug of tea, empty.

**Layer 7 — Atmosphere:** A few cocoa-outline tiny stars
sprinkled around the heads of the sleeping mascots — like a
quiet lullaby.

### Avoid

No literal alarm clock. No phone (none of the v2 evening
scenes include phones). No human bed — the bed shape is small,
mascot-scale.

### Generator prompt

> A flat vector portrait illustration: a deep lavender night sky
> gradient fading to cream-pink at the bottom. In the upper
> right, a friendly crescent moon — flat-fill cream with cocoa
> outline, with a tiny sleeping face (two small closed-eye
> curves, no mouth). A scattering of small cocoa-outline stars
> across the sky. A flat mint ground band along the bottom 15%.
> Centred in the lower middle, a small mascot-scale peach bed
> with a cream-pink pillow, about 30% of canvas width. The cream
> mascot lies tucked in with paws folded on top of the blanket;
> its eyes are closed (small upward-curve arcs instead of dots),
> a tiny smile, and a small cream "z" bubble floats above its
> head. Around the main bed, arranged in a soft semicircle, four
> tiny smaller beds in peach, mint, lavender, and sky-blue, each
> with a sleeping mini-mascot of matching colour, all in sleep
> mode with tiny "z" bubbles. Beside the main bed, a small
> brass-coloured turned-off lamp, a closed cream book with a
> tiny cherry-red bookmark sticking out (the only vivid accent),
> and an empty ceramic mug of tea. A few cocoa-outline tiny
> stars sprinkled around the heads of the sleeping mascots. Mood
> is soft kind goodnight to the team.
> *Flat vector illustration in soft pastel palette, clean
> cocoa-brown line work of consistent ~4px weight, no gradients
> on objects, no textures, no realism, generous whitespace,
> rounded friendly shapes, kawaii character sensibility (Sanrio /
> LINE Friends), contemporary American startup product-launch
> polish (Notion / Linear / Slack illustration system), warm
> cheerful mood, gentle and inviting, no harsh black (cocoa-brown
> instead), no detailed textures, no heavy shading.*

---

# Notes for the reshape checker (same as v1)

Aspect-ratio handling: ±2% tolerance, centre-bias smart-crop,
Lanczos resize, never stretch.

**v2-specific guards:**

- **Mascot consistency check.** Across images 01, 04, 05, 06, 09,
  11, 12, 13, 14, 15, the mascot's body plan must be consistent.
  Run a perceptual hash on the mascot head from each generated
  image and reject if any one diverges substantially from the
  established mean (after the first three are approved). Image
  gen models will drift on character design across multiple
  generations — this is the biggest v2 risk.
- **No-realism guard.** Reject if the image contains realistic
  textures (wood grain, fabric weave, brushed metal, skin
  pores). This shouldn't happen if the suffix is honoured but
  some models hedge.
- **No-face-other-than-mascot guard.** Reject if a human face is
  detected — humans are represented by surrogates (mug, hand
  silhouette) in v2.
- **OCR guard on 08, 09, 14.** Confirm typography spells
  "claws" correctly and tagline reads as written. Image gen
  models corrupt type constantly.
- **Tile guard for 10.** Verify seamless edge alignment by
  tiling the result 2×2 in memory and checking for visible seams.
- **Cherry-red accent count.** In images 01–06 and 11–15, there
  should be ONE vivid cherry-red element. Detect by hue mask
  (#FF7A45–#E84A6F range). If the area exceeds 3% of canvas, the
  accent is too dominant; reject. Image 07 (the GO button) is
  the exception — the button is allowed to be large red.

Output naming:

```
images/v2/01-hello-workshop.jpg
images/v2/02-the-welcome.jpg
...
images/v2/15-sleepy-time.jpg
```

The `/v2/` directory separates this set from the v1 tactile set
so a future direction switch is just a CSS path change.

---

# Subset recommendations

**Five images** (minimum viable brand):
01 (hero), 02 (OG), 03 (icon), 08 (wordmark), 09 (release).

**Eight images** (full launch pack):
add 04 (operator's hour), 05 (lineup), 11 (palette).

**All fifteen** (complete library):
the remaining seven round out community, sovereignty, and
emotional anchors for blog content and ongoing marketing.

**A note on consistency:** v2 will live or die on the mascot
holding together across all 15 generations. Consider doing 03
(the icon) FIRST, locking the mascot's exact proportions and
features, then feeding that approved icon back into every
subsequent prompt as a reference image when the generator
supports image-to-image / character-lock. If the generator
doesn't support character lock, commission the mascot
design from a single illustrator first, then use that artwork
as the source for all other compositions (either traced by
the same illustrator or fed to a generator with character
reference).

**A note on mixing v1 and v2:** they should NOT mix on the same
page. Pick one direction per surface. If you want the warmth of
v1 photography for the "about" page and the friendliness of v2
illustration for the marketing site, that's fine — but inside any
single page or any single email, commit to one.
