# Folkweave's loom

A small wooden loom with a woven sage, cream, and terracotta panel, a shuttle,
simple round eyes, and a smile. No eyebrows. The mascot appears in the header
and sign-in screen; a small PNG supplies the favicon. Settings and account
controls use plain names rather than characterful metaphors.

Generated with the built-in imagegen tool on 2026-11-05. Imvault's keeper was
the primary shape and outline reference; the Comfyware robot supplied the palette
and restrained shading reference. These were style references, not edit targets.

- Original, preserved unchanged: `design/mascot/loom-original.png`.
- Embedded application asset: `internal/folkweave/static/loom.webp`, 256 × 256.
- Favicon: `internal/folkweave/static/favicon-32.png`, 32 × 32.
- All retain genuine alpha transparency. Tests enforce dimensions, transparent
  margins, and a 25,000-byte application image budget.

## Derivatives

```sh
magick design/mascot/loom-original.png -resize 256x256 -strip \
  -define webp:method=6 -quality 85 internal/folkweave/static/loom.webp
magick design/mascot/loom-original.png -resize 32x32 -strip \
  internal/folkweave/static/favicon-32.png
```

## Generation prompt

Use case: illustration-story. Asset type: Folkweave app mascot for a small header
and a larger sign-in illustration. These input images are STYLE REFERENCES ONLY,
not edit targets: image 1 Imvault is the primary reference for simple rounded
forms, confident dark outlines, and small-size readability; image 2 Comfyware
robot is a palette and restrained shading reference. Create one original friendly
anthropomorphic miniature weaving loom, a compact rounded warm wooden rectangular
loom frame, cream warp threads and a simple woven panel of muted sage green,
cream and terracotta bands. Its upper wooden crossbar has two small dark round
eyes and a tiny smiling mouth. Two simple rounded mitten hands and two small
rounded feet make it a character; one hand holds a small wooden weaving shuttle.
Clear loom silhouette and large simple shapes. Calm friendly expression.
Absolutely no eyebrows, eyelashes, blush, hat, lettering, text, symbols, scenery
or extra characters. A few broad shading areas only, no fine thread detail or
complex machinery. Full character centered on a square canvas with generous
clear transparent margins; legible at 40px and attractive at 160px. Genuine alpha
transparent background, no ground shadow, no colored background or checkerboard,
no watermark.
