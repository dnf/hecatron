#!/usr/bin/env python3
"""Build hecatron's logo files: the mark, the favicon set, the wordmark hero
and the social preview.

The mark is Hecate's crossroads: three angle brackets, one for each road,
pointing in at the dot where they meet. One interface where every language's
road converges.

The wordmark is "hecatron" in Space Grotesk SemiBold (SIL Open Font License),
converted to outlines so the SVGs don't depend on the font being installed.

    python3 -m venv /tmp/logo && /tmp/logo/bin/pip install fonttools
    curl -fsSLo SpaceGrotesk.ttf \\
        'https://github.com/google/fonts/raw/main/ofl/spacegrotesk/SpaceGrotesk%5Bwght%5D.ttf'
    /tmp/logo/bin/python assets/logo/build.py SpaceGrotesk.ttf

Needs rsvg-convert (librsvg) and ImageMagick's magick for the PNGs and the
.ico.
"""

import subprocess
import sys
from pathlib import Path

from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

OUT = Path(__file__).resolve().parent

INK = "#14161D"
PAPER = "#F7F6F2"
ACCENT = "#3B5BFF"
WHITE = "#FFFFFF"

WORDMARK = "hecatron"
TAGLINE = "Tools for polyglot repositories"
TRACKING = -0.03  # em, as in the design canvas

# The mark on a 64-unit grid. The chevron points down at the centre and is
# repeated at 120 and 240 degrees; the group sits 1.5 units low so the
# triangle it forms is optically centred.
CHEVRON = "M23 10L32 19L41 10"
NUDGE = 1.5


def mark_group(stroke, dot, width=6.0, radius=6.0):
    chevrons = "".join(
        f'<path d="{CHEVRON}"{f" transform=\"rotate({angle} 32 32)\"" if angle else ""}/>'
        for angle in (0, 120, 240)
    )
    return (
        f'<g transform="translate(0 {NUDGE})">'
        f'<g fill="none" stroke="{stroke}" stroke-width="{width}" '
        f'stroke-linecap="round" stroke-linejoin="round">{chevrons}</g>'
        f'<circle cx="32" cy="32" r="{radius}" fill="{dot}"/></g>'
    )


def svg(width, height, body, label):
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width:g}" height="{height:g}" '
        f'viewBox="0 0 {width:g} {height:g}" role="img" aria-label="{label}">'
        f"<title>{label}</title>{body}</svg>\n"
    )


def tile(size=64):
    """The favicon: the mark reversed out of an accent rounded square, drawn
    heavier so it holds up at 32 px."""
    scale = size / 64
    return (
        f'<rect width="{size}" height="{size}" rx="{14 * scale:g}" fill="{ACCENT}"/>'
        f'<g transform="scale({scale:g}) translate(8 8) scale(0.75)">'
        f"{mark_group(WHITE, WHITE, width=8, radius=7.5)}</g>"
    )


class Text:
    """A line of text from the font as one SVG path, in pixels at `size`."""

    def __init__(self, font, text, size, tracking=0.0):
        glyphs = font.getGlyphSet()
        cmap = font.getBestCmap()
        upem = font["head"].unitsPerEm
        scale = size / upem
        pen = SVGPathPen(glyphs)
        bounds = BoundsPen(glyphs)
        x = 0.0
        for char in text:
            name = cmap[ord(char)]
            # Flip y: font units go up from the baseline, SVG goes down.
            transform = (scale, 0, 0, -scale, x * scale, 0)
            glyphs[name].draw(TransformPen(pen, transform))
            glyphs[name].draw(TransformPen(bounds, transform))
            x += glyphs[name].width + tracking * upem
        self.path = pen.getCommands()
        self.width = (x - tracking * upem) * scale
        self.top = bounds.bounds[1]  # negative: above the baseline
        self.x_height = font["OS/2"].sxHeight * scale


def lockup(font, colour, mark_colour, size):
    """The mark beside the wordmark, as an SVG body and its width and height,
    with the baseline at y = 0 before translating."""
    word = Text(font, WORDMARK, size, TRACKING)
    mark = 1.22 * size
    gap = 0.26 * size
    # Centre the mark between the x-height and the ascender's midpoints: on
    # the ascender alone it looks high next to the lowercase body.
    centre = (-word.x_height / 2 + word.top / 2) / 2
    top = min(centre - mark / 2, word.top)
    bottom = max(centre + mark / 2, 0)
    height = bottom - top
    body = (
        f'<g transform="translate(0 {-top:.2f})">'
        f'<g transform="translate(0 {centre - mark / 2:.2f}) scale({mark / 64:.4f})">'
        f"{mark_group(colour, mark_colour)}</g>"
        f'<path transform="translate({mark + gap:.2f} 0)" fill="{colour}" d="{word.path}"/></g>'
    )
    return body, mark + gap + word.width, height


def write(name, text):
    (OUT / name).write_text(text)
    print(f"wrote {name}")


def png(source, name, size, height=None):
    args = ["rsvg-convert", "-w", str(size)]
    if height:
        args += ["-h", str(height)]
    subprocess.run([*args, "-o", str(OUT / name), str(OUT / source)], check=True)
    print(f"wrote {name}")


def main(font_path):
    font = instantiateVariableFont(TTFont(font_path), {"wght": 600})
    regular = instantiateVariableFont(TTFont(font_path), {"wght": 400})

    write("mark.svg", svg(64, 64, mark_group(INK, ACCENT), "hecatron"))
    write("mark-dark.svg", svg(64, 64, mark_group(PAPER, ACCENT), "hecatron"))
    write("favicon.svg", svg(64, 64, tile(), "hecatron"))

    for name, colour in (("hero.svg", INK), ("hero-dark.svg", PAPER)):
        body, width, height = lockup(font, colour, ACCENT, 96)
        write(name, svg(round(width + 1), round(height + 1), body, "hecatron"))

    # GitHub's social preview: 1280x640, the lockup and the tagline on ink.
    body, width, height = lockup(font, PAPER, ACCENT, 112)
    tagline = Text(regular, TAGLINE, 34)
    total = height + 56 + 34
    top = (640 - total) / 2
    social = (
        f'<rect width="1280" height="640" fill="{INK}"/>'
        f'<g transform="translate({(1280 - width) / 2:.2f} {top:.2f})">{body}</g>'
        f'<path transform="translate({(1280 - tagline.width) / 2:.2f} {top + height + 56 + 26:.2f})" '
        f'fill="#A3A7B3" d="{tagline.path}"/>'
    )
    write("social-preview.svg", svg(1280, 640, social, "hecatron: " + TAGLINE))

    for size in (32, 48):
        png("favicon.svg", f"favicon-{size}.png", size)
    png("favicon.svg", "icon-512.png", 512)
    # Apple touch icons are square; iOS rounds the corners itself.
    write("apple-touch-icon.svg", svg(180, 180, f'<rect width="180" height="180" fill="{ACCENT}"/>'
          f'<g transform="scale({180 / 64:.4f}) translate(8 8) scale(0.75)">'
          f"{mark_group(WHITE, WHITE, width=8, radius=7.5)}</g>", "hecatron"))
    png("apple-touch-icon.svg", "apple-touch-icon.png", 180)
    (OUT / "apple-touch-icon.svg").unlink()
    png("social-preview.svg", "social-preview.png", 1280, 640)
    subprocess.run(
        ["magick", *(str(OUT / f"favicon-{s}.png") for s in (32, 48)), str(OUT / "favicon.ico")],
        check=True,
    )
    print("wrote favicon.ico")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(f"usage: {sys.argv[0]} SpaceGrotesk.ttf")
    main(sys.argv[1])
