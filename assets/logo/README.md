# hecatron's logo

The mark is Hecate's crossroads: three angle brackets, one for each road,
pointing in at the dot where they meet. Hecatron is the one interface where
every language's road converges. The wordmark is "hecatron" in
[Space Grotesk](https://github.com/floriankarsten/space-grotesk) SemiBold,
converted to outlines.

| File | Use |
| --- | --- |
| `mark.svg`, `mark-dark.svg` | The mark alone, for light and dark backgrounds |
| `hero.svg`, `hero-dark.svg` | The mark with the wordmark, for light and dark backgrounds (the README picks one with `prefers-color-scheme`) |
| `favicon.svg` | The favicon: the mark reversed out of a blue rounded square |
| `favicon.ico`, `favicon-16.png`, `favicon-32.png`, `favicon-48.png` | Raster favicons; the 16 px one is drawn on the pixel grid, because the vector mark blurs at that size |
| `apple-touch-icon.png`, `icon-512.png` | Home-screen and web-manifest icons |
| `social-preview.png` (and `.svg`) | GitHub's social preview (1280×640): Settings › General › Social preview |

| Colour | Hex |
| --- | --- |
| Ink | `#14161D` |
| Paper | `#F7F6F2` |
| Accent | `#3B5BFF` |

For a website:

```html
<link rel="icon" href="/favicon.ico" sizes="48x48">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
```

## Rebuilding

Every file here comes from [`build.py`](build.py): change the geometry,
colours or text there and rerun it. It needs fonttools, the Space Grotesk
variable font, `rsvg-convert` (librsvg) and ImageMagick; the script's
docstring has the commands. Space Grotesk is under the
[SIL Open Font License](https://github.com/floriankarsten/space-grotesk/blob/master/OFL.txt),
which allows using its outlines in a logo.
